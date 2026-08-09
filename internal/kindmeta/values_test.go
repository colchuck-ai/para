package kindmeta_test

import (
	"strings"
	"testing"

	"github.com/colchuck-ai/para/internal/kindmeta"
)

// TestSettableStatuses walks §1.7's table: which statuses a kind will accept
// from `add` and `set`, and in the order an error message lists them.
func TestSettableStatuses(t *testing.T) {
	tests := []struct {
		kind kindmeta.Kind
		want []string
	}{
		{kindmeta.KindProject, []string{"planned", "in-progress", "blocked", "done", "dropped"}},
		{kindmeta.KindObjective, []string{"planned", "in-progress", "blocked", "done", "dropped"}},
		// Derived, with the one settable value carved out (§4.3).
		{kindmeta.KindKeyResult, []string{"dropped"}},
		// "none — location is the answer" (§1.7).
		{kindmeta.KindArea, nil},
		{kindmeta.KindResource, nil},
		{kindmeta.KindSkill, nil},
		{kindmeta.KindContainer, nil},
	}
	for _, tt := range tests {
		got := kindmeta.SettableStatuses(tt.kind)
		if strings.Join(got, ",") != strings.Join(tt.want, ",") {
			t.Errorf("SettableStatuses(%s) = %v, want %v", tt.kind, got, tt.want)
		}
	}
}

func TestDefaultStatus(t *testing.T) {
	tests := []struct {
		kind kindmeta.Kind
		want string
	}{
		{kindmeta.KindProject, "planned"},
		{kindmeta.KindObjective, "planned"},
		// A key-result's status is derived from its measurements, so it has no
		// stored default (§4.3); every other kind has no status at all.
		{kindmeta.KindKeyResult, ""},
		{kindmeta.KindArea, ""},
		{kindmeta.KindSkill, ""},
		{kindmeta.KindContainer, ""},
	}
	for _, tt := range tests {
		if got := kindmeta.DefaultStatus(tt.kind); got != tt.want {
			t.Errorf("DefaultStatus(%s) = %q, want %q", tt.kind, got, tt.want)
		}
	}
}

// TestDefaultStatusIsSettable keeps the two functions from drifting apart: the
// value `add` stores must be one `set` would accept, or the entity is born in a
// state it can never be returned to.
func TestDefaultStatusIsSettable(t *testing.T) {
	for _, kind := range []kindmeta.Kind{
		kindmeta.KindProject, kindmeta.KindArea, kindmeta.KindResource,
		kindmeta.KindObjective, kindmeta.KindKeyResult, kindmeta.KindSkill, kindmeta.KindContainer,
	} {
		def := kindmeta.DefaultStatus(kind)
		if def == "" {
			continue
		}
		if !kindmeta.IsStatus(kind, def) {
			t.Errorf("DefaultStatus(%s) = %q, which SettableStatuses(%s) does not admit", kind, def, kind)
		}
	}
}

func TestIsStatus(t *testing.T) {
	if !kindmeta.IsStatus(kindmeta.KindProject, "blocked") {
		t.Error("blocked should be a project status")
	}
	if kindmeta.IsStatus(kindmeta.KindProject, "on-track") {
		t.Error("on-track is a derived key-result status, not a project's")
	}
	if kindmeta.IsStatus(kindmeta.KindKeyResult, "achieved") {
		t.Error("achieved is derived, not settable (§4.3)")
	}
	if !kindmeta.IsStatus(kindmeta.KindKeyResult, "dropped") {
		t.Error("dropped is the one settable key-result status (§4.3)")
	}
	if kindmeta.IsStatus(kindmeta.KindArea, "planned") {
		t.Error("an area has no status at all (§1.7)")
	}
}

// TestPriorities pins the closed set and, more importantly, its order: §17
// makes priority a --sort key, and a key with no declared order sorts
// lexically, which would put high before low before medium.
func TestPriorities(t *testing.T) {
	want := []string{"high", "medium", "low"}
	got := kindmeta.Priorities()
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Errorf("Priorities() = %v, want %v", got, want)
	}
	for i, p := range want {
		rank, ok := kindmeta.PriorityRank(p)
		if !ok || rank != i {
			t.Errorf("PriorityRank(%q) = %d, %v; want %d, true", p, rank, ok, i)
		}
	}
	if _, ok := kindmeta.PriorityRank("urgent"); ok {
		t.Error("PriorityRank admitted a value outside the set")
	}
}

// TestAllStatuses is the union §1.7's per-kind rows add up to: every status
// some kind accepts, in the table's order, each spelled once. It is what a
// completion offers before a locator has said which kind is being addressed.
func TestAllStatuses(t *testing.T) {
	want := []string{"planned", "in-progress", "blocked", "done", "dropped"}
	got := kindmeta.AllStatuses()
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Fatalf("AllStatuses() = %v, want %v", got, want)
	}

	// The union has to cover every kind, or the completion offers a status the
	// verb would then accept from nobody.
	for _, kind := range []kindmeta.Kind{
		kindmeta.KindProject, kindmeta.KindArea, kindmeta.KindResource,
		kindmeta.KindObjective, kindmeta.KindKeyResult, kindmeta.KindSkill,
	} {
		for _, s := range kindmeta.SettableStatuses(kind) {
			if !strings.Contains(","+strings.Join(got, ",")+",", ","+s+",") {
				t.Errorf("AllStatuses() omits %q, which %v accepts", s, kind)
			}
		}
	}
}
