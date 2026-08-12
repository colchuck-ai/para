package cli_test

import (
	"encoding/json"
	"maps"
	"slices"
	"strings"
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

// TestListAndShowJSONHaveNoNounKey is P20.5's own check, made positive
// rather than assumed: R26 says no `noun` key is added because `kind`
// already carries it, and this asserts that directly against the raw JSON
// text rather than only against a Go struct that would simply ignore an
// extra field if one were accidentally added.
func TestListAndShowJSONHaveNoNounKey(t *testing.T) {
	root := plantTree(t, map[string]string{
		".para/state.toml":               "",
		"projects/.para/state.toml":      "name = \"Projects\"\n",
		"projects/acme/.para/state.toml": "name = \"Acme\"\n",
	})

	for _, args := range [][]string{
		{"show", "project", "acme", "--json"},
		{"list", "--json"},
	} {
		t.Run(strings.Join(args, " "), func(t *testing.T) {
			code, stdout, stderr := run(t, root, args...)
			if code != 0 {
				t.Fatalf("%v: exit %d, stderr = %q", args, code, stderr)
			}
			if strings.Contains(stdout, `"noun"`) {
				t.Errorf("%v output = %s, want no \"noun\" key — kind already carries it (R26)", args, stdout)
			}
		})
	}
}

// TestListJSONShapeUnaffectedByKindFilter is P20.5's other half: R19's new
// kind filter narrows which entities `list --json` reports, but must not
// change the shape of the response around them — the same five top-level
// keys (§23), with `total`/`shown` reflecting what the filter matched. The
// key set is checked exactly, against the raw object rather than a Go
// struct, since a struct would silently ignore an unrelated new key the way
// `TestListAndShowJSONHaveNoNounKey` deliberately avoids for `noun`.
func TestListJSONShapeUnaffectedByKindFilter(t *testing.T) {
	root := plantTree(t, map[string]string{
		".para/state.toml":               "",
		"projects/.para/state.toml":      "name = \"Projects\"\n",
		"projects/acme/.para/state.toml": "name = \"Acme\"\n",
		"areas/.para/state.toml":         "name = \"Areas\"\n",
		"areas/health/.para/state.toml":  "name = \"Health\"\n",
	})

	code, stdout, stderr := run(t, root, "list", "project", "--json")
	if code != 0 {
		t.Fatalf("list project --json: exit %d, stderr = %q", code, stderr)
	}

	var raw map[string]json.RawMessage
	if err := json.Unmarshal([]byte(stdout), &raw); err != nil {
		t.Fatalf("unmarshalling %q: %v", stdout, err)
	}
	wantKeys := []string{"total", "shown", "hidden", "hidden-statuses", "entities"}
	if len(raw) != len(wantKeys) {
		t.Errorf("keys = %v, want exactly %v", slices.Collect(maps.Keys(raw)), wantKeys)
	}
	for _, k := range wantKeys {
		if _, ok := raw[k]; !ok {
			t.Errorf("keys = %v, missing %q", slices.Collect(maps.Keys(raw)), k)
		}
	}

	var got struct {
		Total    int `json:"total"`
		Shown    int `json:"shown"`
		Entities []struct {
			Locator string `json:"locator"`
			Kind    string `json:"kind"`
		} `json:"entities"`
	}
	if err := json.Unmarshal([]byte(stdout), &got); err != nil {
		t.Fatalf("unmarshalling %q: %v", stdout, err)
	}
	if got.Total != 1 || got.Shown != 1 {
		t.Errorf("total/shown = %d/%d, want 1/1 — the area must not appear", got.Total, got.Shown)
	}
	if len(got.Entities) != 1 || got.Entities[0].Locator != "project.acme" {
		t.Errorf("entities = %+v, want exactly project.acme", got.Entities)
	}
}
