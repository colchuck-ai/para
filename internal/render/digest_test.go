package render_test

import (
	"strings"
	"testing"

	"github.com/colchuck-ai/para/internal/journal"
	"github.com/colchuck-ai/para/internal/kindmeta"
	"github.com/colchuck-ai/para/internal/render"
	"github.com/colchuck-ai/para/internal/truth"
)

// digestIn is the subject every case here folds: one project, created on the
// day its events fall on.
func digestIn(t *testing.T, events ...journal.Event) render.In {
	t.Helper()
	return render.In{
		Locator: loc(t, "projects.acme"),
		Kind:    kindmeta.KindProject,
		State:   truth.State{Name: "Acme", Created: "2026-03-05T08:00:00-08:00"},
		Events:  events,
	}
}

// TestDigestIsTheSameFoldAsTheFile is §16.4's "they agree by construction",
// made checkable: `activity` and ACTIVITY.md differ in spelling and in nothing
// else, so every line of one is a line of the other with the markdown removed.
func TestDigestIsTheSameFoldAsTheFile(t *testing.T) {
	in := digestIn(t,
		journal.NewChange(ts(t, "2026-03-05T09:00:00"), "status", "planned", "in-progress", ""),
		journal.NewNote(ts(t, "2026-03-05T10:00:00"), "waiting on the ingest team"),
		journal.NewChild(ts(t, "2026-03-04T09:00:00"), journal.ChildOpAdded, "objectives", "", "", ""),
	)

	file, err := render.Activity.Render(in)
	if err != nil {
		t.Fatal(err)
	}
	days, err := render.Digest(in)
	if err != nil {
		t.Fatal(err)
	}

	var fileLines []string
	for _, line := range strings.Split(string(file), "\n") {
		if strings.HasPrefix(line, "- ") {
			fileLines = append(fileLines, strings.TrimPrefix(line, "- "))
		}
	}
	var digestLines []string
	for _, day := range days {
		for _, line := range day.Lines {
			digestLines = append(digestLines, line.Text)
		}
	}
	if len(fileLines) != len(digestLines) {
		t.Fatalf("the file has %d lines and the digest %d:\n%v\n%v", len(fileLines), len(digestLines), fileLines, digestLines)
	}
	for i := range fileLines {
		if got, want := digestLines[i], undress(fileLines[i]); got != want {
			t.Errorf("line %d: digest %q, file (undressed) %q", i, got, want)
		}
	}
}

// undress strips the two things markdown style adds: emphasis markers, and a
// sentence's capital opening and terminating period.
func undress(line string) string {
	line = strings.ReplaceAll(line, "**", "")
	line = strings.TrimSuffix(line, ".")
	return strings.ToLower(line[:1]) + line[1:]
}

func TestDigestLinesAreTerminalShaped(t *testing.T) {
	in := digestIn(t,
		journal.NewChange(ts(t, "2026-03-05T09:00:00"), "status", "planned", "in-progress", ""),
		journal.NewChild(ts(t, "2026-03-05T08:30:00"), journal.ChildOpAdded, "q1-growth", "", "", ""),
	)
	days, err := render.Digest(in)
	if err != nil {
		t.Fatal(err)
	}
	if len(days) != 1 {
		t.Fatalf("days: got %d, want 1", len(days))
	}

	// §26's shape: no emphasis, no terminating period, a lower-case opening.
	// No noun on the child line here because the subject is the project, and
	// `projects.acme.q1-growth` is not a position any kind can occupy — §26's
	// "added objective q1-growth" is logged on the `objectives` container,
	// which is the parent that actually gained a child (§3.3).
	want := []string{
		"changed status from planned to in-progress",
		"added q1-growth",
		"created",
	}
	if len(days[0].Lines) != len(want) {
		t.Fatalf("lines: got %d, want %d", len(days[0].Lines), len(want))
	}
	for i, line := range days[0].Lines {
		if line.Text != want[i] {
			t.Errorf("line %d: got %q, want %q", i, line.Text, want[i])
		}
	}
}

// TestDigestCarriesTheEventKind is what `log --kind` and `--json` filter on.
// The `created` line carries none, because it is a field and not an event
// (§3.1).
func TestDigestCarriesTheEventKind(t *testing.T) {
	in := digestIn(t, journal.NewNote(ts(t, "2026-03-05T09:00:00"), "a note"))
	days, err := render.Digest(in)
	if err != nil {
		t.Fatal(err)
	}
	lines := days[0].Lines
	if lines[0].Kind != journal.KindNote || lines[0].At.IsZero() {
		t.Errorf("note line: kind %q at %s", lines[0].Kind, lines[0].At)
	}
	if lines[1].Kind != "" || !lines[1].At.IsZero() {
		t.Errorf("created line: kind %q at %s, want neither", lines[1].Kind, lines[1].At)
	}
}

// TestDigestIsNewestFirst matches the file's direction, and `log`'s (§16.3).
func TestDigestIsNewestFirst(t *testing.T) {
	in := digestIn(t,
		journal.NewNote(ts(t, "2026-03-03T09:00:00"), "oldest"),
		journal.NewNote(ts(t, "2026-03-05T09:00:00"), "newest"),
		journal.NewNote(ts(t, "2026-03-04T09:00:00"), "middle"),
	)
	days, err := render.Digest(in)
	if err != nil {
		t.Fatal(err)
	}
	var dates []string
	for _, d := range days {
		dates = append(dates, d.Date)
	}
	want := []string{"2026-03-05", "2026-03-04", "2026-03-03"}
	for i := range want {
		if dates[i] != want[i] {
			t.Fatalf("days: got %v, want %v", dates, want)
		}
	}
}

// TestActivitySinceNarrowsWhatIsShownNotWhatIsDerived is `activity --since`
// (§16.4): it drops day sections, and it must not change the lines that survive.
func TestActivitySinceNarrowsWhatIsShownNotWhatIsDerived(t *testing.T) {
	in := digestIn(t,
		journal.NewNote(ts(t, "2026-03-03T09:00:00"), "oldest"),
		journal.NewNote(ts(t, "2026-03-05T09:00:00"), "newest"),
	)

	full, err := render.Activity.Render(in)
	if err != nil {
		t.Fatal(err)
	}
	if !contains(string(full), "2026-03-03") {
		t.Fatal("the fixture should span two days")
	}

	narrowed, err := render.ActivityRenderer{Since: "2026-03-05"}.Render(in)
	if err != nil {
		t.Fatal(err)
	}
	if contains(string(narrowed), "2026-03-03") {
		t.Errorf("--since kept a day before it:\n%s", narrowed)
	}
	if !contains(string(narrowed), "newest") {
		t.Errorf("--since dropped the day it asked for:\n%s", narrowed)
	}
	// Every surviving line is byte-identical to the full render's, so nothing
	// was recomputed against a shorter history.
	for _, line := range strings.Split(string(narrowed), "\n") {
		if line == "" || !strings.HasPrefix(line, "- ") {
			continue
		}
		if !contains(string(full), line) {
			t.Errorf("--since changed a line: %q", line)
		}
	}
}

// TestActivitySinceIsRefusedOnTheWritePath: a truncated ACTIVITY.md is a
// projection that disagrees with its journal, which is what doctor exists to
// report (§10) — so the read-time narrowing cannot reach incremental mode.
func TestActivitySinceIsRefusedOnTheWritePath(t *testing.T) {
	in := digestIn(t, journal.NewNote(ts(t, "2026-03-05T09:00:00"), "a note"))
	_, err := render.ActivityRenderer{Days: []string{"2026-03-05"}, Since: "2026-03-05"}.Render(in)
	if err == nil {
		t.Fatal("Since with Days must be refused")
	}
}
