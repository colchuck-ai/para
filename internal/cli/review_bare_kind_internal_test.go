package cli

import (
	"bytes"
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

// TestReviewAcceptsBareFixedArityKinds is para-nd3: review link/objective/
// key-result all failed before this fix with the same root cause (a
// fixed-arity noun's own address.Validate refuses a 0-length chain) — the
// generalized fix (mirroring list's own bare-noun-is-a-kind-filter
// lookahead) covers all three, not just link.
func TestReviewAcceptsBareFixedArityKinds(t *testing.T) {
	chdirToTestTree(t)
	mustAddViaCLI(t, "project", "acme")

	for _, noun := range []string{"link", "objective", "key-result"} {
		t.Run(noun, func(t *testing.T) {
			if out, err := execRead(newReviewCmd(), []string{noun}); err != nil {
				t.Fatalf("review %s: %v (%s)", noun, err, out)
			}
		})
	}
}

// TestReviewBareKindComposesWithScope is review's own version of `list
// link project acme`: a kind filter followed by a noun-and-chain scope.
func TestReviewBareKindComposesWithScope(t *testing.T) {
	chdirToTestTree(t)
	mustAddViaCLI(t, "project", "acme")

	if out, err := execRead(newReviewCmd(), []string{"link", "project", "acme"}); err != nil {
		t.Fatalf("review link project acme: %v (%s)", err, out)
	}
}

// TestReviewContainerFilterRefused mirrors list's own refusal (§25):
// containers are transparent and never rows, in review as in list.
func TestReviewContainerFilterRefused(t *testing.T) {
	chdirToTestTree(t)

	_, err := execRead(newReviewCmd(), []string{"container"})
	if err == nil {
		t.Fatal("review container: want a refusal, got none")
	}
	if !strings.Contains(err.Error(), "not a legal filter") {
		t.Errorf("review container error = %q, want the same refusal list gives", err.Error())
	}
}

// TestReviewStaleLinkEndToEnd drives a stale link all the way through the
// real add/config/review CLI commands.
func TestReviewStaleLinkEndToEnd(t *testing.T) {
	chdirToTestTree(t)
	mustAddViaCLI(t, "project", "acme")
	addLink(t, "project.acme.jira-epic", "--direction", "output", "--created", "2020-01-01")

	root := &cobra.Command{Use: "para"}
	root.AddCommand(newConfigCmd())
	var cfgOut bytes.Buffer
	root.SetOut(&cfgOut)
	root.SetErr(&cfgOut)
	root.SetArgs([]string{"config", "set", "link.stale-after", "7"})
	if err := root.Execute(); err != nil {
		t.Fatalf("config set link.stale-after: %v (%s)", err, cfgOut.String())
	}

	out, err := execRead(newReviewCmd(), []string{"--stale", "link"})
	if err != nil {
		t.Fatalf("review --stale link: %v (%s)", err, out)
	}
	if !strings.Contains(out, "jira-epic") {
		t.Errorf("review --stale link = %q, want the stale link listed", out)
	}
	if !strings.Contains(out, "link.stale-after") {
		t.Errorf("review --stale link = %q, want it to name the threshold", out)
	}
}
