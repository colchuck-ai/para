package render_test

import (
	"fmt"
	"strings"
	"testing"

	"github.com/colchuck-ai/para/internal/kindmeta"
	"github.com/colchuck-ai/para/internal/mdfile"
	"github.com/colchuck-ai/para/internal/render"
	"github.com/colchuck-ai/para/internal/truth"
)

func TestReadmePreservesTheBodyByteForByte(t *testing.T) {
	// §2.1: para owns frontmatter, humans own bodies. No exceptions.
	body := "\n# My own heading\n\nProse para must not touch.\n\n- a list\n- with items\n"
	existing := "---\nkind: \"area\"\nname: \"Stale\"\n---" + body

	in := render.In{
		Locator:  loc(t, "areas.health"),
		Kind:     kindmeta.KindArea,
		State:    truth.State{Name: "Health", Description: "Staying in one piece.", Created: "2026-01-01"},
		Existing: map[string][]byte{"areas/health/README.md": []byte(existing)},
	}

	got, err := render.Readme.Render(in)
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	if !strings.HasSuffix(string(got), body) {
		t.Errorf("Readme =\n%s\nwant it to end with the existing body\n%s", got, body)
	}
	if !strings.Contains(string(got), `name: "Health"`) {
		t.Errorf("Readme =\n%s\nwant the frontmatter regenerated from state", got)
	}
}

func TestReadmeSurvivesArbitraryRewrites(t *testing.T) {
	// The property that matters on the write path: rendering README.md over its
	// own output is a fixed point, however many times it happens. If it were
	// not, every mutation would churn the file and doctor would report drift on
	// a clean tree (§0.2).
	in := render.In{
		Locator: loc(t, "areas.health"),
		Kind:    kindmeta.KindArea,
		State:   truth.State{Name: "Health", Description: "Staying in one piece.", Created: "2026-01-01"},
	}

	current, err := render.Readme.Render(in)
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	for i := 0; i < 5; i++ {
		in.Existing = map[string][]byte{"areas/health/README.md": current}
		next, err := render.Readme.Render(in)
		if err != nil {
			t.Fatalf("rewrite %d: %v", i, err)
		}
		if string(next) != string(current) {
			t.Fatalf("rewrite %d changed the file:\n%s\nvs\n%s", i, next, current)
		}
	}
}

func TestReadmeAdoptsAFileWithNoFrontmatterAsBody(t *testing.T) {
	// `remove --keep-files` strips the frontmatter block and keeps the body
	// (§18.4). Adding an entity back over that directory must re-adopt the
	// prose rather than bury it under a stub.
	existing := "# Notes I wrote before para\n\nStill mine.\n"
	in := render.In{
		Locator:  loc(t, "resources.notes"),
		Kind:     kindmeta.KindResource,
		State:    truth.State{Name: "Notes", Description: "Collected reading.", Created: "2026-01-01"},
		Existing: map[string][]byte{"resources/notes/README.md": []byte(existing)},
	}

	got, err := render.Readme.Render(in)
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	if !strings.HasSuffix(string(got), existing) {
		t.Errorf("Readme =\n%s\nwant it to end with the pre-existing prose", got)
	}
}

func TestAgentsNeverTouchesAByteOutsideTheMarkers(t *testing.T) {
	// §6, and the property Phase 3's block codec was built for — asserted here
	// against the real prose rather than against a synthetic block.
	house := "\nIn this repo every project links its Jira epic in the README frontmatter.\n"
	existing := mdfile.BeginMarker + "\nwhatever an older para version said\n" + mdfile.EndMarker + "\n" + house

	in := render.In{
		Locator:  loc(t, "projects"),
		Kind:     kindmeta.KindContainer,
		State:    truth.State{Name: "Projects", Created: "2026-01-01"},
		Existing: map[string][]byte{"projects/AGENTS.md": []byte(existing)},
	}

	got, err := render.Agents.Render(in)
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	if !strings.HasSuffix(string(got), house) {
		t.Errorf("Agents =\n%s\nwant it to end with the house rules\n%s", got, house)
	}
	if strings.Contains(string(got), "whatever an older para version said") {
		t.Errorf("Agents =\n%s\nwant the generated block replaced", got)
	}
}

func TestAgentsSurvivesArbitraryRewrites(t *testing.T) {
	in := render.In{
		Locator: loc(t, "areas"),
		Kind:    kindmeta.KindContainer,
		State:   truth.State{Name: "Areas", Created: "2026-01-01"},
	}

	current, err := render.Agents.Render(in)
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	for i := 0; i < 5; i++ {
		in.Existing = map[string][]byte{"areas/AGENTS.md": current}
		next, err := render.Agents.Render(in)
		if err != nil {
			t.Fatalf("rewrite %d: %v", i, err)
		}
		if string(next) != string(current) {
			t.Fatalf("rewrite %d changed the file", i)
		}
	}
}

func TestAgentsIsRefusedWhereItDoesNotBelong(t *testing.T) {
	// Emitting AGENTS.md on an entity would be two more files per project all
	// saying nearly the same thing (§6), so asking for one is a bug in the
	// caller, not a file to write.
	in := render.In{
		Locator: loc(t, "projects.acme"),
		Kind:    kindmeta.KindProject,
		State:   truth.State{Name: "Acme", Created: "2026-01-01"},
	}
	if _, err := render.Agents.Render(in); err == nil {
		t.Error("Agents.Render on a project returned no error")
	}
	if _, err := render.Claude.Render(in); err == nil {
		t.Error("Claude.Render on a project returned no error")
	}
}

func TestGitAttributesKeepsExistingLines(t *testing.T) {
	// §2.2 calls the human-owned part of .gitattributes "append-only to an
	// existing file": a repository that already has one keeps every line.
	existing := "*.png binary\n*.pdf binary\n"

	in := render.In{Kind: kindmeta.KindContainer, Tree: truth.Tree{Schema: 1, Name: "brain"}}
	fresh, err := render.GitAttributes.Render(in)
	if err != nil {
		t.Fatalf("Render: %v", err)
	}

	in.Existing = map[string][]byte{".gitattributes": []byte(existing + string(fresh))}
	got, err := render.GitAttributes.Render(in)
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	if !strings.HasPrefix(string(got), existing) {
		t.Errorf("GitAttributes =\n%s\nwant it to keep the leading human lines", got)
	}
	if got, want := strings.Count(string(got), mdfile.HashMarkers.Begin), 1; got != want {
		t.Errorf("begin marker appears %d times, want %d", got, want)
	}
}

func TestGitAttributesIsRootOnly(t *testing.T) {
	in := render.In{Locator: loc(t, "projects"), Kind: kindmeta.KindContainer}
	if _, err := render.GitAttributes.Render(in); err == nil {
		t.Error("GitAttributes.Render on a bucket returned no error")
	}
}

func TestClaudeImportPathsAreRelativeToTheFileThatCarriesThem(t *testing.T) {
	cases := map[string]string{
		"":                  "@.agents/rules/para-x.md",
		"projects":          "@../.agents/rules/para-x.md",
		"archive.resources": "@../../.agents/rules/para-x.md",
	}
	for locStr, want := range cases {
		t.Run(fmt.Sprintf("at %q", locStr), func(t *testing.T) {
			in := render.In{Kind: kindmeta.KindContainer, Rules: []string{"para-x.md"}}
			if locStr != "" {
				in.Locator = loc(t, locStr)
			}
			got, err := render.Claude.Render(in)
			if err != nil {
				t.Fatalf("Render: %v", err)
			}
			if !strings.Contains(string(got), want+"\n") {
				t.Errorf("Claude =\n%s\nwant it to contain %q", got, want)
			}
		})
	}
}

func TestClaudeSortsAndDeduplicatesItsImports(t *testing.T) {
	in := render.In{
		Kind:  kindmeta.KindContainer,
		Rules: []string{"para-z.md", "para-a.md", "para-z.md"},
	}
	got, err := render.Claude.Render(in)
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	want := "@AGENTS.md\n@.agents/rules/para-a.md\n@.agents/rules/para-z.md\n"
	if string(got) != want {
		t.Errorf("Claude =\n%s\nwant\n%s", got, want)
	}
}
