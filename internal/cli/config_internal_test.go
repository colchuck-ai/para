package cli

import (
	"path/filepath"
	"strings"
	"testing"
)

// TestResolveConfigAt is R24's rule for config's own address argument
// (--at, and config show's second positional argument, both of which
// funnel through this one function): it takes the one-token dotted form
// rather than the old plural-bucket locator string, since a flag value is
// one token — and "." still works, resolved against cwd exactly as every
// other address argument's does.
func TestResolveConfigAt(t *testing.T) {
	root := writeAddrFixture(t)

	cases := []struct {
		name    string
		at      string
		cwd     string
		want    string
		wantErr string
	}{
		{name: "dotted noun and chain", at: "project.acme", cwd: root, want: "projects.acme"},
		{name: "bare noun is the bucket", at: "project", cwd: root, want: "projects"},
		{name: "dot resolves against cwd", at: ".", cwd: filepath.Join(root, "projects", "acme"), want: "projects.acme"},
		{name: "illegal chain names the noun and the arity", at: "project.a.b", cwd: root, wantErr: "not a chain of 2"},
		{name: "not a noun at all", at: "bogus.acme", cwd: root, wantErr: "not a noun"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			loc, err := resolveConfigAt(root, c.cwd, c.at)
			if c.wantErr != "" {
				if err == nil {
					t.Fatalf("resolveConfigAt(%q) = %v, want error containing %q", c.at, loc, c.wantErr)
				}
				if !strings.Contains(err.Error(), c.wantErr) {
					t.Errorf("resolveConfigAt(%q) error = %q, want it to contain %q", c.at, err.Error(), c.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("resolveConfigAt(%q): %v", c.at, err)
			}
			if got := loc.String(); got != c.want {
				t.Errorf("resolveConfigAt(%q) = %q, want %q", c.at, got, c.want)
			}
		})
	}
}
