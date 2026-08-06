package writeset_test

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/colchuck-ai/para/internal/journal"
	"github.com/colchuck-ai/para/internal/kindmeta"
	"github.com/colchuck-ai/para/internal/locator"
	"github.com/colchuck-ai/para/internal/render"
	"github.com/colchuck-ai/para/internal/truth"
	"github.com/colchuck-ai/para/internal/writeset"
)

// TestRenderThenApplyIsTheWholeWritePath composes the two halves of the
// projection engine the way a mutation will (Phase 8): render every artifact the
// subject owns, then write the set in truth-first order. It is here rather than
// in render because it is the only test where the slash-separated, root-relative
// paths render produces meet the OS paths writeset consumes, and a mismatch
// between the two would otherwise only surface in a much later phase.
func TestRenderThenApplyIsTheWholeWritePath(t *testing.T) {
	root := t.TempDir()

	in := render.In{
		Locator: locator.Locator{"projects", "acme-migration"},
		Kind:    kindmeta.KindProject,
		State: truth.State{
			Name:        "Acme migration",
			Description: "Rebuild the consumer.",
			Status:      "in-progress",
			Created:     "2026-03-05T08:00:00-08:00",
		},
		Events: []journal.Event{journal.NewNote(at(t, "2026-03-05T09:00:00"), "kickoff")},
		Config: render.DefaultConfig(),
	}

	artifacts, err := render.Artifacts(in)
	if err != nil {
		t.Fatalf("Artifacts: %v", err)
	}
	state, err := truth.EncodeState(in.State)
	if err != nil {
		t.Fatalf("EncodeState: %v", err)
	}

	entityDir := filepath.Join(root, "projects", "acme-migration")
	projections := make([]writeset.File, 0, len(artifacts))
	for _, a := range artifacts {
		projections = append(projections, writeset.File{
			Path:  filepath.Join(root, filepath.FromSlash(a.Path)),
			Bytes: a.Bytes,
		})
	}

	if _, err := writeset.Apply(writeset.Mutation{
		Subjects: []writeset.Subject{{
			Dir:         entityDir,
			Events:      in.Events,
			State:       state,
			Projections: projections,
		}},
		Parents: []writeset.Subject{{
			Dir:    filepath.Join(root, "projects"),
			Events: []journal.Event{journal.NewChild(at(t, "2026-03-05T09:00:00"), journal.ChildOpAdded, "acme-migration", "", "", "")},
		}},
	}); err != nil {
		t.Fatalf("Apply: %v", err)
	}

	got := make([]string, 0)
	for path := range snapshot(t, root) {
		got = append(got, path)
	}
	sort.Strings(got)
	want := []string{
		"projects/.para/logs/20260305T170000Z.jsonl",
		"projects/acme-migration/.para/logs/20260305T170000Z.jsonl",
		"projects/acme-migration/.para/state.toml",
		"projects/acme-migration/ACTIVITY.md",
		"projects/acme-migration/README.md",
	}
	if strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Errorf("tree =\n  %v\nwant\n  %v", got, want)
	}

	// The bytes on disk are the bytes the renderer produced — the equality
	// doctor's byte-for-byte comparison depends on (§10).
	for _, a := range artifacts {
		onDisk, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(a.Path)))
		if err != nil {
			t.Fatalf("reading %s: %v", a.Path, err)
		}
		if string(onDisk) != string(a.Bytes) {
			t.Errorf("%s on disk differs from what was rendered:\n%s\nvs\n%s", a.Path, onDisk, a.Bytes)
		}
	}

	// And re-rendering with what is now on disk changes nothing, which is what
	// makes rebuild idempotent and doctor quiet on a clean tree (§2.4, §21.1).
	in.Existing = map[string][]byte{}
	for _, a := range artifacts {
		in.Existing[a.Path] = a.Bytes
	}
	again, err := render.Artifacts(in)
	if err != nil {
		t.Fatalf("Artifacts: %v", err)
	}
	for i := range again {
		if string(again[i].Bytes) != string(artifacts[i].Bytes) {
			t.Errorf("%s is not a fixed point:\n%s\nvs\n%s", again[i].Path, again[i].Bytes, artifacts[i].Bytes)
		}
	}
}
