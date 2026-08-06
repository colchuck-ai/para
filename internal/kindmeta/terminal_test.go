package kindmeta_test

import (
	"slices"
	"testing"

	"github.com/colchuck-ai/para/internal/kindmeta"
)

// TestIsTerminal is §1.7's Terminal column, including the one row that is not
// simply "the last values in the vocabulary".
func TestIsTerminal(t *testing.T) {
	cases := []struct {
		kind   kindmeta.Kind
		status string
		want   bool
	}{
		{kindmeta.KindProject, "done", true},
		{kindmeta.KindProject, "dropped", true},
		{kindmeta.KindProject, "blocked", false},
		{kindmeta.KindProject, "in-progress", false},
		{kindmeta.KindObjective, "done", true},
		{kindmeta.KindObjective, "planned", false},
		{kindmeta.KindKeyResult, "achieved", true},
		{kindmeta.KindKeyResult, "dropped", true},
		{kindmeta.KindKeyResult, "on-track", false},
		{kindmeta.KindKeyResult, "at-risk", false},
		// §1.7: "a blown deadline is the one thing that should not be hideable".
		{kindmeta.KindKeyResult, "missed", false},
		// Location is the answer for these, not status (§1.6).
		{kindmeta.KindArea, "done", false},
		{kindmeta.KindResource, "dropped", false},
		{kindmeta.KindSkill, "done", false},
		{kindmeta.KindContainer, "done", false},
	}
	for _, tc := range cases {
		if got := kindmeta.IsTerminal(tc.kind, tc.status); got != tc.want {
			t.Errorf("IsTerminal(%s, %q): got %v, want %v", tc.kind, tc.status, got, tc.want)
		}
	}
}

func TestTerminalStatusesIsASubsetOfWhatTheKindCanHold(t *testing.T) {
	for _, kind := range []kindmeta.Kind{kindmeta.KindProject, kindmeta.KindObjective} {
		settable := kindmeta.SettableStatuses(kind)
		for _, status := range kindmeta.TerminalStatuses(kind) {
			if !slices.Contains(settable, status) {
				t.Errorf("%s: terminal status %q is not one it accepts", kind, status)
			}
		}
	}
	// A key-result's terminal set deliberately spans both: `achieved` is derived
	// and `dropped` is settable (§4.3).
	got := kindmeta.TerminalStatuses(kindmeta.KindKeyResult)
	if !slices.Contains(got, "achieved") || !slices.Contains(got, "dropped") {
		t.Errorf("key-result terminal statuses: got %v", got)
	}
}

func TestTerminalStatusesReturnsACopy(t *testing.T) {
	got := kindmeta.TerminalStatuses(kindmeta.KindProject)
	got[0] = "mutated"
	if kindmeta.TerminalStatuses(kindmeta.KindProject)[0] == "mutated" {
		t.Error("TerminalStatuses handed out its own slice")
	}
}
