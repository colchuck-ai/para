package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/colchuck-ai/para/internal/locator"
)

// mustAddrLoc parses s with locator.Parse for building a "want" value in a
// table below — the same helper repair_internal_test.go already carries
// under a different name, kept local so this file has no cross-file
// dependency on it.
func mustAddrLoc(t *testing.T, s string) locator.Locator {
	t.Helper()
	l, err := locator.Parse(s)
	if err != nil {
		t.Fatalf("locator.Parse(%q): %v", s, err)
	}
	return l
}

// writeAddrFixture builds the minimal tree parseAddressArgs' "." handling
// needs to resolve against — nothing else in this file touches a
// filesystem.
func writeAddrFixture(t *testing.T) (root string) {
	t.Helper()
	root = t.TempDir()
	files := map[string]string{
		".para/tree.toml":                "schema = 1\n",
		"projects/.para/state.toml":      "name = \"Projects\"\n",
		"projects/acme/.para/state.toml": "name = \"Acme\"\n",
	}
	for rel, content := range files {
		full := filepath.Join(root, rel)
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return root
}

// TestChainToLocator is R3's table read off the noun-plus-chain pair
// address.Parse already validates, exercised through the CLI's own entry
// point rather than address's, plus the two things this layer adds: --archived
// (R7) and R17's bucket refusal.
func TestChainToLocator(t *testing.T) {
	cases := []struct {
		name     string
		noun     string
		chain    string
		archived bool
		bucketOK bool
		want     locator.Locator
		wantErr  string
	}{
		{name: "project entity", noun: "project", chain: "acme", bucketOK: true, want: mustAddrLoc(t, "projects.acme")},
		{name: "objective, structural words inserted", noun: "objective", chain: "acme.q1-growth", bucketOK: false, want: mustAddrLoc(t, "projects.acme.objectives.q1-growth")},
		{name: "key-result, both structural words inserted", noun: "key-result", chain: "acme.q1-growth.signups", bucketOK: false, want: mustAddrLoc(t, "projects.acme.objectives.q1-growth.key-results.signups")},
		{name: "skill maps to the skills bucket", noun: "skill", chain: "signups-report", bucketOK: false, want: mustAddrLoc(t, "skills.signups-report")},
		{name: "area nests to arbitrary depth", noun: "area", chain: "health.training", bucketOK: true, want: mustAddrLoc(t, "areas.health.training")},
		{name: "archived prepends the place", noun: "project", chain: "acme", archived: true, bucketOK: true, want: mustAddrLoc(t, "archive.projects.acme")},
		{name: "bucket allowed with no chain", noun: "project", chain: "", bucketOK: true, want: mustAddrLoc(t, "projects")},
		{name: "bucket refused names the noun and the shape", noun: "project", chain: "", bucketOK: false, wantErr: "project names a bucket, not an entity"},
		{name: "objective has no bucket form at all, arity wins over bucketOK", noun: "objective", chain: "", bucketOK: true, wantErr: "objective"},
		{name: "wrong arity names the noun and expected shape", noun: "key-result", chain: "acme.q1-growth", bucketOK: false, wantErr: "key-result takes exactly three segments"},
		{name: "reserved word in an id position", noun: "project", chain: "projects", bucketOK: false, wantErr: "reserved word"},
		{name: "not a noun at all", noun: "sprocket", chain: "x", bucketOK: false, wantErr: "not a noun"},
		{name: "container, two segments ending in objectives", noun: "container", chain: "acme.objectives", bucketOK: false, want: mustAddrLoc(t, "projects.acme.objectives")},
		{name: "container, three segments ending in key-results", noun: "container", chain: "acme.q1-growth.key-results", bucketOK: false, want: mustAddrLoc(t, "projects.acme.objectives.q1-growth.key-results")},
		{name: "container, wrong terminal word", noun: "container", chain: "acme.milestones", bucketOK: false, wantErr: "container"},
		{name: "container, archived", noun: "container", chain: "acme.objectives", archived: true, bucketOK: false, want: mustAddrLoc(t, "archive.projects.acme.objectives")},
		{name: "skill cannot be archived, even as a bucket", noun: "skill", chain: "", archived: true, bucketOK: true, wantErr: "skill cannot be archived"},
		{name: "skill cannot be archived, as an entity", noun: "skill", chain: "signups-report", archived: true, bucketOK: false, wantErr: "skill cannot be archived"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, err := chainToLocator(c.noun, c.chain, c.archived, c.bucketOK)
			if c.wantErr != "" {
				if err == nil {
					t.Fatalf("chainToLocator(%q, %q) = %v, want error containing %q", c.noun, c.chain, got, c.wantErr)
				}
				if !strings.Contains(err.Error(), c.wantErr) {
					t.Errorf("chainToLocator(%q, %q) error = %q, want it to contain %q", c.noun, c.chain, err.Error(), c.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("chainToLocator(%q, %q): %v", c.noun, c.chain, err)
			}
			if got.String() != c.want.String() {
				t.Errorf("chainToLocator(%q, %q) = %q, want %q", c.noun, c.chain, got.String(), c.want.String())
			}
		})
	}
}

// TestParseAddressArgsRootAndBucketArity is R17's and R18's two axes,
// exercised through the four arity shapes R12's table reduces to: a bare
// noun is the bucket where bucketOK allows it and a refusal where it does
// not, and no noun at all is the tree root where rootOK allows it and a
// refusal where it does not.
func TestParseAddressArgsRootAndBucketArity(t *testing.T) {
	root := writeAddrFixture(t)
	cwd := root

	cases := []struct {
		name    string
		args    []string
		arity   addressArity
		want    string // locator.String(), "" for the root
		wantErr string
		restLen int
	}{
		{name: "scopeArity, no args, is the root", args: nil, arity: scopeArity, want: ""},
		{name: "scopeArity, bare noun, is the bucket", args: []string{"project"}, arity: scopeArity, want: "projects"},
		{name: "scopeArity, noun and chain, is a scope", args: []string{"project", "acme"}, arity: scopeArity, want: "projects.acme"},
		{name: "bucketArity, no args, refused: a noun is required", args: nil, arity: bucketArity, wantErr: "a noun is required"},
		{name: "bucketArity, bare noun, is the bucket", args: []string{"area"}, arity: bucketArity, want: "areas"},
		{name: "entityArity, bare noun, refused as a bucket", args: []string{"project"}, arity: entityArity, wantErr: "project names a bucket, not an entity"},
		{name: "entityArity, noun and chain, is an entity", args: []string{"project", "acme"}, arity: entityArity, want: "projects.acme"},
		{name: "note's trailing text is returned unconsumed", args: []string{"project", "acme", "text goes here"}, arity: entityArity, want: "projects.acme", restLen: 1},
		{name: "unset's trailing fields are returned unconsumed", args: []string{"area", "health", "tags", "status"}, arity: entityArity, want: "areas.health", restLen: 2},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			loc, rest, err := parseAddressArgs(root, cwd, c.args, c.arity, false)
			if c.wantErr != "" {
				if err == nil {
					t.Fatalf("parseAddressArgs(%v) = %v, want error containing %q", c.args, loc, c.wantErr)
				}
				if !strings.Contains(err.Error(), c.wantErr) {
					t.Errorf("parseAddressArgs(%v) error = %q, want it to contain %q", c.args, err.Error(), c.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("parseAddressArgs(%v): %v", c.args, err)
			}
			if got := loc.String(); got != c.want {
				t.Errorf("parseAddressArgs(%v) locator = %q, want %q", c.args, got, c.want)
			}
			if len(rest) != c.restLen {
				t.Errorf("parseAddressArgs(%v) rest = %v, want length %d", c.args, rest, c.restLen)
			}
		})
	}
}

// TestParseAddressArgsDot is R16's "." convenience: it occupies the whole
// address as one argument, resolved against cwd by walking up to the
// nearest .para/state.toml, and is refused outright — rather than parsed as
// a noun named "." — wherever dotOK is false.
func TestParseAddressArgsDot(t *testing.T) {
	root := writeAddrFixture(t)
	cwd := filepath.Join(root, "projects", "acme")

	loc, rest, err := parseAddressArgs(root, cwd, []string{"."}, entityArity, false)
	if err != nil {
		t.Fatalf("parseAddressArgs(.): %v", err)
	}
	if got, want := loc.String(), "projects.acme"; got != want {
		t.Errorf("parseAddressArgs(.) = %q, want %q", got, want)
	}
	if len(rest) != 0 {
		t.Errorf("rest = %v, want none", rest)
	}

	// "." followed by more arguments (note's text, unset's fields) leaves
	// them unconsumed, exactly as a noun-and-chain pair would.
	_, rest, err = parseAddressArgs(root, cwd, []string{".", "a note"}, entityArity, false)
	if err != nil {
		t.Fatalf("parseAddressArgs(., text): %v", err)
	}
	if len(rest) != 1 || rest[0] != "a note" {
		t.Errorf("rest = %v, want [%q]", rest, "a note")
	}

	_, noDotRest, err := parseAddressArgs(root, cwd, []string{".", "extra"}, entityArityNoDot, false)
	if err == nil {
		t.Fatal("parseAddressArgs(.) with dotOK false: want a refusal, got none")
	}
	if !strings.Contains(err.Error(), `"." is not accepted here`) {
		t.Errorf("parseAddressArgs(.) with dotOK false, error = %q, want it to contain %q", err.Error(), `"." is not accepted here`)
	}
	if len(noDotRest) != 1 || noDotRest[0] != "extra" {
		t.Errorf("rest = %v, want [%q]", noDotRest, "extra")
	}
}

// TestParseAddressArgsArchived confirms --archived (R7) prepends the place
// to whatever chainToLocator produces, threaded all the way through the
// args-reading layer.
func TestParseAddressArgsArchived(t *testing.T) {
	root := writeAddrFixture(t)
	loc, _, err := parseAddressArgs(root, root, []string{"project", "acme"}, entityArity, true)
	if err != nil {
		t.Fatalf("parseAddressArgs: %v", err)
	}
	if got, want := loc.String(), "archive.projects.acme"; got != want {
		t.Errorf("locator = %q, want %q", got, want)
	}
}
