package journal

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/colchuck-ai/para/internal/paraerr"
	"github.com/colchuck-ai/para/internal/ptime"
)

// DefaultRotateBytes is §7's default for the log.rotate-bytes threshold.
const DefaultRotateBytes int64 = 4 * 1024 * 1024

// Append writes e as one line into dir: the newest file if it hasn't yet
// exceeded rotateBytes, or a new file named for e's own timestamp otherwise
// (§3.4). Rotation only ever opens a new file — a closed file is never
// reopened for writing once a later one exists.
//
// It returns the file the line landed in. Which file that is cannot be predicted
// from outside — it depends on the newest existing file's size — and §23 makes a
// mutation print every file it wrote by name, so the writer is the only honest
// source for it.
func Append(dir string, e Event, rotateBytes int64) (string, error) {
	data, err := Encode(e)
	if err != nil {
		return "", err
	}

	path, err := targetFile(dir, e, rotateBytes)
	if err != nil {
		return "", err
	}

	// Whether the file is about to be created decides whether the *directory*
	// needs syncing as well: an fsync of the file makes its bytes durable, and
	// only an fsync of the directory makes the entry naming them durable. Every
	// other truth file para writes does both (writeset.writeAtomic), and a
	// journal line is the most primitive form truth takes (§3.1) — so the first
	// event into a new or rotated file must not be the one write in the design
	// that a power failure can lose while the state.toml written after it
	// survives.
	_, statErr := os.Stat(path)
	fresh := os.IsNotExist(statErr)

	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return "", paraerr.Wrap(paraerr.KindInternal, err, fmt.Sprintf("journal: opening %s", path))
	}
	defer f.Close()

	if _, err := f.Write(data); err != nil {
		return "", paraerr.Wrap(paraerr.KindInternal, err, fmt.Sprintf("journal: appending to %s", path))
	}
	if err := f.Sync(); err != nil {
		return "", paraerr.Wrap(paraerr.KindInternal, err, fmt.Sprintf("journal: syncing %s", path))
	}
	if fresh {
		syncDir(dir)
	}
	return path, nil
}

// PlanAppend reports the file each of events would land in if appended, in
// order, through Append — without writing anything. It is `--dry-run`'s read
// of Append: a batch landing in one journal within a single mutation (`set`
// changing several fields at once) can cross rotateBytes partway through, and
// a caller cannot get that right by calling targetFile once and reusing the
// answer, because Append's own decision for event N+1 depends on event N
// having actually landed.
//
// PlanAppend simulates that instead of performing it: the first event's home
// is decided from the real on-disk state exactly as Append would, and after
// that each event's encoded length is added to a running total in place of an
// actual write, so the same rotateBytes boundary trips at the same point a
// real sequential run of Append calls would.
func PlanAppend(dir string, events []Event, rotateBytes int64) ([]string, error) {
	if len(events) == 0 {
		return nil, nil
	}
	if rotateBytes <= 0 {
		rotateBytes = DefaultRotateBytes
	}

	newest, err := newestFile(dir)
	if err != nil {
		return nil, err
	}
	var current string
	var size int64
	if newest != "" {
		current = filepath.Join(dir, newest)
		if size, err = fileSize(current); err != nil {
			return nil, err
		}
	}

	paths := make([]string, len(events))
	for i, e := range events {
		if current == "" || rotates(size, rotateBytes) {
			current = filepath.Join(dir, ptime.JournalFilename(e.At))
			size = 0
		}
		data, err := Encode(e)
		if err != nil {
			return nil, err
		}
		paths[i] = current
		size += int64(len(data))
	}
	return paths, nil
}

// syncDir makes a newly created file's directory entry durable.
//
// Failures are ignored, exactly as writeset.syncDir ignores them and for the
// same reason: syncing a directory is unsupported on some platforms and
// filesystems — Windows cannot sync a directory handle at all — the line is
// already written and visible to every reader, and what is at stake is
// durability across a power loss rather than the correctness of this append.
func syncDir(dir string) {
	f, err := os.Open(dir)
	if err != nil {
		return
	}
	defer f.Close()
	_ = f.Sync()
}

// targetFile decides which file e's line lands in: the current newest file
// unless it has already grown past rotateBytes (§3.4: "grows until it
// exceeds ... at which point the next event opens a new file"), else a new
// file named for e's own timestamp.
func targetFile(dir string, e Event, rotateBytes int64) (string, error) {
	newest, err := newestFile(dir)
	if err != nil {
		return "", err
	}
	if newest != "" {
		size, err := fileSize(filepath.Join(dir, newest))
		if err != nil {
			return "", err
		}
		if !rotates(size, rotateBytes) {
			return filepath.Join(dir, newest), nil
		}
	}
	return filepath.Join(dir, ptime.JournalFilename(e.At)), nil
}

// rotates is §3.4's boundary, the one rule targetFile and PlanAppend must
// agree on bit for bit: a file rotates once it has grown past rotateBytes, not
// once it reaches it — a file sized exactly at the threshold still takes the
// next event.
func rotates(size, rotateBytes int64) bool {
	return size > rotateBytes
}

// newestFile returns the lexically last *.jsonl entry in dir. §3.4's fixed,
// zero-padded filename layout makes lexical order match chronology for the
// events actually written through Append, which is the only thing this
// package relies on. An absent or empty dir reports "" and no error — a
// brand-new entity's journal starts with no files at all.
func newestFile(dir string) (string, error) {
	names, err := listJSONL(dir)
	if err != nil {
		return "", err
	}
	if len(names) == 0 {
		return "", nil
	}
	return names[len(names)-1], nil
}

// Files lists dir's journal files, oldest first, as absolute paths.
//
// §3.4's fixed, zero-padded filename layout makes lexical order match
// chronology, which is what lets ReadAll concatenate them and what lets doctor
// report a bad line as "this file, this line" rather than as an offset into a
// journal that spans several. It is exported so that the answer to "which files
// are this entity's journal" has one definition: a reader that guessed the
// glob would eventually disagree with the writer about a name.
func Files(dir string) ([]string, error) {
	names, err := listJSONL(dir)
	if err != nil {
		return nil, err
	}
	out := make([]string, 0, len(names))
	for _, name := range names {
		out = append(out, filepath.Join(dir, name))
	}
	return out, nil
}

// listJSONL lists the *.jsonl entries directly in dir, sorted lexically. An
// absent dir reports no entries and no error.
func listJSONL(dir string) ([]string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, paraerr.Wrap(paraerr.KindInternal, err, fmt.Sprintf("journal: reading %s", dir))
	}

	var names []string
	for _, entry := range entries {
		if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".jsonl") {
			names = append(names, entry.Name())
		}
	}
	sort.Strings(names)
	return names, nil
}

func fileSize(path string) (int64, error) {
	info, err := os.Stat(path)
	if err != nil {
		return 0, paraerr.Wrap(paraerr.KindInternal, err, fmt.Sprintf("journal: stat %s", path))
	}
	return info.Size(), nil
}

// ReadAll reads every *.jsonl file in dir and returns their events ordered
// by At — never by file or line position (§3.1) — via a stable sort, so
// events sharing an instant keep their file-then-line encounter order. An
// absent dir reports no events and no error.
func ReadAll(dir string) ([]Event, error) {
	names, err := listJSONL(dir)
	if err != nil {
		return nil, err
	}

	var events []Event
	for _, name := range names {
		fileEvents, err := readFile(filepath.Join(dir, name))
		if err != nil {
			return nil, err
		}
		events = append(events, fileEvents...)
	}

	sort.SliceStable(events, func(i, j int) bool {
		return events[i].At.Before(events[j].At)
	})
	return events, nil
}

func readFile(path string) ([]Event, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, paraerr.Wrap(paraerr.KindInternal, err, fmt.Sprintf("journal: opening %s", path))
	}
	defer f.Close()

	var events []Event
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := scanner.Bytes()
		if len(strings.TrimSpace(string(line))) == 0 {
			continue
		}
		e, err := Decode(line)
		if err != nil {
			return nil, paraerr.Wrap(paraerr.KindValidation, err, fmt.Sprintf("journal: %s", path))
		}
		events = append(events, e)
	}
	if err := scanner.Err(); err != nil {
		return nil, paraerr.Wrap(paraerr.KindInternal, err, fmt.Sprintf("journal: reading %s", path))
	}
	return events, nil
}
