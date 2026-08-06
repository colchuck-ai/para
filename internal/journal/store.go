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
	return path, nil
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
		if size <= rotateBytes {
			return filepath.Join(dir, newest), nil
		}
	}
	return filepath.Join(dir, ptime.JournalFilename(e.At)), nil
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
