package render_test

import (
	"strings"
	"testing"

	"github.com/colchuck-ai/para/internal/kindmeta"
	"github.com/colchuck-ai/para/internal/render"
	"github.com/colchuck-ai/para/internal/truth"
)

func TestCursorRuleAlwaysAppliesWithNoScope(t *testing.T) {
	in := render.In{
		Locator: loc(t, "skills.signups-report"),
		Kind:    kindmeta.KindSkill,
		State:   truth.State{Name: "Signups report", Description: "when asked for the number"},
	}
	got, err := render.CursorRule.Render(in)
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	if !strings.Contains(string(got), "alwaysApply: true\n") {
		t.Errorf("CursorRule =\n%s\nwant alwaysApply: true", got)
	}
	if strings.Contains(string(got), "globs:") {
		t.Errorf("CursorRule =\n%s\nwant no globs line", got)
	}
	if !strings.Contains(string(got), "generated_from: \"para-signups-report\"\n") {
		t.Errorf("CursorRule =\n%s\nwant generated_from", got)
	}
	if !strings.Contains(string(got), "`.cursor/skills/para-signups-report/SKILL.md`") {
		t.Errorf("CursorRule =\n%s\nwant it to point at the mirror", got)
	}
}

func TestCursorRuleGlobsOnScopedSkill(t *testing.T) {
	in := render.In{
		Locator: loc(t, "skills.wide"),
		Kind:    kindmeta.KindSkill,
		State: truth.State{
			Name:  "Wide",
			Scope: []string{"project", "area.health", "resource.notes"},
		},
	}
	got, err := render.CursorRule.Render(in)
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	if !strings.Contains(string(got), `globs: "projects/**,areas/health/**,resources/notes/**"`) {
		t.Errorf("CursorRule =\n%s\nwant the joined globs", got)
	}
	if strings.Contains(string(got), "alwaysApply") {
		t.Errorf("CursorRule =\n%s\nwant no alwaysApply line", got)
	}
}

func TestCursorRuleKeepsScopeEntryOrderAsStored(t *testing.T) {
	in := render.In{
		Locator: loc(t, "skills.wide"),
		Kind:    kindmeta.KindSkill,
		State:   truth.State{Name: "Wide", Scope: []string{"resource.notes", "area.health"}},
	}
	got, err := render.CursorRule.Render(in)
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	if !strings.Contains(string(got), `globs: "resources/notes/**,areas/health/**"`) {
		t.Errorf("CursorRule =\n%s\nwant scope in stored order", got)
	}
}

func TestCursorRuleRendersAnUnresolvableScopeEntryAsWritten(t *testing.T) {
	in := render.In{
		Locator: loc(t, "skills.wide"),
		Kind:    kindmeta.KindSkill,
		State:   truth.State{Name: "Wide", Scope: []string{"Not A Locator"}},
	}
	got, err := render.CursorRule.Render(in)
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	if !strings.Contains(string(got), `globs: "Not A Locator"`) {
		t.Errorf("CursorRule =\n%s\nwant the entry rendered as written", got)
	}
}

func TestCursorRuleRefusesANonSkill(t *testing.T) {
	in := render.In{Locator: loc(t, "areas.health"), Kind: kindmeta.KindArea, State: truth.State{Name: "Health"}}
	if _, err := render.CursorRule.Render(in); err == nil {
		t.Error("CursorRule.Render on an area returned no error")
	}
	if _, err := render.CursorRule.Path(in); err == nil {
		t.Error("CursorRule.Path on an area returned no error")
	}
}

func TestCursorRuleFilenames(t *testing.T) {
	got := render.CursorRuleFilenames([]string{"b", "a", "a"})
	want := []string{"para-a.mdc", "para-b.mdc"}
	if len(got) != len(want) {
		t.Fatalf("CursorRuleFilenames = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("CursorRuleFilenames = %v, want %v", got, want)
		}
	}
}

func TestForIncludesCursorRuleOnlyWhenEmitCursorIsOn(t *testing.T) {
	base := render.In{
		Locator: loc(t, "skills.signups-report"),
		Kind:    kindmeta.KindSkill,
		State:   truth.State{Name: "Signups report"},
	}

	off, err := render.Artifacts(base)
	if err != nil {
		t.Fatalf("Artifacts: %v", err)
	}
	for _, a := range off {
		if strings.HasPrefix(a.Path, render.CursorRulesDir) {
			t.Errorf("Artifacts with emit.cursor off produced %q", a.Path)
		}
	}

	on := base
	on.Config = render.Config{EmitCursor: true}
	got, err := render.Artifacts(on)
	if err != nil {
		t.Fatalf("Artifacts: %v", err)
	}
	want := ".cursor/rules/para-signups-report.mdc"
	found := false
	for _, a := range got {
		if a.Path == want {
			found = true
		}
	}
	if !found {
		t.Errorf("Artifacts with emit.cursor on = %v, want it to include %q", got, want)
	}
}
