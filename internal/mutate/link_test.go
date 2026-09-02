package mutate_test

import (
	"slices"
	"strings"
	"testing"

	"github.com/colchuck-ai/para/internal/mutate"
)

func linkFields(kv ...string) mutate.Fields {
	return fields(append([]string{"type", "jira-epic", "ref", "PROJ-123", "direction", "output"}, kv...)...)
}

// TestAddLinkCreatesItsContainerLazily is para-6g7's central departure from
// key-result's eager objectives/key-results: links/ does not exist until the
// first `add link` under a parent, exactly the way .agents/skills/ is made
// on the way past rather than being a precondition (tree.ParentExists).
func TestAddLinkCreatesItsContainerLazily(t *testing.T) {
	root := plantTree(t)
	e := env(t, root)
	addProject(t, e, "acme")

	before := snapshot(t, root)
	for path := range before {
		if path == "projects/acme/links/.para/state.toml" {
			t.Fatal("links/ already exists before any link was added")
		}
	}

	res, err := e.Add(loc(t, "projects.acme.links.jira-epic"), linkFields("name", "Jira epic"))
	if err != nil {
		t.Fatalf("Add link: %v", err)
	}
	for _, want := range []string{
		"projects/acme/links/jira-epic/.para/state.toml",
		"projects/acme/links/jira-epic/README.md",
		"projects/acme/links/jira-epic/ACTIVITY.md",
		"projects/acme/links/.para/state.toml",
		"projects/acme/links/README.md",
	} {
		if !slices.Contains(res.Wrote, want) {
			t.Errorf("wrote = %v, missing %q", res.Wrote, want)
		}
	}

	got := read(t, root, "projects/acme/links/.para/state.toml")
	if !strings.Contains(got, `name = "Links"`) {
		t.Errorf("links/ container state.toml =\n%s\nwant a name of \"Links\"", got)
	}
	if !strings.Contains(got, "Acme migration") {
		t.Errorf("links/ container state.toml =\n%s\nwant its description to name the parent", got)
	}
}

// TestAddSecondLinkDoesNotRecreateTheContainer: the container is created
// once, not on every add.
func TestAddSecondLinkDoesNotRecreateTheContainer(t *testing.T) {
	root := plantTree(t)
	e := env(t, root)
	addProject(t, e, "acme")
	if _, err := e.Add(loc(t, "projects.acme.links.jira-epic"), linkFields()); err != nil {
		t.Fatalf("Add first link: %v", err)
	}

	res, err := e.Add(loc(t, "projects.acme.links.slack-channel"), linkFields("type", "slack-channel", "ref", "C123"))
	if err != nil {
		t.Fatalf("Add second link: %v", err)
	}
	for _, w := range res.Wrote {
		if w == "projects/acme/links/.para/state.toml" {
			t.Error("adding a second link rewrote the container's own state.toml")
		}
	}
}

// TestAddLinkUnderAreaAtAnyDepth confirms para-6g7's arbitrary-nesting
// requirement: a link attaches under an area at any depth, not just the
// shallowest one, and areas that never get a link never grow a links/
// folder of their own.
func TestAddLinkUnderAreaAtAnyDepth(t *testing.T) {
	root := plantTree(t)
	e := env(t, root)
	addArea(t, e, "areas.health")
	addArea(t, e, "areas.health.training")

	if _, err := e.Add(loc(t, "areas.health.training.links.blog"), linkFields("type", "blog", "ref", "https://example.com")); err != nil {
		t.Fatalf("Add link under nested area: %v", err)
	}

	before := snapshot(t, root)
	if _, ok := before["areas/health/links/.para/state.toml"]; ok {
		t.Error("areas/health grew a links/ container it was never given a link under")
	}
	if _, ok := before["areas/health/training/links/.para/state.toml"]; !ok {
		t.Error("areas/health/training has no links/ container even though it was given a link")
	}
}

// TestAddLinkDefaultsNameToItsID is the one field row (para-6g7) where a
// link's own name is optional, unlike every other kind — the bead's CLI
// shape never types --name.
func TestAddLinkDefaultsNameToItsID(t *testing.T) {
	root := plantTree(t)
	e := env(t, root)
	addProject(t, e, "acme")

	if _, err := e.Add(loc(t, "projects.acme.links.jira-epic"), linkFields()); err != nil {
		t.Fatalf("Add link: %v", err)
	}
	got := read(t, root, "projects/acme/links/jira-epic/.para/state.toml")
	if !strings.Contains(got, `name = "jira-epic"`) {
		t.Errorf("state.toml =\n%s\nwant name defaulted to the id \"jira-epic\"", got)
	}
}

// TestAddLinkRequiresRefAndDirection is §15's row for the two fields with no
// default: type, ref, and direction must all be given.
func TestAddLinkRequiresRefAndDirection(t *testing.T) {
	root := plantTree(t)
	e := env(t, root)
	addProject(t, e, "acme")

	_, err := e.Add(loc(t, "projects.acme.links.jira-epic"), fields("type", "jira-epic", "ref", "PROJ-123"))
	errorContains(t, err, "--direction")

	_, err = e.Add(loc(t, "projects.acme.links.jira-epic"), fields("type", "jira-epic", "direction", "output"))
	errorContains(t, err, "--ref")

	_, err = e.Add(loc(t, "projects.acme.links.jira-epic"), fields("ref", "PROJ-123", "direction", "output"))
	errorContains(t, err, "--type")
}
