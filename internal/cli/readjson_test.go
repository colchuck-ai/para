package cli_test

import (
	"encoding/json"
	"testing"
)

// TestShowJSONLocatorIsTheDottedAddress is R24/R26's rule for `show --json`:
// the `locator` key carries the dotted address form, the key itself does
// not change, and the `kind` key already agrees with it by construction —
// so no new `noun` key is added.
func TestShowJSONLocatorIsTheDottedAddress(t *testing.T) {
	root := plantTree(t, map[string]string{
		".para/state.toml":                                    "",
		"projects/.para/state.toml":                           "name = \"Projects\"\n",
		"projects/acme-migration/.para/state.toml":            "name = \"Acme migration\"\n",
		".agents/skills/para-signups-report/.para/state.toml": "name = \"Signups report\"\nscope = [\"project.acme-migration\"]\n",
	})

	code, stdout, stderr := run(t, root, "show", "project", "acme-migration", "--json")
	if code != 0 {
		t.Fatalf("exit code = %d, want 0; stderr = %q", code, stderr)
	}

	var got struct {
		Locator string `json:"locator"`
		Kind    string `json:"kind"`
		Skills  []struct {
			Locator string `json:"locator"`
			Via     string `json:"via"`
		} `json:"skills"`
	}
	if err := json.Unmarshal([]byte(stdout), &got); err != nil {
		t.Fatalf("unmarshalling %q: %v", stdout, err)
	}
	if got.Locator != "project.acme-migration" {
		t.Errorf("locator = %q, want %q", got.Locator, "project.acme-migration")
	}
	if got.Kind != "project" {
		t.Errorf("kind = %q, want %q, and it should agree with locator's first segment", got.Kind, "project")
	}
	if len(got.Skills) != 1 || got.Skills[0].Locator != "skill.signups-report" {
		t.Errorf("skills = %+v, want one naming skill.signups-report", got.Skills)
	}
	if got.Skills[0].Via != "project.acme-migration" {
		t.Errorf("skills[0].via = %q, want the dotted scope entry that reached it", got.Skills[0].Via)
	}
}

// TestListJSONLocatorIsTheDottedAddress is the same rule for `list --json`,
// whose entities share entityJSON with `show`.
func TestListJSONLocatorIsTheDottedAddress(t *testing.T) {
	root := plantTree(t, map[string]string{
		".para/state.toml":                       "",
		"areas/.para/state.toml":                 "name = \"Areas\"\n",
		"areas/health/.para/state.toml":          "name = \"Health\"\n",
		"areas/health/training/.para/state.toml": "name = \"Training\"\n",
	})

	code, stdout, stderr := run(t, root, "list", "--json")
	if code != 0 {
		t.Fatalf("exit code = %d, want 0; stderr = %q", code, stderr)
	}

	var got struct {
		Entities []struct {
			Locator string `json:"locator"`
			Kind    string `json:"kind"`
		} `json:"entities"`
	}
	if err := json.Unmarshal([]byte(stdout), &got); err != nil {
		t.Fatalf("unmarshalling %q: %v", stdout, err)
	}
	want := map[string]string{
		"area.health":          "area",
		"area.health.training": "area",
	}
	if len(got.Entities) != len(want) {
		t.Fatalf("entities = %+v, want %d rows", got.Entities, len(want))
	}
	for _, e := range got.Entities {
		if wantKind, ok := want[e.Locator]; !ok || e.Kind != wantKind {
			t.Errorf("entity %+v not among the expected dotted addresses %v", e, want)
		}
	}
}
