package mutate_test

import (
	"strings"
	"testing"
	"time"

	"github.com/colchuck-ai/para/internal/clock"
	"github.com/colchuck-ai/para/internal/mutate"
)

// TestSuppressWritesEventAndCachesSuppression is §28.2/§28.4's happy path: a
// suppress event lands in the journal and its until/note are cached into
// state.toml's [suppression] table.
func TestSuppressWritesEventAndCachesSuppression(t *testing.T) {
	root := plantTree(t)
	e := env(t, root)
	addProject(t, e, "acme")

	res, err := e.Suppress(loc(t, "projects.acme"), "2027-03-01", "paused, resumes with Q1 relaunch")
	if err != nil {
		t.Fatalf("Suppress: %v", err)
	}
	assertEqual(t, "wrote", res.Wrote, []string{
		"projects/acme/.para/logs/20260305T170000Z.jsonl",
		"projects/acme/.para/state.toml",
		"projects/acme/ACTIVITY.md",
	})

	got := read(t, root, "projects/acme/.para/logs/20260305T170000Z.jsonl")
	if !strings.Contains(got, `"kind":"suppress"`) || !strings.Contains(got, `"value":"2027-03-01"`) {
		t.Errorf("journal line = %q, want a suppress event carrying until", got)
	}

	state := read(t, root, "projects/acme/.para/state.toml")
	if !strings.Contains(state, "[suppression]") ||
		!strings.Contains(state, `until = "2027-03-01"`) ||
		!strings.Contains(state, `note = "paused, resumes with Q1 relaunch"`) {
		t.Errorf("state.toml = %q, want [suppression] cached", state)
	}

	activity := read(t, root, "projects/acme/ACTIVITY.md")
	if !strings.Contains(activity, "Suppressed until 2027-03-01 — paused, resumes with Q1 relaunch") {
		t.Errorf("ACTIVITY.md = %q, want a Suppressed line naming until and the reason", activity)
	}
}

// TestSuppressAgainSameSecondCorrectsTheUntil is the suppress-side twin of
// TestUnsuppressClearsTheCachedSuppression: two suppress calls sharing one
// env's `now()` tie by `at`, and the second (a correction to a different
// date) must win over the first, not lose to it.
func TestSuppressAgainSameSecondCorrectsTheUntil(t *testing.T) {
	root := plantTree(t)
	e := env(t, root)
	addProject(t, e, "acme")

	if _, err := e.Suppress(loc(t, "projects.acme"), "2027-03-01", "paused"); err != nil {
		t.Fatalf("Suppress: %v", err)
	}
	if _, err := e.Suppress(loc(t, "projects.acme"), "2027-06-01", "pushed further out"); err != nil {
		t.Fatalf("Suppress: %v", err)
	}

	state := read(t, root, "projects/acme/.para/state.toml")
	if !strings.Contains(state, `until = "2027-06-01"`) || !strings.Contains(state, `note = "pushed further out"`) {
		t.Errorf("state.toml = %q, want the second, same-second suppress to win", state)
	}
}

// TestSuppressNeverMovesAttentionPastWhatIsAlreadyCached is §28.2: suppression
// and attention are independent facts, so suppressing must not itself move an
// already-cached attention clock forward to the suppress event's own instant.
func TestSuppressNeverMovesAttentionPastWhatIsAlreadyCached(t *testing.T) {
	root := plantTree(t)
	e := env(t, root)
	addProject(t, e, "acme")
	// Backdated so the note below can land after created (this test's fixed
	// now, 2026-03-05) without being refused as being in the future.
	if _, err := e.Set(loc(t, "projects.acme"), fields("created", "2026-01-01"), ""); err != nil {
		t.Fatalf("Set created: %v", err)
	}
	if _, err := e.Note(loc(t, "projects.acme"), "an ordinary note", "2026-03-01", false); err != nil {
		t.Fatalf("Note: %v", err)
	}

	if _, err := e.Suppress(loc(t, "projects.acme"), "2027-03-01", "paused"); err != nil {
		t.Fatalf("Suppress: %v", err)
	}
	// state.toml is rewritten regardless — this suppression is being cached
	// for the first time — but attention within it must not have moved.
	state := read(t, root, "projects/acme/.para/state.toml")
	if !strings.Contains(state, `attention = "2026-03-01T08:00:00Z"`) {
		t.Errorf("state.toml = %q, want attention still at the note's instant, not suppress's own", state)
	}
}

// TestSuppressAcceptsUntilsProgressivePrecision proves --until takes due's
// grammar (§15.1, §18.7): a bare year or year-month is legal.
func TestSuppressAcceptsUntilsProgressivePrecision(t *testing.T) {
	root := plantTree(t)
	e := env(t, root)
	addProject(t, e, "acme")

	if _, err := e.Suppress(loc(t, "projects.acme"), "2027", "a whole year out"); err != nil {
		t.Errorf("Suppress with a bare year: %v", err)
	}
	state := read(t, root, "projects/acme/.para/state.toml")
	if !strings.Contains(state, `until = "2027"`) {
		t.Errorf("state.toml = %q, want until stored exactly as typed", state)
	}
}

func TestSuppressRequiresUntil(t *testing.T) {
	root := plantTree(t)
	e := env(t, root)
	addProject(t, e, "acme")

	_, err := e.Suppress(loc(t, "projects.acme"), "", "a note")
	errorContains(t, err, "--until is required")
}

func TestSuppressRequiresNote(t *testing.T) {
	root := plantTree(t)
	e := env(t, root)
	addProject(t, e, "acme")

	_, err := e.Suppress(loc(t, "projects.acme"), "2027-03-01", "")
	errorContains(t, err, "--note is required")
}

func TestSuppressRefusesAnInvalidUntil(t *testing.T) {
	root := plantTree(t)
	e := env(t, root)
	addProject(t, e, "acme")

	_, err := e.Suppress(loc(t, "projects.acme"), "not-a-date", "a note")
	errorContains(t, err, "until:")
}

// TestUnsuppressClearsTheCachedSuppression is §28.2's "no second kind": the
// same event kind with until empty folds to no active suppression.
//
// Both calls share one env's `now()` deliberately: suppress/unsuppress take
// no --at, so two calls issued back to back — an ordinary, legal sequence —
// tie by `at`. journal.ActiveSuppression breaks that tie by keeping the
// LAST event encountered rather than the first (the one respect it departs
// from bound()'s shared tie-break, bounds.go), precisely so this same-second
// unsuppress is the one that wins and actually clears the cache.
func TestUnsuppressClearsTheCachedSuppression(t *testing.T) {
	root := plantTree(t)
	e := env(t, root)
	addProject(t, e, "acme")

	if _, err := e.Suppress(loc(t, "projects.acme"), "2027-03-01", "paused, resumes with Q1 relaunch"); err != nil {
		t.Fatalf("Suppress: %v", err)
	}

	res, err := e.Unsuppress(loc(t, "projects.acme"), "relaunch moved up")
	if err != nil {
		t.Fatalf("Unsuppress: %v", err)
	}
	assertEqual(t, "wrote", res.Wrote, []string{
		"projects/acme/.para/logs/20260305T170000Z.jsonl",
		"projects/acme/.para/state.toml",
		"projects/acme/ACTIVITY.md",
	})

	state := read(t, root, "projects/acme/.para/state.toml")
	if strings.Contains(state, "[suppression]") {
		t.Errorf("state.toml = %q, want [suppression] gone after unsuppress", state)
	}

	// Both events land in the same file — rotation is by size, not by day
	// (§3.4) — so it is the *last* line, not the file, that must show the
	// clearing unsuppress.
	got := read(t, root, "projects/acme/.para/logs/20260305T170000Z.jsonl")
	lines := strings.Split(strings.TrimRight(got, "\n"), "\n")
	last := lines[len(lines)-1]
	if !strings.Contains(last, `"kind":"suppress"`) || strings.Contains(last, `"value"`) {
		t.Errorf("last journal line = %q, want a suppress event with no value", last)
	}
}

func TestUnsuppressRequiresNote(t *testing.T) {
	root := plantTree(t)
	e := env(t, root)
	addProject(t, e, "acme")

	_, err := e.Unsuppress(loc(t, "projects.acme"), "")
	errorContains(t, err, "--note is required")
}

// TestSuppressTouchesOnlyItsOwnSubject proves a suppress on one project
// writes nothing under a sibling's directory — the same "nothing walks a
// subtree" invariant §2.3 already holds for every other verb.
func TestSuppressTouchesOnlyItsOwnSubject(t *testing.T) {
	root := plantTree(t)
	e := env(t, root)
	addProject(t, e, "acme")
	addProject(t, e, "other")
	before := snapshot(t, root)

	if _, err := e.Suppress(loc(t, "projects.acme"), "2027-03-01", "paused"); err != nil {
		t.Fatalf("Suppress: %v", err)
	}
	for _, path := range changedPaths(t, before, snapshot(t, root)) {
		if strings.Contains(path, "/other/") {
			t.Errorf("suppressing acme changed %s, a sibling's file", path)
		}
	}
}

// TestSuppressAgainIdenticalNoOpSkipsTheWrite proves writeThroughCache's
// "only write if something actually differs" rule holds for suppression,
// not only for attention (TestNoteNoAttentionLeavesAnAlreadyCachedAttentionAlone
// covers that side): repeating the exact same until/note at a later,
// non-tied instant must not rewrite state.toml.
func TestSuppressAgainIdenticalNoOpSkipsTheWrite(t *testing.T) {
	root := plantTree(t)
	e := env(t, root)
	addProject(t, e, "acme")
	if _, err := e.Suppress(loc(t, "projects.acme"), "2027-03-01", "paused"); err != nil {
		t.Fatalf("Suppress: %v", err)
	}

	later := mutate.NewEnv(root, clock.Fixed{At: now().Add(24 * time.Hour)})
	res, err := later.Suppress(loc(t, "projects.acme"), "2027-03-01", "paused")
	if err != nil {
		t.Fatalf("Suppress: %v", err)
	}
	for _, path := range res.Wrote {
		if strings.HasSuffix(path, "state.toml") {
			t.Errorf("wrote %v, want state.toml untouched — until/note are identical to what is already cached", res.Wrote)
		}
	}
}

// TestSuppressWorksOnASkill is §18.7: legal on every addressable kind,
// entities and skills alike.
func TestSuppressWorksOnASkill(t *testing.T) {
	root := plantTree(t)
	e := env(t, root)
	if _, err := e.Add(loc(t, "skills.report"), fields("name", "Report", "description", "when asked")); err != nil {
		t.Fatalf("Add skill: %v", err)
	}

	if _, err := e.Suppress(loc(t, "skills.report"), "2027-03-01", "on hold"); err != nil {
		t.Fatalf("Suppress: %v", err)
	}
	state := read(t, root, ".agents/skills/para-report/.para/state.toml")
	if !strings.Contains(state, `until = "2027-03-01"`) {
		t.Errorf("state.toml = %q, want [suppression] cached for a skill too", state)
	}
}
