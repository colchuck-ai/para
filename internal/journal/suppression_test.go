package journal

import "testing"

// TestActiveSuppression walks §28.2's fold: the newest suppress event by
// `at` wins, and an unsuppress — the same kind with `until` empty — clears
// it even when it is the newest event.
func TestActiveSuppression(t *testing.T) {
	first := mustParseAt(t, "2026-08-20T10:00:00-07:00")
	second := mustParseAt(t, "2026-09-01T09:00:00-07:00")

	cases := []struct {
		name   string
		events []Event
		until  string
		note   string
	}{
		{name: "no events", events: nil, until: "", note: ""},
		{
			name:   "one suppress is active",
			events: []Event{NewSuppress(first, "2027-03-01", "paused, resumes with Q1 relaunch")},
			until:  "2027-03-01", note: "paused, resumes with Q1 relaunch",
		},
		{
			name: "a later unsuppress clears it",
			events: []Event{
				NewSuppress(first, "2027-03-01", "paused, resumes with Q1 relaunch"),
				NewSuppress(second, "", "relaunch moved up"),
			},
			until: "", note: "",
		},
		{
			name: "a later suppress wins over an earlier unsuppress",
			events: []Event{
				NewSuppress(first, "", "relaunch moved up"),
				NewSuppress(second, "2027-03-01", "paused again"),
			},
			until: "2027-03-01", note: "paused again",
		},
		{
			name: "other kinds are never mistaken for a suppress",
			events: []Event{
				NewNote(first, "unrelated"),
				NewChange(second, "status", "in-progress", "blocked", ""),
			},
			until: "", note: "",
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			until, note := ActiveSuppression(c.events)
			if until != c.until || note != c.note {
				t.Errorf("ActiveSuppression() = (%q, %q), want (%q, %q)", until, note, c.until, c.note)
			}
		})
	}
}

// TestActiveSuppression_TieKeepsLastEncountered pins the one respect this
// fold deliberately differs from bound()'s shared tie-break (bounds.go):
// suppress/unsuppress take no --at, so two administratively sequential
// calls can tie at the same second, and the slice's own order — prior
// events before the one just added — is already write order even on a
// tie. Keeping the first-encountered event, as bound() does for every other
// caller, would let a same-second unsuppress lose to the suppress it was
// meant to clear.
func TestActiveSuppression_TieKeepsLastEncountered(t *testing.T) {
	tie := mustParseAt(t, "2026-08-20T10:00:00-07:00")
	events := []Event{
		NewSuppress(tie, "2027-01-01", "first"),
		NewSuppress(tie, "2027-06-01", "second"),
	}
	until, note := ActiveSuppression(events)
	if until != "2027-06-01" || note != "second" {
		t.Errorf("ActiveSuppression() = (%q, %q), want the last-encountered tie (2027-06-01, second)", until, note)
	}
}

// TestActiveSuppression_SameSecondUnsuppressClearsASuppress is the exact
// hazard the tie-break above exists to close: a suppress already on disk
// (prior) and an unsuppress landing in the same wall-clock second (added)
// must still clear it, because the unsuppress genuinely happened after —
// merge() (mutate.go) puts added events after prior ones for a tied `at`,
// so this is the shape a real same-second suppress-then-unsuppress
// produces.
func TestActiveSuppression_SameSecondUnsuppressClearsASuppress(t *testing.T) {
	tie := mustParseAt(t, "2026-08-20T10:00:00-07:00")
	prior := []Event{NewSuppress(tie, "2027-01-01", "paused")}
	added := []Event{NewSuppress(tie, "", "resumed")}

	until, note := ActiveSuppression(append(prior, added...))
	if until != "" || note != "" {
		t.Errorf("ActiveSuppression() = (%q, %q), want cleared by the same-second unsuppress", until, note)
	}
}
