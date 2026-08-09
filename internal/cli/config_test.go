package cli_test

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/colchuck-ai/para/internal/cli"
	"github.com/colchuck-ai/para/internal/clock"
)

var fixedClock = clock.Fixed{At: time.Date(2026, time.August, 5, 12, 0, 0, 0, time.UTC)}

// run invokes para against the tree at root, which it addresses through
// $PARA_HOME so the test never has to change the process's directory.
func run(t *testing.T, root string, args ...string) (int, string, string) {
	t.Helper()
	t.Setenv("PARA_HOME", root)
	var stdout, stderr bytes.Buffer
	code := cli.Run(fixedClock, args, &stdout, &stderr)
	return code, stdout.String(), stderr.String()
}

// §22's worked example, byte for byte. The chain is what §7 makes the
// condition of chained resolution being defensible, so its shape is part of
// the contract and not a rendering detail.
func TestConfigShowPrintsSection22sChain(t *testing.T) {
	root := plantTree(t, map[string]string{
		".para/config.toml":                        "project.stale-after = 14\n",
		"projects/.para/state.toml":                "name = \"Projects\"\n",
		"projects/.para/config.toml":               "project.stale-after = 30\n",
		"projects/acme-migration/.para/state.toml": "name = \"Acme migration\"\n",
	})

	code, stdout, stderr := run(t, root, "config", "show", "project.stale-after", "projects.acme-migration")

	if code != 0 {
		t.Fatalf("exit code = %d, want 0; stderr = %q", code, stderr)
	}
	want := "30\n" +
		"\n" +
		"  projects.acme-migration      —\n" +
		"→ projects                     30\n" +
		"  <root>                       14\n"
	if stdout != want {
		t.Errorf("stdout =\n%q\nwant\n%q", stdout, want)
	}
}

func TestConfigShowAtTheRootWhenNoLocatorIsGiven(t *testing.T) {
	root := plantTree(t, map[string]string{
		".para/config.toml": "project.stale-after = 14\n",
	})

	code, stdout, stderr := run(t, root, "config", "show", "project.stale-after")

	if code != 0 {
		t.Fatalf("exit code = %d, want 0; stderr = %q", code, stderr)
	}
	want := "14\n\n→ <root>      14\n"
	if stdout != want {
		t.Errorf("stdout =\n%q\nwant\n%q", stdout, want)
	}
}

// §7: "Unset everywhere means the check never fires." The chain still
// prints, because the answer to "why is nothing stale" is the chain.
func TestConfigShowOfAnUnsetKeyPrintsAnEmptyValueAndTheChain(t *testing.T) {
	root := plantTree(t, map[string]string{".para/config.toml": ""})

	code, stdout, stderr := run(t, root, "config", "show", "project.stale-after")

	if code != 0 {
		t.Fatalf("exit code = %d, want 0; stderr = %q", code, stderr)
	}
	want := "—\n\n  <root>      —\n"
	if stdout != want {
		t.Errorf("stdout =\n%q\nwant\n%q", stdout, want)
	}
}

// A built-in default is a level of the chain too, marked as the winner
// wherever no config.toml supplies one — otherwise the arrow would be
// missing and the value would come from nowhere.
func TestConfigShowMarksABuiltInDefault(t *testing.T) {
	root := plantTree(t, map[string]string{".para/config.toml": ""})

	code, stdout, stderr := run(t, root, "config", "show", "emit.claude-skills")

	if code != 0 {
		t.Fatalf("exit code = %d, want 0; stderr = %q", code, stderr)
	}
	want := "symlink\n" +
		"\n" +
		"  <root>         —\n" +
		"→ (default)      symlink\n"
	if stdout != want {
		t.Errorf("stdout =\n%q\nwant\n%q", stdout, want)
	}
}

func TestConfigShowJSONCarriesTheWholeChain(t *testing.T) {
	root := plantTree(t, map[string]string{
		".para/config.toml":                        "project.stale-after = 14\n",
		"projects/.para/state.toml":                "name = \"Projects\"\n",
		"projects/.para/config.toml":               "project.stale-after = 30\n",
		"projects/acme-migration/.para/state.toml": "name = \"Acme migration\"\n",
	})

	code, stdout, stderr := run(t, root, "config", "show", "--json", "project.stale-after", "projects.acme-migration")
	if code != 0 {
		t.Fatalf("exit code = %d, want 0; stderr = %q", code, stderr)
	}

	var got struct {
		Key     string  `json:"key"`
		Locator string  `json:"locator"`
		Value   float64 `json:"value"`
		Set     bool    `json:"set"`
		Default bool    `json:"default"`
		Source  *string `json:"source"`
		Chain   []struct {
			Level  string `json:"level"`
			File   string `json:"file"`
			Set    bool   `json:"set"`
			Value  any    `json:"value"`
			Winner bool   `json:"winner"`
		} `json:"chain"`
	}
	if err := json.Unmarshal([]byte(stdout), &got); err != nil {
		t.Fatalf("unmarshalling %q: %v", stdout, err)
	}
	if got.Key != "project.stale-after" || got.Value != 30 || !got.Set || got.Default {
		t.Errorf("got %+v, want project.stale-after = 30, set, not default", got)
	}
	if got.Source == nil || *got.Source != "projects" {
		t.Errorf("source = %v, want %q", got.Source, "projects")
	}
	if len(got.Chain) != 3 {
		t.Fatalf("chain has %d levels, want 3", len(got.Chain))
	}
	if got.Chain[0].Level != "projects.acme-migration" || got.Chain[0].Set {
		t.Errorf("chain[0] = %+v, want the unset entity level", got.Chain[0])
	}
	if got.Chain[1].Level != "projects" || !got.Chain[1].Winner {
		t.Errorf("chain[1] = %+v, want projects marked as the winner", got.Chain[1])
	}
	if got.Chain[2].Level != "" || got.Chain[2].File != ".para/config.toml" {
		t.Errorf("chain[2] = %+v, want the root level", got.Chain[2])
	}
}

func TestConfigSetWritesTheRootByDefaultAndSaysSo(t *testing.T) {
	root := plantTree(t, map[string]string{".para/config.toml": ""})

	code, stdout, stderr := run(t, root, "config", "set", "project.stale-after", "30")

	if code != 0 {
		t.Fatalf("exit code = %d, want 0; stderr = %q", code, stderr)
	}
	// The config file, the journal line §8.1 requires, and the ACTIVITY.md that
	// line lands in are one mutation (Phase 8).
	assertWrote(t, stdout,
		".para/logs/20260805T120000Z.jsonl",
		".para/config.toml",
		"ACTIVITY.md")
	assertFile(t, root, ".para/config.toml", "project.stale-after = 30\n")
	assertContains(t, root, "ACTIVITY.md", "Set **project.stale-after** to 30")
}

// §22: --at is how the chain gets built deliberately rather than by accident.
func TestConfigSetAtWritesTheNamedLevel(t *testing.T) {
	root := plantTree(t, map[string]string{
		".para/config.toml":         "",
		"projects/.para/state.toml": "name = \"Projects\"\n",
	})

	code, stdout, stderr := run(t, root, "config", "set", "--at", "projects", "project.stale-after", "30")

	if code != 0 {
		t.Fatalf("exit code = %d, want 0; stderr = %q", code, stderr)
	}
	// The event lands in the level's own journal, not the root's: an event is
	// written to the journal of the thing it happened to (§3.2, §8.1).
	assertWrote(t, stdout,
		"projects/.para/logs/20260805T120000Z.jsonl",
		"projects/.para/config.toml",
		"projects/ACTIVITY.md")
	assertFile(t, root, "projects/.para/config.toml", "project.stale-after = 30\n")
	assertFile(t, root, ".para/config.toml", "")
}

func TestConfigSetOnASkillWritesInsideTheSkill(t *testing.T) {
	root := plantTree(t, map[string]string{
		".para/config.toml": "",
		".agents/skills/para-signups-report/.para/state.toml": "name = \"Signups report\"\n",
	})

	code, stdout, stderr := run(t, root, "config", "set", "--at", "skills.signups-report", "review.cadence", "90")

	if code != 0 {
		t.Fatalf("exit code = %d, want 0; stderr = %q", code, stderr)
	}
	assertWrote(t, stdout,
		".agents/skills/para-signups-report/.para/logs/20260805T120000Z.jsonl",
		".agents/skills/para-signups-report/.para/config.toml",
		".agents/skills/para-signups-report/ACTIVITY.md")
	assertFile(t, root, ".agents/skills/para-signups-report/.para/config.toml", "review.cadence = 90\n")
}

// §23: a no-op set exits 0 and says `no change`. It must also write nothing,
// or every re-run of a provisioning script would churn the file.
func TestConfigSetToTheStoredValueIsANoOp(t *testing.T) {
	root := plantTree(t, map[string]string{".para/config.toml": "project.stale-after = 30\n"})
	path := filepath.Join(root, ".para", "config.toml")
	before := statOf(t, path)

	code, stdout, stderr := run(t, root, "config", "set", "project.stale-after", "30")

	if code != 0 {
		t.Fatalf("exit code = %d, want 0; stderr = %q", code, stderr)
	}
	if stdout != "no change\n" {
		t.Errorf("stdout = %q, want %q", stdout, "no change\n")
	}
	if after := statOf(t, path); after != before {
		t.Errorf("the file was rewritten: %v then %v", before, after)
	}
}

func TestConfigSetRefusesWhatItCannotStore(t *testing.T) {
	tests := []struct {
		name    string
		args    []string
		wantErr string
	}{
		{
			name:    "an unknown key, which would otherwise be a knob nothing reads",
			args:    []string{"config", "set", "project.stale-aftr", "30"},
			wantErr: "unknown config key",
		},
		{
			name:    "a value of the wrong type",
			args:    []string{"config", "set", "project.stale-after", "soon"},
			wantErr: "whole number",
		},
		{
			name:    "a value outside an enum",
			args:    []string{"config", "set", "emit.claude-skills", "hardlink"},
			wantErr: "symlink, copy",
		},
		{
			name:    "--at naming something that does not exist",
			args:    []string{"config", "set", "--at", "projects.nope", "project.stale-after", "30"},
			wantErr: "projects.nope",
		},
		{
			name:    "--at naming an illegal locator",
			args:    []string{"config", "set", "--at", "projects.a.b", "project.stale-after", "30"},
			wantErr: "projects.a.b",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			root := plantTree(t, map[string]string{".para/config.toml": ""})

			code, _, stderr := run(t, root, tt.args...)

			if code != 1 {
				t.Errorf("exit code = %d, want 1", code)
			}
			if !strings.Contains(stderr, tt.wantErr) {
				t.Errorf("stderr = %q, want it to mention %q", stderr, tt.wantErr)
			}
			assertFile(t, root, ".para/config.toml", "")
		})
	}
}

func TestConfigUnsetRemovesTheValueAtOneLevel(t *testing.T) {
	root := plantTree(t, map[string]string{
		".para/config.toml":          "project.stale-after = 14\n",
		"projects/.para/state.toml":  "name = \"Projects\"\n",
		"projects/.para/config.toml": "emit.claude = true\nproject.stale-after = 30\n",
	})

	code, stdout, stderr := run(t, root, "config", "unset", "--at", "projects", "project.stale-after")

	if code != 0 {
		t.Fatalf("exit code = %d, want 0; stderr = %q", code, stderr)
	}
	assertWrote(t, stdout,
		"projects/.para/logs/20260805T120000Z.jsonl",
		"projects/.para/config.toml",
		"projects/ACTIVITY.md")
	assertFile(t, root, "projects/.para/config.toml", "emit.claude = true\n")
	assertContains(t, root, "projects/ACTIVITY.md", "Unset **project.stale-after** (was 30)")
	// §7: unset removes a value and resolution continues up the chain.
	assertFile(t, root, ".para/config.toml", "project.stale-after = 14\n")
}

func TestConfigUnsetOfAnAbsentKeyIsANoOp(t *testing.T) {
	root := plantTree(t, map[string]string{".para/config.toml": "emit.claude = true\n"})
	path := filepath.Join(root, ".para", "config.toml")
	before := statOf(t, path)

	code, stdout, stderr := run(t, root, "config", "unset", "project.stale-after")

	if code != 0 {
		t.Fatalf("exit code = %d, want 0; stderr = %q", code, stderr)
	}
	if stdout != "no change\n" {
		t.Errorf("stdout = %q, want %q", stdout, "no change\n")
	}
	if after := statOf(t, path); after != before {
		t.Errorf("the file was rewritten: %v then %v", before, after)
	}
}

func TestConfigListPrintsEveryKnobAndWhereItCameFrom(t *testing.T) {
	root := plantTree(t, map[string]string{
		".para/config.toml":          "emit.claude = true\n",
		"projects/.para/state.toml":  "name = \"Projects\"\n",
		"projects/.para/config.toml": "project.stale-after = 30\n",
	})

	code, stdout, stderr := run(t, root, "config", "list", "projects")

	if code != 0 {
		t.Fatalf("exit code = %d, want 0; stderr = %q", code, stderr)
	}
	// Every key para recognises, resolved at that level: the set ones name
	// the level they came from, the defaulted ones say so, and the unset
	// ones are visible rather than absent — a knob nobody can find is a
	// knob that will be wrong (§20).
	want := map[string][]string{
		"emit.claude":             {"true", "<root>"},
		"emit.claude-skills":      {"symlink", "(default)"},
		"emit.gitattributes":      {"true", "(default)"},
		"project.stale-after":     {"30", "projects"},
		"review.cadence":          {"—", "—"},
		"key-result.at-risk-pace": {"—", "—"},
		"log.rotate-bytes":        {"4194304", "(default)"},
	}
	got := listRows(stdout)
	if len(got) != 11 {
		t.Errorf("listed %d keys, want all 11 of §7's table:\n%s", len(got), stdout)
	}
	for key, cols := range want {
		if !slices.Equal(got[key], cols) {
			t.Errorf("%s = %v, want %v; full output:\n%s", key, got[key], cols, stdout)
		}
	}
}

// listRows splits `config list` output into key → [value, source]. The
// columns are aligned with runs of spaces, and no cell contains one.
func listRows(stdout string) map[string][]string {
	rows := make(map[string][]string)
	for _, line := range strings.Split(strings.TrimRight(stdout, "\n"), "\n") {
		fields := strings.Fields(line)
		if len(fields) == 3 {
			rows[fields[0]] = fields[1:]
		}
	}
	return rows
}

func TestConfigListPrefixFilters(t *testing.T) {
	root := plantTree(t, map[string]string{".para/config.toml": "emit.claude = true\n"})

	code, stdout, stderr := run(t, root, "config", "list", "--prefix", "emit")

	if code != 0 {
		t.Fatalf("exit code = %d, want 0; stderr = %q", code, stderr)
	}
	if strings.Contains(stdout, "stale-after") || strings.Contains(stdout, "log.rotate-bytes") {
		t.Errorf("--prefix emit printed keys outside the prefix:\n%s", stdout)
	}
	for _, want := range []string{"emit.claude", "emit.claude-skills", "emit.gitattributes"} {
		if !strings.Contains(stdout, want) {
			t.Errorf("stdout does not contain %q; got:\n%s", want, stdout)
		}
	}
}

func TestConfigListJSON(t *testing.T) {
	root := plantTree(t, map[string]string{".para/config.toml": "emit.claude = true\n"})

	code, stdout, stderr := run(t, root, "config", "list", "--prefix", "emit.claude", "--json")
	if code != 0 {
		t.Fatalf("exit code = %d, want 0; stderr = %q", code, stderr)
	}

	var got struct {
		Locator string `json:"locator"`
		Keys    []struct {
			Key     string  `json:"key"`
			Set     bool    `json:"set"`
			Default bool    `json:"default"`
			Source  *string `json:"source"`
		} `json:"keys"`
	}
	if err := json.Unmarshal([]byte(stdout), &got); err != nil {
		t.Fatalf("unmarshalling %q: %v", stdout, err)
	}
	if len(got.Keys) != 2 {
		t.Fatalf("keys = %d, want 2 (emit.claude and emit.claude-skills)", len(got.Keys))
	}
	if got.Keys[0].Key != "emit.claude" || !got.Keys[0].Set || got.Keys[0].Default {
		t.Errorf("keys[0] = %+v, want emit.claude set from a level", got.Keys[0])
	}
	if got.Keys[1].Key != "emit.claude-skills" || got.Keys[1].Set || !got.Keys[1].Default {
		t.Errorf("keys[1] = %+v, want emit.claude-skills defaulted", got.Keys[1])
	}
}

func TestConfigShowRefusesAnUnknownKey(t *testing.T) {
	root := plantTree(t, map[string]string{".para/config.toml": ""})

	code, _, stderr := run(t, root, "config", "show", "project.stale-aftr")

	if code != 1 {
		t.Errorf("exit code = %d, want 1", code)
	}
	if !strings.Contains(stderr, "unknown config key") {
		t.Errorf("stderr = %q", stderr)
	}
}

// Config is a property of a tree, so every config command discovers the root
// before it does anything else.
func TestConfigNeedsATree(t *testing.T) {
	code, _, stderr := run(t, t.TempDir(), "config", "list")

	if code != 1 {
		t.Errorf("exit code = %d, want 1", code)
	}
	if !strings.Contains(stderr, "tree.toml") {
		t.Errorf("stderr = %q, want it to name the missing root marker", stderr)
	}
}

func plantTree(t *testing.T, files map[string]string) string {
	t.Helper()
	root := t.TempDir()
	// t.TempDir on macOS hands back a path under /var, which is a symlink
	// to /private/var; resolving it here keeps a written path comparable to
	// the root the command discovered.
	resolved, err := filepath.EvalSymlinks(root)
	if err != nil {
		t.Fatal(err)
	}
	root = resolved
	if err := os.MkdirAll(filepath.Join(root, ".para"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, ".para", "tree.toml"), []byte("schema = 1\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	for rel, content := range files {
		path := filepath.Join(root, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return root
}

func assertFile(t *testing.T, root, rel, want string) {
	t.Helper()
	got, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(rel)))
	if err != nil {
		t.Fatalf("reading %s: %v", rel, err)
	}
	if string(got) != want {
		t.Errorf("%s =\n%q\nwant\n%q", rel, got, want)
	}
}

// statOf is the evidence a no-op wrote nothing: size and modification time
// together change on any rewrite, including one that produced the same bytes.
func statOf(t *testing.T, path string) [2]int64 {
	t.Helper()
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat %s: %v", path, err)
	}
	return [2]int64{info.Size(), info.ModTime().UnixNano()}
}

// assertWrote checks the §23 file list a mutation prints: a labelled first
// line, the rest aligned under it, in write order.
func assertWrote(t *testing.T, stdout string, paths ...string) {
	t.Helper()
	var b strings.Builder
	for i, path := range paths {
		if i == 0 {
			b.WriteString("wrote  ")
		} else {
			b.WriteString("       ")
		}
		b.WriteString(path)
		b.WriteString("\n")
	}
	if stdout != b.String() {
		t.Errorf("stdout =\n%q\nwant\n%q", stdout, b.String())
	}
}

func assertContains(t *testing.T, root, rel, want string) {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(rel)))
	if err != nil {
		t.Fatalf("reading %s: %v", rel, err)
	}
	if !strings.Contains(string(data), want) {
		t.Errorf("%s =\n%s\nwant it to contain %q", rel, data, want)
	}
}
