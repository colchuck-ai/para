// Package writeset performs the ordered write of one mutation's file set.
//
// A mutation touches four or five files (§2.3), and the design deliberately
// does not reach for a transaction. Instead it exploits a property it already
// has: the tree has exactly one defined degraded state — truth is correct,
// projections are stale — and that state already has a defined repair, doctor
// plus rebuild (§2.4, §19). So the ordering is chosen to make that the only
// state a crash can produce:
//
//	journal append → state.toml → config.toml → projections → the parent's
//	journal and ACTIVITY.md, last, and only when containment changed (§3.3)
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
// directory, fsync, rename — and the journal append is an O_APPEND of a single
// line, which is atomic for a line this size on every filesystem para targets.
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

	// Dirs are directories that must exist even while empty, as OS paths.
	// There is exactly one such directory in the design — a new entity's
	// .para/logs/, which `add` creates although the journal starts empty
	// (§18.1) so that every .para/ has the same shape (§5.1, §8.4). Nothing
	// may depend on it: git does not carry an empty directory, so a fresh
	// clone will not have one, which is why every reader treats an absent
	// logs/ as an empty journal.
	Dirs []string

	// Events are appended to the subject's own journal, in order (§3.1). A
	// mutation that changes nothing appends nothing: setting a field to the
	// value it already holds writes no event and no projection (§3.1, §15).
	Events []journal.Event

	// State is the subject's new state.toml, or nil when its truth did not
	// change — a `note` or a `measure` changes the journal without changing
	// state.
	State []byte

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

	// Parent is written last, and only when containment changed (§2.3, §3.3):
	// a note or a field change on a child touches nothing at the parent. It is
	// a Subject because it is one — the fields it leaves empty are the point.
	// A parent's README frontmatter never changes, because its stored fields
	// say nothing about its children; the child list is `ls` (§8.2).
	Parent *Subject
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
)

func (k OpKind) String() string {
	switch k {
	case OpAppend:
		return "append"
	case OpMkdir:
		return "mkdir"
	default:
		return "write"
	}
}

// Op is one filesystem operation Apply performed.
type Op struct {
	Kind OpKind
	Path string
}

// Ops is the ordered record of a mutation's filesystem operations — the
// evidence for §2.3's invariant.
type Ops []Op

// Paths returns the paths touched, in order.
func (o Ops) Paths() []string {
	out := make([]string, 0, len(o))
	for _, op := range o {
		out = append(out, op.Path)
	}
	return out
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
	if len(m.Subjects) == 0 && m.Parent == nil {
		return nil, paraerr.New(paraerr.KindInternal, "writeset: mutation has no subject")
	}
	for _, s := range m.Subjects {
		if s.Dir == "" {
			return nil, paraerr.New(paraerr.KindInternal, "writeset: mutation has a subject with no directory")
		}
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
		written, err := writeFiles(s.Projections)
		ops = append(ops, written...)
		if err != nil {
			return ops, err
		}
	}

	// 3. The parent, last, and only when containment changed.
	if m.Parent != nil {
		written, err := writeTruth(*m.Parent)
		ops = append(ops, written...)
		if err != nil {
			return ops, err
		}
		written, err = writeFiles(m.Parent.Projections)
		ops = append(ops, written...)
		if err != nil {
			return ops, err
		}
	}

	return ops, nil
}

// writeTruth writes one subject's directories, journal lines, and truth files,
// in that order.
func writeTruth(s Subject) (Ops, error) {
	rotate := s.RotateBytes
	if rotate <= 0 {
		rotate = journal.DefaultRotateBytes
	}

	var ops Ops
	for _, dir := range s.Dirs {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return ops, paraerr.Wrap(paraerr.KindInternal, err, fmt.Sprintf("writeset: creating %s", dir))
		}
		ops = append(ops, Op{Kind: OpMkdir, Path: dir})
	}

	appended, err := appendAll(truth.LogsDir(s.Dir), s.Events, rotate)
	ops = append(ops, appended...)
	if err != nil {
		return ops, err
	}

	written, err := writeFiles([]File{
		{Path: truth.StatePath(s.Dir), Bytes: s.State},
		{Path: truth.ConfigPath(s.Dir), Bytes: s.Config},
	})
	ops = append(ops, written...)
	return ops, err
}

// writeFiles writes each file whose bytes are non-nil, in order. A nil Bytes
// means "unchanged", which is how a `note` skips state.toml and a parent skips
// everything but its ACTIVITY.md.
func writeFiles(files []File) (Ops, error) {
	var ops Ops
	for _, f := range files {
		if f.Bytes == nil {
			continue
		}
		if err := writeAtomic(f.Path, f.Bytes); err != nil {
			return ops, err
		}
		ops = append(ops, Op{Kind: OpWrite, Path: f.Path})
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
		ops = append(ops, Op{Kind: OpAppend, Path: path})
	}
	return ops, nil
}

// WriteFile atomically replaces the file at path, creating its directory if
// needed.
//
// It is the same primitive Apply uses for every file in a mutation, exported
// for the writes that are not mutations of an entity: a config.toml is truth
// but not state (§2.1), so `config set` writes one file and appends no event,
// and it should still be as crash-safe as everything else para writes.
func WriteFile(path string, data []byte) error {
	return writeAtomic(path, data)
}

// writeAtomic replaces path's contents in one step: write a temp file in the
// same directory so the rename cannot cross a filesystem boundary, fsync it so
// the bytes are durable before anything points at them, rename it into place,
// then fsync the directory so the rename itself is durable. A reader either
// sees the whole old file or the whole new one.
func writeAtomic(path string, data []byte) error {
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
	if err := tmp.Sync(); err != nil {
		tmp.Close()
		return paraerr.Wrap(paraerr.KindInternal, err, fmt.Sprintf("writeset: syncing %s", path))
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
