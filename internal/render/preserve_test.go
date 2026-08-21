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

func TestWithoutClaudeBlockRemovesOnlyTheBlock(t *testing.T) {
	// Render's other half (§6.1, Phase 25's consumer): removing para's block
	// leaves the rest of the file exactly where it was, and reports found so a
	// caller can tell a block-bearing file from one para never touched.
	prefix := "# Team notes\n\n"
	suffix := "\n## More notes\n\nDon't touch this.\n"
	block := mdfile.BeginMarker + "\n@AGENTS.md\n@../.agents/rules/para-x.md\n" + mdfile.EndMarker
	existing := prefix + block + suffix

	got, found, err := render.WithoutClaudeBlock("projects/CLAUDE.md", []byte(existing))
	if err != nil {
		t.Fatalf("WithoutClaudeBlock: %v", err)
	}
	if !found {
		t.Fatal("found = false, want true")
	}
	// The newline that closed the end marker's own line goes with the block
	// (mdfile.RemoveDelimited's doc comment), so it is one of suffix's two
	// leading newlines, not the prefix's blank line, that disappears.
	if want := prefix + strings.TrimPrefix(suffix, "\n"); string(got) != want {
		t.Errorf("WithoutClaudeBlock =\n%s\nwant\n%s", got, want)
	}
}

func TestWithoutClaudeBlockLeavesAFileParaNeverWroteAlone(t *testing.T) {
	// R7: no begin marker at all is not damage — para simply has no block in
	// this file — and it comes back unchanged with found false.
	foreign := "# Team notes\n\nRun `make check` before every commit.\n"
	got, found, err := render.WithoutClaudeBlock("projects/CLAUDE.md", []byte(foreign))
	if err != nil {
		t.Fatalf("WithoutClaudeBlock: %v", err)
	}
	if found {
		t.Error("found = true, want false")
	}
	if string(got) != foreign {
		t.Errorf("WithoutClaudeBlock =\n%s\nwant it unchanged\n%s", got, foreign)
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
	block := "@AGENTS.md\n@.agents/rules/para-a.md\n@.agents/rules/para-z.md\n"
	want := mdfile.BeginMarker + "\n" + block + mdfile.EndMarker + "\n"
	if string(got) != want {
		t.Errorf("Claude =\n%s\nwant\n%s", got, want)
	}
}

// The four shapes CLAUDE.md can be in when Render runs (§2.2's interop test,
// R1-R3): absent, foreign content with no block, para's own block sitting
// mid-file, and a foreign marker-delimited block that happens to look like
// para's (the beads `<!-- BEGIN BEADS INTEGRATION -->` shape).

func TestClaudeWithNoExistingFileContainsOnlyTheBlock(t *testing.T) {
	in := render.In{Kind: kindmeta.KindContainer, Rules: []string{"para-x.md"}}
	got, err := render.Claude.Render(in)
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	want := mdfile.BeginMarker + "\n@AGENTS.md\n@.agents/rules/para-x.md\n" + mdfile.EndMarker + "\n"
	if string(got) != want {
		t.Errorf("Claude =\n%s\nwant\n%s", got, want)
	}
}

func TestClaudeAppendsBehindForeignContent(t *testing.T) {
	// R2: a CLAUDE.md that predates para keeps every byte, and para's block
	// goes at the end — the same append semantics as .gitattributes.
	foreign := "# Team notes\n\nRun `make check` before every commit.\n"
	in := render.In{
		Locator:  loc(t, "projects"),
		Kind:     kindmeta.KindContainer,
		Rules:    []string{"para-x.md"},
		Existing: map[string][]byte{"projects/CLAUDE.md": []byte(foreign)},
	}
	got, err := render.Claude.Render(in)
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	if !strings.HasPrefix(string(got), foreign) {
		t.Errorf("Claude =\n%s\nwant it to keep the foreign prose first\n%s", got, foreign)
	}
	if !strings.Contains(string(got), mdfile.BeginMarker+"\n@AGENTS.md\n@../.agents/rules/para-x.md\n"+mdfile.EndMarker+"\n") {
		t.Errorf("Claude =\n%s\nwant para's block appended after the foreign prose", got)
	}
}

func TestClaudeRefreshesItsBlockInPlaceInTheMiddle(t *testing.T) {
	// R3: block position is preserved. A para block sitting in the middle of a
	// file stays in the middle, with prefix and suffix untouched.
	prefix := "# Team notes\n\n"
	suffix := "\n## More notes\n\nDon't touch this.\n"
	stale := mdfile.BeginMarker + "\nwhatever an older para version said\n" + mdfile.EndMarker
	existing := prefix + stale + suffix

	in := render.In{
		Locator:  loc(t, "projects"),
		Kind:     kindmeta.KindContainer,
		Rules:    []string{"para-x.md"},
		Existing: map[string][]byte{"projects/CLAUDE.md": []byte(existing)},
	}
	got, err := render.Claude.Render(in)
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	fresh := mdfile.BeginMarker + "\n@AGENTS.md\n@../.agents/rules/para-x.md\n" + mdfile.EndMarker
	want := prefix + fresh + suffix
	if string(got) != want {
		t.Errorf("Claude =\n%s\nwant\n%s", got, want)
	}
}

func TestClaudeReportsADamagedBlockByPath(t *testing.T) {
	// R4: a begin marker with no matching end marker is a validation error
	// naming the path and the repair, not a silent overwrite — the same
	// behavior .gitattributes already has, at the path CLAUDE.md holds.
	damaged := mdfile.BeginMarker + "\nwhatever an older para version said\n"
	in := render.In{
		Locator:  loc(t, "projects"),
		Kind:     kindmeta.KindContainer,
		Rules:    []string{"para-x.md"},
		Existing: map[string][]byte{"projects/CLAUDE.md": []byte(damaged)},
	}
	_, err := render.Claude.Render(in)
	if err == nil {
		t.Fatal("Render on a damaged block returned no error")
	}
	if !strings.Contains(err.Error(), "projects/CLAUDE.md") {
		t.Errorf("error = %q, want it to name the path projects/CLAUDE.md", err)
	}
}

func TestClaudeLeavesAForeignMarkerDelimitedBlockUntouched(t *testing.T) {
	// A file can carry a marker-delimited block of its own that is not para's —
	// beads' `<!-- BEGIN BEADS INTEGRATION -->` shape is the real-world example.
	// Para's marker search must not mistake it for a stale block of its own;
	// the whole thing is foreign content to append behind.
	beadsBlock := "<!-- BEGIN BEADS INTEGRATION -->\nSee `bd prime` for workflow context.\n<!-- END BEADS INTEGRATION -->\n"
	in := render.In{
		Locator:  loc(t, "projects"),
		Kind:     kindmeta.KindContainer,
		Rules:    []string{"para-x.md"},
		Existing: map[string][]byte{"projects/CLAUDE.md": []byte(beadsBlock)},
	}
	got, err := render.Claude.Render(in)
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	if !strings.HasPrefix(string(got), beadsBlock) {
		t.Errorf("Claude =\n%s\nwant the beads block untouched and first\n%s", got, beadsBlock)
	}
	if !strings.Contains(string(got), mdfile.BeginMarker+"\n@AGENTS.md\n@../.agents/rules/para-x.md\n"+mdfile.EndMarker+"\n") {
		t.Errorf("Claude =\n%s\nwant para's own block appended after it", got)
	}
}
