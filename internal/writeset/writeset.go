// Package writeset performs the ordered write of one mutation's file set.
//
// A mutation touches four or five files (§2.3), and the design deliberately
// does not reach for a transaction. Instead it exploits a property it already
// has: the tree has exactly one defined degraded state — truth is correct,
// projections are stale — and that state already has a defined repair, doctor
// plus rebuild (§2.4, §19). So the ordering is chosen to make that the only
// state a crash can produce:
//
//	journal append → state.toml → config.toml → projections → the parents'
//	journals and ACTIVITY.md, last, and only when containment changed (§3.3)
//
// A relocating verb runs one phase ahead of that, through Relocate: the renames
// and deletions that move the bytes (§18.3–§18.5). See Relocation for why that
// is a separate call rather than a fifth field of Mutation.
//
// A mutation may have more than one subject — `add` creates a project and its
// `objectives/` container in one operation (§18.1) — and the phases above are
// applied across all of them: every subject's truth, then every subject's
// projections. A crash between the two subjects would otherwise leave the second
// one missing entirely, which is a state no repair the design defines can fix.
//
// Truth first, projections after. A crash at any point leaves the tree with
// correct truth and possibly stale projections, never with truth lost or half
// written. Individual file writes are atomic — a temp file in the same
// directory, rename — and the journal append is an O_APPEND of a single line,
// which is atomic for a line this size on every filesystem para targets.
//
// Truth is also fsynced and projections are not, which is the same argument
// applied to power loss rather than to a process dying. See durability.
//
// Apply returns what it actually did, because §2.3's invariant is a claim about
// how many files a mutation touches ("nothing walks a subtree on write, nothing
// walks to root on write") and the only honest way to assert a claim about
// filesystem writes is to count them.
package writeset

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/colchuck-ai/para/internal/journal"
	"github.com/colchuck-ai/para/internal/paraerr"
	"github.com/colchuck-ai/para/internal/truth"
)

// File is one file to write atomically. Path is an OS path, absolute or
// relative to the process's working directory — never the slash-separated,
// root-relative form render.Artifact carries, which the caller joins onto the
// tree root first.
type File struct {
	Path  string
	Bytes []byte
}

// Subject is one entity's, container's, or root's half of a mutation: the
// events its journal gains, its truth files, and its projections.
type Subject struct {
	// Dir is the subject's own directory. Its journal and truth files live
	// under Dir/.para/.
	Dir string

	// Dirs are directories that must exist even while empty, as OS paths: a new
	// entity's .para/logs/, which `add` creates although the journal starts
	// empty (§18.1) so that every .para/ has the same shape (§5.1, §8.4), and
	// `init`'s two .agents/ directories. Nothing may depend on any of them: git
	// does not carry an empty directory, so a fresh clone will not have one,
	// which is why every reader treats an absent logs/ as an empty journal.
	//
	// They are created *after* the subject's truth, for exactly that reason —
	// see writeTruth.
	Dirs []string

	// Events are appended to the subject's own journal, in order (§3.1). A
	// mutation that changes nothing appends nothing: setting a field to the
	// value it already holds writes no event and no projection (§3.1, §15).
	Events []journal.Event

	// State is the subject's new state.toml, or nil when its truth did not
	// change — a `note` or a `measure` changes the journal without changing
	// state.
	State []byte

	// Tree is the new .para/tree.toml, or nil. It is the root's State by
	// another name: the root has no state.toml because the root is not an
	// entity, it is the tree (§8.1). Only `init` sets it, and only on the root,
	// so no subject ever carries both.
	//
	// It is written in the truth phase like every other truth file, and `init`
	// deliberately makes the root its *last* subject so that this is the last
	// truth byte to land. Until it exists the directory is not a tree at all,
	// which is a state `init` can simply be run on again; once it exists, every
	// other subject's truth is already there and what remains is projections —
	// the one degraded state the design defines a repair for (§2.4).
	Tree []byte

	// Config is the subject's new config.toml, or nil when it did not change.
	// A config.toml is truth but not state (§2.1), so it is written in the
	// truth phase beside state.toml: `add` creates an empty one, and
	// `config set` writes one and nothing else of the kind.
	Config []byte

	// Projections are the generated files to rewrite, in the order they should
	// be written. They come from render.Artifacts, so the set is exactly the
	// files the subject owns.
	Projections []File

	// RotateBytes is log.rotate-bytes as resolved *at this subject* (§3.4,
	// §7). It is per subject rather than per mutation because the key is
	// chain-resolved like every other: a project that sets its own must not
	// decide when its parent bucket's journal rotates. Zero means
	// journal.DefaultRotateBytes.
	RotateBytes int64
}

// Mutation is one mutating operation's complete file set.
type Mutation struct {
	// Subjects are the entities the mutation creates or changes. There is
	// normally one; `add` has two, because a project's `objectives/` container
	// is created eagerly in the same operation (§18.1).
	//
	// Every subject's truth is written before any subject's projections, so
	// the one degraded state a crash can produce is still the defined one —
	// truth correct, projections stale — even for a mutation that creates two
	// directories at once.
	Subjects []Subject

	// Parents are written last, and only when containment changed (§2.3, §3.3):
	// a note or a field change on a child touches nothing at the parent. Each
	// is a Subject because it is one — the fields it leaves empty are the
	// point. A parent's README frontmatter never changes, because its stored
	// fields say nothing about its children; the child list is `ls` (§8.2).
	//
	// There is normally one. A relocation has two — §18.3 logs `child moved` at
	// the old parent and the new one — and a rename inside one parent has one
	// again, because the two ends are the same journal and one event is what
	// happened.
	Parents []Subject
}

// Relocation is the byte-moving phase of `move`, `archive`, `unarchive`, and
// `remove`: the directories a stub chain needs, the renames themselves, and the
// paths a removal deletes (§18.3–§18.5).
//
// It is deliberately not a phase of Mutation, and the reason is that everything
// Apply writes is *read* from the post-relocation tree — an entity's README body,
// its ACTIVITY.md's prior days, and the chain of config.toml files that decides
// its rotation threshold all live at the new path once the rename has happened.
// So a relocating verb calls Relocate first and builds its write set against the
// tree that results.
//
// That ordering is also the safe one. The rename *is* the mutation: a locator is
// a path (§1.4), so moving the bytes is the truth change, and the journal line is
// the record of it. A crash after the rename loses the record of a move that
// happened, which leaves correct truth and a stale projection — the one degraded
// state the design defines a repair for (§2.4). A crash after the journal append
// but before the rename would leave a record of a move that did not happen, and
// `rebuild` would faithfully re-derive an ACTIVITY.md claiming it.
type Relocation struct {
	// Dirs are directories to create before any rename, as OS paths. These are
	// §1.6's archive stubs: bare ancestry placeholders with no .para/ and no
	// README.md, which exist so an archived entity records where it came from.
	Dirs []string

	// Moves are the renames, in order. Ordering matters when a chain is being
	// reinstated: an ancestor's own files move before the child beneath it, so
	// no intermediate state has an archived entity sitting at a live path.
	Moves []Move

	// Prunes are paths deleted recursively, after the moves. `remove` deletes
	// the subtree or para's footprint within it (§18.4); `unarchive` deletes a
	// stub left recording nothing (§1.6).
	Prunes []string
}

// Move is one rename: an existing path, and a path that must not exist. Refusing
// an occupied destination rather than letting rename(2) decide is what keeps the
// stub merge honest — os.Rename would silently succeed onto an empty directory
// and fail onto a non-empty one, so the caller, which knows whether it meant to
// adopt a stub, is the one that has to have decided.
type Move struct {
	From string
	To   string
}

// Empty reports whether r would touch nothing.
func (r Relocation) Empty() bool {
	return len(r.Dirs) == 0 && len(r.Moves) == 0 && len(r.Prunes) == 0
}

// Relocate performs r and reports what it did, stopping at the first failure and
// returning the operations already completed — exactly the ones a crash at that
// point would have left behind.
func Relocate(r Relocation) (Ops, error) {
	var ops Ops

	for _, dir := range r.Dirs {
		if isDir(dir) {
			// Already there: an ancestor's archive directory that another
			// archive created earlier, or a bucket `init` made. Creating it
			// again is not an operation anyone performed.
			continue
		}
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return ops, paraerr.Wrap(paraerr.KindInternal, err, fmt.Sprintf("writeset: creating %s", dir))
		}
		ops = record(ops, Op{Kind: OpMkdir, Path: dir})
	}

	for _, m := range r.Moves {
		if m.From == "" || m.To == "" {
			return ops, paraerr.New(paraerr.KindInternal, "writeset: move with an empty path")
		}
		if _, err := os.Lstat(m.To); err == nil {
			return ops, paraerr.Newf(paraerr.KindConflict, "writeset: %s already exists", m.To)
		} else if !os.IsNotExist(err) {
			return ops, paraerr.Wrap(paraerr.KindInternal, err, fmt.Sprintf("writeset: checking %s", m.To))
		}
		if err := os.MkdirAll(filepath.Dir(m.To), 0o755); err != nil {
			return ops, paraerr.Wrap(paraerr.KindInternal, err, fmt.Sprintf("writeset: creating %s", filepath.Dir(m.To)))
		}
		if err := os.Rename(m.From, m.To); err != nil {
			// A cross-device rename is the one failure worth naming, because
			// the repair is not para's: a tree with a submount inside it cannot
			// be relocated with rename(2), and copying instead would have to
			// reproduce modes, times, and hard links to be a move rather than an
			// approximation of one.
			return ops, paraerr.Wrap(paraerr.KindInternal, err,
				fmt.Sprintf("writeset: renaming %s to %s (a tree spanning two filesystems must be moved by hand, then `para rebuild`)", m.From, m.To))
		}
		ops = record(ops, Op{Kind: OpMove, From: m.From, Path: m.To})
	}

	// Deepest first, which is the only ordering among the prunes that matters.
	//
	// `remove --keep-files` deletes para's footprint throughout a subtree
	// (§18.4), and a shallow-first sweep spends the interval between two of them
	// with an entity's `.para/` gone and its descendants' still there — a
	// state.toml the walk can no longer descend to, which §10 calls `orphan`,
	// which is an *error*, and which no `rebuild` can repair. Deepest first, the
	// worst a crash leaves is a partly-stripped subtree whose remaining footprint
	// is still reachable from the top: content inside an entity, which is never
	// reported, or one `untracked` directory at the end (§0.2, §21.2).
	prunes := slices.Clone(r.Prunes)
	slices.SortStableFunc(prunes, func(a, b string) int {
		return strings.Count(b, string(filepath.Separator)) - strings.Count(a, string(filepath.Separator))
	})

	for _, path := range prunes {
		if _, err := os.Lstat(path); err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return ops, paraerr.Wrap(paraerr.KindInternal, err, fmt.Sprintf("writeset: checking %s", path))
		}
		if err := os.RemoveAll(path); err != nil {
			return ops, paraerr.Wrap(paraerr.KindInternal, err, fmt.Sprintf("writeset: removing %s", path))
		}
		ops = record(ops, Op{Kind: OpPrune, Path: path})
	}

	return ops, nil
}

func isDir(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}

// OpKind distinguishes the ways Apply touches the filesystem.
type OpKind int

const (
	// OpAppend is a single-line O_APPEND into a journal (§3.1).
	OpAppend OpKind = iota
	// OpWrite is an atomic whole-file replacement.
	OpWrite
	// OpMkdir is the creation of a directory that stays empty.
	OpMkdir
	// OpMove is a rename (§18.3).
	OpMove
	// OpPrune is a recursive delete (§18.4).
	OpPrune
)

func (k OpKind) String() string {
	switch k {
	case OpAppend:
		return "append"
	case OpMkdir:
		return "mkdir"
	case OpMove:
		return "move"
	case OpPrune:
		return "prune"
	default:
		return "write"
	}
}

// Op is one filesystem operation Apply or Relocate performed.
type Op struct {
	Kind OpKind
	Path string
	// From is the path an OpMove came from, and empty for every other kind.
	From string
}

// Ops is the ordered record of a mutation's filesystem operations — the
// evidence for §2.3's invariant.
type Ops []Op

// record appends op and gives the crash hook a chance to end the process.
//
// Every operation this package *reports* goes through it, which is what lets
// Phase 14's crash matrix sweep PARA_CRASH_AFTER from 1 upward and step over no
// reported write. Three things it does not reach, named because "exhaustive"
// would be the wrong word without them:
//
//   - the MkdirAll inside appendAll, writeAtomic, and Relocate, each of which
//     makes a directory on the way to a write rather than as one;
//   - the inside of writeAtomic, whose temp file, write, chmod and rename are
//     one recorded op — a kill between them is unreachable here.
//
// Both leave residue that has been checked by hand rather than swept: an empty
// entity directory, which is the `untracked` advisory the matrix already covers,
// and a leftover `.<name>.para-*` temp file, which no reader and no doctor check
// looks at. Neither can lose truth, because neither happens after a truth file
// has been renamed into place.
//
// In a production build crashPoint compiles to nothing.
func record(ops Ops, op Op) Ops {
	ops = append(ops, op)
	crashPoint(op)
	return ops
}

// Paths returns the paths touched, in order.
func (o Ops) Paths() []string {
	out := make([]string, 0, len(o))
	for _, op := range o {
		out = append(out, op.Path)
	}
	return out
}

// validate is the one shape rule both Apply and Plan refuse before doing
// anything else: a mutation with nothing in it, or a subject with nowhere to
// write. Apply and Plan share this rather than each stating it, so the rule a
// rehearsal refuses on is provably the rule a real run refuses on too.
func validate(m Mutation) error {
	if len(m.Subjects) == 0 && len(m.Parents) == 0 {
		return paraerr.New(paraerr.KindInternal, "writeset: mutation has no subject")
	}
	for _, s := range m.Subjects {
		if s.Dir == "" {
			return paraerr.New(paraerr.KindInternal, "writeset: mutation has a subject with no directory")
		}
	}
	return nil
}

// Apply writes m in the order the package doc fixes, and reports what it did.
// On error it stops at the failing write and returns the operations completed
// so far, which are exactly the ones a crash at that point would have left
// behind.
//
// Apply creates the directory each file needs, which is what lets `add` be one
// call rather than a mkdir pass followed by a write pass: the write set is the
// definition of which directories exist.
func Apply(m Mutation) (Ops, error) {
	if err := validate(m); err != nil {
		return nil, err
	}
	var ops Ops

	// 1. Truth, for every subject, before any projection: journals, then
	//    state.toml, then config.toml. The journal comes first because it is
	//    the most primitive form truth takes — an append-only line nothing else
	//    derives from.
	for _, s := range m.Subjects {
		written, err := writeTruth(s)
		ops = append(ops, written...)
		if err != nil {
			return ops, err
		}
	}

	// 2. The projections. Everything from here on is re-derivable, so a crash
	//    leaves stale-projection and nothing worse.
	for _, s := range m.Subjects {
		written, err := writeFiles(s.Projections, rederivable)
		ops = append(ops, written...)
		if err != nil {
			return ops, err
		}
	}

	// 3. The parents, last, and only when containment changed.
	for _, p := range m.Parents {
		written, err := writeTruth(p)
		ops = append(ops, written...)
		if err != nil {
			return ops, err
		}
		written, err = writeFiles(p.Projections, rederivable)
		ops = append(ops, written...)
		if err != nil {
			return ops, err
		}
	}

	return ops, nil
}

// Plan reports what Apply would do to m without writing anything, for
// `--dry-run`. It walks the exact phases Apply's own doc comment fixes —
// subjects' truth, then subjects' projections, then parents' truth, then
// parents' projections — so a rehearsal and a real run can never drift apart
// by consulting two different orderings of the same Mutation.
//
// It never calls record: crashPoint exists to let Phase 14's crash matrix kill
// the process between two real writes, and a dry run performs none, so there
// is nothing for a mid-mutation crash to interrupt.
func Plan(m Mutation) (Ops, error) {
	if err := validate(m); err != nil {
		return nil, err
	}
	var ops Ops

	for _, s := range m.Subjects {
		planned, err := planTruth(s)
		ops = append(ops, planned...)
		if err != nil {
			return ops, err
		}
	}
	for _, s := range m.Subjects {
		ops = append(ops, planFiles(s.Projections)...)
	}
	for _, p := range m.Parents {
		planned, err := planTruth(p)
		ops = append(ops, planned...)
		if err != nil {
			return ops, err
		}
		ops = append(ops, planFiles(p.Projections)...)
	}
	return ops, nil
}

// planTruth is writeTruth's read-only twin: the journal appends and the truth
// files a subject would gain, in the same order writeTruth itself uses.
//
// It has nothing to say about s.Dirs — the scaffolding directories `add` and
// `init` create even while empty — because wrote (mutate.go) already filters
// OpMkdir out of everything it prints: a directory creation is not a file
// anyone wrote, dry run or not, so there is nothing here for it to plan.
func planTruth(s Subject) (Ops, error) {
	rotate := s.RotateBytes
	if rotate <= 0 {
		rotate = journal.DefaultRotateBytes
	}

	paths, err := journal.PlanAppend(truth.LogsDir(s.Dir), s.Events, rotate)
	if err != nil {
		return nil, err
	}
	ops := make(Ops, 0, len(paths))
	for _, p := range paths {
		ops = append(ops, Op{Kind: OpAppend, Path: p})
	}

	ops = append(ops, planFiles([]File{
		{Path: truth.StatePath(s.Dir), Bytes: s.State},
		{Path: truth.ConfigPath(s.Dir), Bytes: s.Config},
		{Path: truth.TreePath(s.Dir), Bytes: s.Tree},
	})...)
	return ops, nil
}

// planFiles is writeFiles' read-only twin: which of files would be written,
// in order, skipping the ones whose Bytes is nil exactly as writeFiles does.
func planFiles(files []File) Ops {
	var ops Ops
	for _, f := range files {
		if f.Bytes == nil {
			continue
		}
		ops = append(ops, Op{Kind: OpWrite, Path: f.Path})
	}
	return ops
}

// writeTruth writes one subject's directories, journal lines, and truth files,
// in that order.
func writeTruth(s Subject) (Ops, error) {
	rotate := s.RotateBytes
	if rotate <= 0 {
		rotate = journal.DefaultRotateBytes
	}

	var ops Ops
	appended, err := appendAll(truth.LogsDir(s.Dir), s.Events, rotate)
	ops = append(ops, appended...)
	if err != nil {
		return ops, err
	}

	// tree.toml goes last of the three, and it is the only ordering decision
	// among them that matters: it is the marker that makes a directory a tree
	// (§8.1), so nothing should be able to find a root above a truth file that
	// is not there yet. See Subject.Tree.
	written, err := writeFiles([]File{
		{Path: truth.StatePath(s.Dir), Bytes: s.State},
		{Path: truth.ConfigPath(s.Dir), Bytes: s.Config},
		{Path: truth.TreePath(s.Dir), Bytes: s.Tree},
	}, durable)
	ops = append(ops, written...)
	if err != nil {
		return ops, err
	}

	// The empty directories last, after every byte of truth. They are scaffolding
	// rather than truth — nothing may depend on them existing, since git does not
	// carry an empty directory (§18.1) — and creating them first would make the
	// first thing a crash can leave behind an empty directory in a bucket, which
	// `doctor` reports as `untracked` and `rebuild` will not remove. Truth first
	// means the first thing a crash leaves is a state.toml, and that is the one
	// degraded state the design defines a repair for (§0.2, §2.4).
	for _, dir := range s.Dirs {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return ops, paraerr.Wrap(paraerr.KindInternal, err, fmt.Sprintf("writeset: creating %s", dir))
		}
		ops = record(ops, Op{Kind: OpMkdir, Path: dir})
	}
	return ops, nil
}

// writeFiles writes each file whose bytes are non-nil, in order. A nil Bytes
// means "unchanged", which is how a `note` skips state.toml and a parent skips
// everything but its ACTIVITY.md.
func writeFiles(files []File, d durability) (Ops, error) {
	var ops Ops
	for _, f := range files {
		if f.Bytes == nil {
			continue
		}
		if err := writeAtomic(f.Path, f.Bytes, d); err != nil {
			return ops, err
		}
		ops = record(ops, Op{Kind: OpWrite, Path: f.Path})
	}
	return ops, nil
}

func appendAll(logsDir string, events []journal.Event, rotate int64) (Ops, error) {
	if len(events) == 0 {
		return nil, nil
	}
	if err := os.MkdirAll(logsDir, 0o755); err != nil {
		return nil, paraerr.Wrap(paraerr.KindInternal, err, fmt.Sprintf("writeset: creating %s", logsDir))
	}
	var ops Ops
	for _, e := range events {
		path, err := journal.Append(logsDir, e, rotate)
		if err != nil {
			return ops, err
		}
		ops = record(ops, Op{Kind: OpAppend, Path: path})
	}
	return ops, nil
}

// durability says whether a file's bytes must survive a power failure or only a
// process crash — the difference between an fsync and no fsync, and, measured,
// the difference between 10ms a file and 0.25ms.
//
// The two answers fall straight out of §0.2 rather than out of taste. Truth is
// the only thing para cannot re-derive, so it is written durably. A projection
// is a function of truth (§2.2), so a projection lost to a power failure leaves
// the tree in *precisely* the degraded state §0.2 already defines and §2.4
// already repairs: truth correct, projections stale, `doctor` names it and
// `rebuild` fixes it. Paying ten milliseconds a file to avoid a state that has a
// one-command repair is paying for nothing — and it is what made a whole-tree
// rebuild cost a minute where it now costs seconds.
//
// Both are equally safe against the crash the design actually models, and the
// one the crash matrix exercises: a process that stops. A rename is visible to
// every subsequent reader whether or not anything was flushed to the platter.
type durability int

const (
	// durable fsyncs the bytes and the directory entry before returning.
	durable durability = iota
	// rederivable does neither, because what is at stake is a file `rebuild`
	// writes from truth.
	rederivable
)

// WriteProjection atomically replaces the generated file at path, creating its
// directory if needed, and does not fsync.
//
// It is `rebuild`'s primitive, and the only one exported: a mutation's truth
// goes through Apply, which is where the ordering §0.2 fixes lives, and there is
// no caller left that writes truth any other way. There was one — `config set`
// wrote its config.toml directly — and since Phase 8 it goes through Apply like
// everything else, because the file, the event, and the level's ACTIVITY.md are
// one mutation.
//
// See durability for why a rebuild that skips the fsync is not a rebuild that
// skips a guarantee.
func WriteProjection(path string, data []byte) error {
	return writeAtomic(path, data, rederivable)
}

// writeAtomic replaces path's contents in one step: write a temp file in the
// same directory so the rename cannot cross a filesystem boundary, optionally
// fsync it so the bytes are durable before anything points at them, rename it
// into place, then optionally fsync the directory so the rename itself is
// durable. A reader either sees the whole old file or the whole new one, either
// way.
func writeAtomic(path string, data []byte, d durability) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return paraerr.Wrap(paraerr.KindInternal, err, fmt.Sprintf("writeset: creating %s", dir))
	}

	tmp, err := os.CreateTemp(dir, "."+filepath.Base(path)+".para-*")
	if err != nil {
		return paraerr.Wrap(paraerr.KindInternal, err, fmt.Sprintf("writeset: creating a temp file beside %s", path))
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName) // a no-op once the rename has succeeded

	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return paraerr.Wrap(paraerr.KindInternal, err, fmt.Sprintf("writeset: writing %s", path))
	}
	if d == durable {
		if err := tmp.Sync(); err != nil {
			tmp.Close()
			return paraerr.Wrap(paraerr.KindInternal, err, fmt.Sprintf("writeset: syncing %s", path))
		}
	}
	if err := tmp.Close(); err != nil {
		return paraerr.Wrap(paraerr.KindInternal, err, fmt.Sprintf("writeset: closing the temp file for %s", path))
	}
	// 0o644 rather than CreateTemp's 0o600: these are files a human reads and
	// a repository carries.
	if err := os.Chmod(tmpName, 0o644); err != nil {
		return paraerr.Wrap(paraerr.KindInternal, err, fmt.Sprintf("writeset: setting the mode of %s", path))
	}
	if err := os.Rename(tmpName, path); err != nil {
		return paraerr.Wrap(paraerr.KindInternal, err, fmt.Sprintf("writeset: renaming into %s", path))
	}
	if d != durable {
		return nil
	}
	return syncDir(dir)
}

// syncDir makes a rename durable. A failure to open or sync a directory is not
// fatal on every platform — Windows cannot sync a directory handle at all — so
// this reports only the errors that indicate the write itself is in doubt.
func syncDir(dir string) error {
	f, err := os.Open(dir)
	if err != nil {
		return paraerr.Wrap(paraerr.KindInternal, err, fmt.Sprintf("writeset: opening %s to sync it", dir))
	}
	defer f.Close()
	if err := f.Sync(); err != nil {
		// Syncing a directory is unsupported on some platforms and
		// filesystems. The rename has already happened; the only thing at
		// stake is durability across a power loss, which is not this
		// mutation's contract to guarantee alone.
		return nil
	}
	return nil
}
