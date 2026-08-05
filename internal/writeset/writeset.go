// Package writeset performs the ordered write of one mutation's file set.
//
// A mutation touches four or five files (§2.3), and the design deliberately
// does not reach for a transaction. Instead it exploits a property it already
// has: the tree has exactly one defined degraded state — truth is correct,
// projections are stale — and that state already has a defined repair, doctor
// plus rebuild (§2.4, §19). So the ordering is chosen to make that the only
// state a crash can produce:
//
//	journal append → state.toml → projections → the parent's journal and
//	ACTIVITY.md, last, and only when containment changed (§3.3)
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

// Mutation is one mutating operation's complete file set.
type Mutation struct {
	// Dir is the subject's own directory. Its journal and truth files live
	// under Dir/.para/.
	Dir string

	// Events are appended to the subject's own journal, in order (§3.1). A
	// mutation that changes nothing appends nothing: setting a field to the
	// value it already holds writes no event and no projection (§3.1, §15).
	Events []journal.Event

	// State is the subject's new state.toml, or nil when its truth did not
	// change — a `note` or a `measure` changes the journal without changing
	// state.
	State []byte

	// Projections are the generated files to rewrite, in the order they should
	// be written. They come from render.Artifacts, so the set is exactly the
	// files the subject owns.
	Projections []File

	// Parent is written last, and only when containment changed (§2.3, §3.3):
	// a note or a field change on a child touches nothing at the parent.
	Parent *Parent

	// RotateBytes is the resolved log.rotate-bytes (§3.4, §7). Zero means
	// journal.DefaultRotateBytes.
	RotateBytes int64
}

// Parent is the parent's half of a containment change: its own `child` event,
// and the ACTIVITY.md that event lands in. Its README frontmatter does not
// change, because a parent's stored fields say nothing about its children —
// the child list is `ls` (§8.2).
type Parent struct {
	Dir      string
	Events   []journal.Event
	Activity File
}

// OpKind distinguishes the two ways Apply touches the filesystem.
type OpKind int

const (
	// OpAppend is a single-line O_APPEND into a journal (§3.1).
	OpAppend OpKind = iota
	// OpWrite is an atomic whole-file replacement.
	OpWrite
)

func (k OpKind) String() string {
	if k == OpAppend {
		return "append"
	}
	return "write"
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
	if m.Dir == "" {
		return nil, paraerr.New(paraerr.KindInternal, "writeset: mutation has no directory")
	}
	rotate := m.RotateBytes
	if rotate <= 0 {
		rotate = journal.DefaultRotateBytes
	}

	var ops Ops

	// 1. The journal. Truth first, and the journal is the most primitive form
	//    truth takes: an append-only line nothing else derives from.
	appended, err := appendAll(truth.LogsDir(m.Dir), m.Events, rotate)
	ops = append(ops, appended...)
	if err != nil {
		return ops, err
	}

	// 2. state.toml. Still truth, and the file every projection derives from.
	if m.State != nil {
		if err := writeAtomic(truth.StatePath(m.Dir), m.State); err != nil {
			return ops, err
		}
		ops = append(ops, Op{Kind: OpWrite, Path: truth.StatePath(m.Dir)})
	}

	// 3. The projections. Everything from here on is re-derivable, so a crash
	//    leaves stale-projection and nothing worse.
	for _, f := range m.Projections {
		if err := writeAtomic(f.Path, f.Bytes); err != nil {
			return ops, err
		}
		ops = append(ops, Op{Kind: OpWrite, Path: f.Path})
	}

	// 4. The parent, last, and only when containment changed.
	if m.Parent != nil {
		appended, err := appendAll(truth.LogsDir(m.Parent.Dir), m.Parent.Events, rotate)
		ops = append(ops, appended...)
		if err != nil {
			return ops, err
		}
		if m.Parent.Activity.Path != "" {
			if err := writeAtomic(m.Parent.Activity.Path, m.Parent.Activity.Bytes); err != nil {
				return ops, err
			}
			ops = append(ops, Op{Kind: OpWrite, Path: m.Parent.Activity.Path})
		}
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
		if err := journal.Append(logsDir, e, rotate); err != nil {
			return ops, err
		}
		ops = append(ops, Op{Kind: OpAppend, Path: logsDir})
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
