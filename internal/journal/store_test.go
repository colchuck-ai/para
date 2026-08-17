package journal

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func mustAppend(t *testing.T, dir string, e Event, rotateBytes int64) string {
	t.Helper()
	path, err := Append(dir, e, rotateBytes)
	if err != nil {
		t.Fatalf("Append: %v", err)
	}
	return path
}

// TestAppend_ReportsTheFileTheLineLandedIn pins the return value §23's output
// depends on: a mutation prints every file it wrote by name, and which journal
// file an event lands in is decided inside Append by the newest file's size.
func TestAppend_ReportsTheFileTheLineLandedIn(t *testing.T) {
	dir := t.TempDir()
	first := time.Date(2026, time.March, 5, 17, 0, 0, 0, time.UTC)
	second := first.Add(time.Hour)

	firstPath := mustAppend(t, dir, Event{At: first, Kind: KindNote, Note: "first"}, DefaultRotateBytes)
	if want := filepath.Join(dir, "20260305T170000Z.jsonl"); firstPath != want {
		t.Errorf("first Append = %q, want %q", firstPath, want)
	}

	// A second event below the threshold lands in the same file...
	samePath := mustAppend(t, dir, Event{At: second, Kind: KindNote, Note: "second"}, DefaultRotateBytes)
	if samePath != firstPath {
		t.Errorf("second Append = %q, want the same file %q", samePath, firstPath)
	}

	// ... and one past it names the new file, which is the case a caller
	// cannot compute for itself.
	rotated := mustAppend(t, dir, Event{At: second, Kind: KindNote, Note: "third"}, 1)
	if want := filepath.Join(dir, "20260305T180000Z.jsonl"); rotated != want {
		t.Errorf("rotated Append = %q, want %q", rotated, want)
	}
}

func listJSONLNames(t *testing.T, dir string) []string {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("ReadDir: %v", err)
	}
	var names []string
	for _, e := range entries {
		names = append(names, e.Name())
	}
	return names
}

// TestAppend_FirstEventCreatesFileNamedForItself pins §3.4's naming rule on
// an empty logs/ directory.
func TestAppend_FirstEventCreatesFileNamedForItself(t *testing.T) {
	dir := t.TempDir()
	at := mustParseAt(t, "2026-01-01T08:08:01-08:00")
	mustAppend(t, dir, Event{At: at, Kind: KindNote, Note: "first"}, DefaultRotateBytes)

	names := listJSONLNames(t, dir)
	if len(names) != 1 || names[0] != "20260101T160801Z.jsonl" {
		t.Fatalf("listJSONL = %v, want exactly [20260101T160801Z.jsonl]", names)
	}
}

// TestAppend_StaysInOneFileUnderThreshold proves ordinary appends land in
// the same file rather than rotating on every write.
func TestAppend_StaysInOneFileUnderThreshold(t *testing.T) {
	dir := t.TempDir()
	base := mustParseAt(t, "2026-01-01T08:00:00-08:00")

	for i := 0; i < 5; i++ {
		mustAppend(t, dir, Event{At: base.Add(time.Duration(i) * time.Minute), Kind: KindNote, Note: "n"}, DefaultRotateBytes)
	}

	names := listJSONLNames(t, dir)
	if len(names) != 1 {
		t.Fatalf("listJSONL = %v, want exactly one file", names)
	}

	events, err := ReadAll(dir)
	if err != nil {
		t.Fatalf("ReadAll: %v", err)
	}
	if len(events) != 5 {
		t.Fatalf("ReadAll returned %d events, want 5", len(events))
	}
}

// TestAppend_RotatesOnSize proves a new file opens once the newest file's
// size meets rotateBytes, and that the closed file's bytes never change
// afterward (§3.4: "closed journal files are immutable").
func TestAppend_RotatesOnSize(t *testing.T) {
	dir := t.TempDir()
	first := mustParseAt(t, "2026-01-01T08:00:00-08:00")
	second := mustParseAt(t, "2026-01-02T09:00:00-08:00")

	mustAppend(t, dir, Event{At: first, Kind: KindNote, Note: "first"}, 10)

	firstPath := filepath.Join(dir, "20260101T160000Z.jsonl")
	before, err := os.ReadFile(firstPath)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}

	mustAppend(t, dir, Event{At: second, Kind: KindNote, Note: "second"}, 10)

	names := listJSONLNames(t, dir)
	if len(names) != 2 {
		t.Fatalf("listJSONL = %v, want two files after rotation", names)
	}

	secondPath := filepath.Join(dir, "20260102T170000Z.jsonl")
	if _, err := os.Stat(secondPath); err != nil {
		t.Fatalf("expected rotation to create %s: %v", secondPath, err)
	}

	after, err := os.ReadFile(firstPath)
	if err != nil {
		t.Fatalf("ReadFile after rotation: %v", err)
	}
	if string(before) != string(after) {
		t.Errorf("closed file changed after rotation:\nbefore: %q\nafter:  %q", before, after)
	}
}

// TestAppend_RotationBoundary pins §3.4's exact wording — "grows until it
// exceeds log.rotate-bytes, at which point the next event opens a new
// file" — at the byte boundary: a file sized exactly at rotateBytes has not
// yet exceeded it and still takes the next event; sized one byte over, it
// has, and the next event rotates.
func TestAppend_RotationBoundary(t *testing.T) {
	dir := t.TempDir()
	first := mustParseAt(t, "2026-01-01T08:00:00-08:00")
	second := mustParseAt(t, "2026-01-02T09:00:00-08:00")

	mustAppend(t, dir, Event{At: first, Kind: KindNote, Note: "first"}, DefaultRotateBytes)
	firstPath := filepath.Join(dir, "20260101T160000Z.jsonl")
	size, err := fileSize(firstPath)
	if err != nil {
		t.Fatalf("fileSize: %v", err)
	}

	t.Run("exactly at threshold stays in the same file", func(t *testing.T) {
		mustAppend(t, dir, Event{At: second, Kind: KindNote, Note: "second"}, size)
		if names := listJSONLNames(t, dir); len(names) != 1 {
			t.Fatalf("listJSONL = %v, want exactly one file (threshold not yet exceeded)", names)
		}
	})
}

func TestAppend_RotationBoundary_OneByteOverRotates(t *testing.T) {
	dir := t.TempDir()
	first := mustParseAt(t, "2026-01-01T08:00:00-08:00")
	second := mustParseAt(t, "2026-01-02T09:00:00-08:00")

	mustAppend(t, dir, Event{At: first, Kind: KindNote, Note: "first"}, DefaultRotateBytes)
	firstPath := filepath.Join(dir, "20260101T160000Z.jsonl")
	size, err := fileSize(firstPath)
	if err != nil {
		t.Fatalf("fileSize: %v", err)
	}

	mustAppend(t, dir, Event{At: second, Kind: KindNote, Note: "second"}, size-1)
	if names := listJSONLNames(t, dir); len(names) != 2 {
		t.Fatalf("listJSONL = %v, want two files (threshold exceeded by one byte)", names)
	}
}

// TestAppend_RotationNamesFileForItsOwnFirstEvent proves the new file after
// rotation is named for the event that opened it, not the one that
// triggered rotation retroactively renaming anything.
func TestAppend_RotationNamesFileForItsOwnFirstEvent(t *testing.T) {
	dir := t.TempDir()
	first := mustParseAt(t, "2026-01-01T08:00:00-08:00")
	second := mustParseAt(t, "2026-01-05T00:00:00-08:00")
	third := mustParseAt(t, "2026-01-05T00:00:05-08:00")

	mustAppend(t, dir, Event{At: first, Kind: KindNote, Note: "a"}, 10)
	mustAppend(t, dir, Event{At: second, Kind: KindNote, Note: "b"}, 10)
	mustAppend(t, dir, Event{At: third, Kind: KindNote, Note: "c"}, 10)

	names := listJSONLNames(t, dir)
	if len(names) != 3 {
		t.Fatalf("listJSONL = %v, want three files (each write exceeded the tiny threshold)", names)
	}
	want := []string{"20260101T160000Z.jsonl", "20260105T080000Z.jsonl", "20260105T080005Z.jsonl"}
	for i, w := range want {
		if names[i] != w {
			t.Errorf("names[%d] = %q, want %q", i, names[i], w)
		}
	}
}

// TestPlanAppend_RotatesMidBatchLikeSequentialAppendsWould is the case a
// caller cannot get right by calling targetFile once and reusing the answer:
// a batch of events landing in the *same* journal within one mutation (e.g.
// `set` changing several fields at once) must roll over exactly where a real
// sequential run of Append calls would, including a second rotation inside
// the same batch — not just the first.
//
// RotateBytes: 1 makes the boundary trivial to reason about without knowing
// any event's exact encoded size: whatever lands in a file, that file is now
// over the threshold, so the very next event always opens a new one.
func TestPlanAppend_RotatesMidBatchLikeSequentialAppendsWould(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "logs")
	base := mustParseAt(t, "2026-01-01T08:00:00-08:00")
	events := []Event{
		{At: base, Kind: KindNote, Note: "first"},
		{At: base.Add(time.Minute), Kind: KindNote, Note: "second"},
		{At: base.Add(2 * time.Minute), Kind: KindNote, Note: "third"},
	}

	planned, err := PlanAppend(dir, events, 1)
	if err != nil {
		t.Fatalf("PlanAppend: %v", err)
	}
	want := []string{
		filepath.Join(dir, "20260101T160000Z.jsonl"),
		filepath.Join(dir, "20260101T160100Z.jsonl"),
		filepath.Join(dir, "20260101T160200Z.jsonl"),
	}
	if len(planned) != len(want) {
		t.Fatalf("PlanAppend = %v, want %v", planned, want)
	}
	for i := range want {
		if planned[i] != want[i] {
			t.Errorf("planned[%d] = %q, want %q", i, planned[i], want[i])
		}
	}

	// Nothing was created: the directory does not even exist yet.
	if _, err := os.Stat(dir); !os.IsNotExist(err) {
		t.Errorf("PlanAppend created %s", dir)
	}

	// And a real sequential run, at the same threshold, agrees file for file —
	// the plan is not merely plausible, it is what Append actually does.
	// (Append itself never creates its directory — that is appendAll's job in
	// production — so the test does it here instead.)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	for i, e := range events {
		if got := mustAppend(t, dir, e, 1); got != want[i] {
			t.Errorf("Append[%d] landed in %q, want %q", i, got, want[i])
		}
	}
}

// TestPlanAppend_StartsFromRealOnDiskStateWithoutWriting proves the planner
// reads the newest file's actual size to decide the first event's home — the
// part it cannot get from rotateBytes alone — and touches nothing while doing
// it.
func TestPlanAppend_StartsFromRealOnDiskStateWithoutWriting(t *testing.T) {
	dir := t.TempDir()
	base := mustParseAt(t, "2026-01-01T08:00:00-08:00")

	seededPath := mustAppend(t, dir, Event{At: base, Kind: KindNote, Note: "seed"}, DefaultRotateBytes)
	before, err := os.ReadFile(seededPath)
	if err != nil {
		t.Fatal(err)
	}
	seedSize, err := fileSize(seededPath)
	if err != nil {
		t.Fatal(err)
	}

	// A threshold at exactly the seeded size: the boundary test above already
	// pins "at the threshold stays" for Append itself, so reusing that value
	// here proves PlanAppend reads the same real size Append would have.
	next := Event{At: base.Add(time.Minute), Kind: KindNote, Note: "next"}
	planned, err := PlanAppend(dir, []Event{next}, seedSize)
	if err != nil {
		t.Fatalf("PlanAppend: %v", err)
	}
	if len(planned) != 1 || planned[0] != seededPath {
		t.Fatalf("PlanAppend = %v, want [%s] (at the threshold, stays in the seeded file)", planned, seededPath)
	}

	after, err := os.ReadFile(seededPath)
	if err != nil {
		t.Fatal(err)
	}
	if string(before) != string(after) {
		t.Errorf("PlanAppend changed the seeded file's bytes")
	}
	if names := listJSONLNames(t, dir); len(names) != 1 {
		t.Errorf("PlanAppend left %v, want exactly the one seeded file", names)
	}
}

func TestPlanAppend_EmptyEventsReturnsNil(t *testing.T) {
	dir := t.TempDir()
	planned, err := PlanAppend(dir, nil, DefaultRotateBytes)
	if err != nil {
		t.Fatalf("PlanAppend: %v", err)
	}
	if planned != nil {
		t.Errorf("PlanAppend(nil) = %v, want nil", planned)
	}
	if names := listJSONLNames(t, dir); len(names) != 0 {
		t.Errorf("PlanAppend on an empty batch created %v", names)
	}
}

// TestPlanAppend_OnAnAbsentDirMatchesFirstAppend proves the planner handles a
// brand-new journal — no newest file to stat at all — the same way Append's
// own first call does.
func TestPlanAppend_OnAnAbsentDirMatchesFirstAppend(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "logs")
	at := mustParseAt(t, "2026-01-01T08:08:01-08:00")

	planned, err := PlanAppend(dir, []Event{{At: at, Kind: KindNote, Note: "first"}}, DefaultRotateBytes)
	if err != nil {
		t.Fatalf("PlanAppend: %v", err)
	}
	want := filepath.Join(dir, "20260101T160801Z.jsonl")
	if len(planned) != 1 || planned[0] != want {
		t.Fatalf("PlanAppend = %v, want [%s]", planned, want)
	}
	if _, err := os.Stat(dir); !os.IsNotExist(err) {
		t.Errorf("PlanAppend on an absent dir created it")
	}
}

// TestReadAll_OrdersByAtNotFilePosition builds two files where lexical
// (chronological-by-name) file order disagrees with the events' own `at`
// values, and asserts ReadAll still returns them ordered by `at` (§3.1).
func TestReadAll_OrdersByAtNotFilePosition(t *testing.T) {
	dir := t.TempDir()

	// Later file, but its one event predates everything in the earlier file.
	late := mustParseAt(t, "2026-01-01T00:00:00-08:00")
	writeRawFile(t, dir, "20260201T080000Z.jsonl", Event{At: late, Kind: KindNote, Note: "actually earliest"})

	// Earlier file, containing events that are individually out of order
	// within the file too.
	e1 := mustParseAt(t, "2026-01-10T00:00:00-08:00")
	e2 := mustParseAt(t, "2026-01-05T00:00:00-08:00")
	writeRawFile(t, dir, "20260101T080000Z.jsonl",
		Event{At: e1, Kind: KindNote, Note: "second-file second-line but later at"},
		Event{At: e2, Kind: KindNote, Note: "second-file first-line but earlier at"},
	)

	events, err := ReadAll(dir)
	if err != nil {
		t.Fatalf("ReadAll: %v", err)
	}
	if len(events) != 3 {
		t.Fatalf("ReadAll returned %d events, want 3", len(events))
	}
	for i := 1; i < len(events); i++ {
		if events[i].At.Before(events[i-1].At) {
			t.Fatalf("events not ordered by At: %+v", events)
		}
	}
	if !events[0].At.Equal(late) {
		t.Errorf("events[0].At = %v, want %v (the file-order-defying earliest event)", events[0].At, late)
	}
}

func TestReadAll_AbsentDirReportsNoEvents(t *testing.T) {
	events, err := ReadAll(filepath.Join(t.TempDir(), "does-not-exist"))
	if err != nil {
		t.Fatalf("ReadAll on an absent dir: %v", err)
	}
	if events != nil {
		t.Errorf("ReadAll on an absent dir = %v, want nil", events)
	}
}

func writeRawFile(t *testing.T, dir, name string, events ...Event) {
	t.Helper()
	var data []byte
	for _, e := range events {
		line, err := Encode(e)
		if err != nil {
			t.Fatalf("Encode: %v", err)
		}
		data = append(data, line...)
	}
	if err := os.WriteFile(filepath.Join(dir, name), data, 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
}
