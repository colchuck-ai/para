package mutate_test

import (
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/colchuck-ai/para/internal/clock"
	"github.com/colchuck-ai/para/internal/kindmeta"
	"github.com/colchuck-ai/para/internal/locator"
	"github.com/colchuck-ai/para/internal/mutate"
)

// pacific is the offset every test types its timestamps in, so that §15.1's
// "what you type is local; what is stored is UTC" has a visible conversion —
// never the host's zone (§0.2).
var pacific = time.FixedZone("PST", -8*3600)

// now is 2026-03-05 09:00 Pacific, which is 17:00Z: the same instant every
// clock in this package's tests reads.
func now() time.Time { return time.Date(2026, time.March, 5, 9, 0, 0, 0, pacific) }

func env(t *testing.T, root string) *mutate.Env {
	t.Helper()
	return mutate.NewEnv(root, clock.Fixed{At: now()})
}

// plantTree writes the smallest tree a mutation can run against: the root
// marker and the four buckets. Planting it rather than calling `para init`
// keeps these tests independent of a command this phase does not own.
func plantTree(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	files := map[string]string{
		".para/tree.toml":            "schema = 1\n",
		"README.md":                  "---\nkind: \"tree\"\n---\n",
		"ACTIVITY.md":                "# Activity\n",
		"projects/.para/state.toml":  "name = \"Projects\"\ncreated = \"2026-01-01T00:00:00Z\"\n",
		"projects/README.md":         "---\nkind: \"container\"\nlocator: \"projects\"\nname: \"Projects\"\n---\n",
		"projects/ACTIVITY.md":       "# Activity\n\n## 2026-01-01\n- Created.\n",
		"areas/.para/state.toml":     "name = \"Areas\"\ncreated = \"2026-01-01T00:00:00Z\"\n",
		"areas/ACTIVITY.md":          "# Activity\n\n## 2026-01-01\n- Created.\n",
		"resources/.para/state.toml": "name = \"Resources\"\ncreated = \"2026-01-01T00:00:00Z\"\n",
		"archive/.para/state.toml":   "name = \"Archive\"\ncreated = \"2026-01-01T00:00:00Z\"\n",
		".agents/skills/.keep":       "",
	}
	writeFiles(t, root, files)
	return root
}

func writeFiles(t *testing.T, root string, files map[string]string) {
	t.Helper()
	for rel, content := range files {
		path := filepath.Join(root, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
}

func loc(t *testing.T, s string) locator.Locator {
	t.Helper()
	l, err := locator.Parse(s)
	if err != nil {
		t.Fatalf("locator.Parse(%q): %v", s, err)
	}
	return l
}

func fields(kv ...string) mutate.Fields {
	var f mutate.Fields
	for i := 0; i+1 < len(kv); i += 2 {
		f.Set(kindmeta.Field(kv[i]), kv[i+1])
	}
	return f
}

// addProject is the fixture most tests start from: a project with its eager
// objectives/ container.
func addProject(t *testing.T, e *mutate.Env, id string) mutate.Result {
	t.Helper()
	res, err := e.Add(loc(t, "projects."+id), fields("name", "Acme migration", "description", "Rebuild the consumer."))
	if err != nil {
		t.Fatalf("Add: %v", err)
	}
	return res
}

func read(t *testing.T, root, rel string) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(rel)))
	if err != nil {
		t.Fatalf("reading %s: %v", rel, err)
	}
	return string(data)
}

// snapshot records every file in the tree by content, keyed by its
// slash-separated path relative to root.
func snapshot(t *testing.T, root string) map[string]string {
	t.Helper()
	out := map[string]string{}
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		out[filepath.ToSlash(rel)] = string(data)
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return out
}

func changedPaths(t *testing.T, before, after map[string]string) []string {
	t.Helper()
	var changed []string
	for path, content := range after {
		if prior, ok := before[path]; !ok || prior != content {
			changed = append(changed, path)
		}
	}
	for path := range before {
		if _, ok := after[path]; !ok {
			changed = append(changed, path)
		}
	}
	sort.Strings(changed)
	return changed
}

func assertEqual(t *testing.T, what string, got, want []string) {
	t.Helper()
	if strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Errorf("%s =\n  %s\nwant\n  %s", what, strings.Join(got, "\n  "), strings.Join(want, "\n  "))
	}
}

func errorContains(t *testing.T, err error, want string) {
	t.Helper()
	if err == nil {
		t.Fatalf("expected an error mentioning %q, got none", want)
	}
	if !strings.Contains(err.Error(), want) {
		t.Errorf("error = %q, want it to mention %q", err, want)
	}
}

// joinRoot joins a slash-separated, root-relative path onto an OS path.
func joinRoot(root, rel string) string {
	return filepath.Join(root, filepath.FromSlash(rel))
}
