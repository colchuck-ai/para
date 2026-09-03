package cli

import (
	"bytes"
	"slices"
	"strings"
	"testing"

	"github.com/spf13/cobra"

	"github.com/colchuck-ai/para/internal/kindmeta"
)

// addLink is add link's own helper (para-6g7): mustAddViaCLI's --name/
// --description shape does not fit link's own required flags.
func addLink(t *testing.T, chain string, extra ...string) {
	t.Helper()
	root := &cobra.Command{Use: "para"}
	root.AddCommand(newAddCmd())
	var out bytes.Buffer
	root.SetOut(&out)
	root.SetErr(&out)
	args := append([]string{
		"add", "link", chain,
		"--type", "jira-epic", "--ref", "PROJ-123", "--direction", "output",
	}, extra...)
	root.SetArgs(args)
	if err := root.Execute(); err != nil {
		t.Fatalf("add link %s: %v (%s)", chain, err, out.String())
	}
}

// TestAddLinkUsageDoesNotClaimNameIsRequired is the spec-review catch: link
// is the one kind where --name is Optional (defaulted to the id), so add's
// Use string must not show it as a bare required flag the way every other
// kind's genuinely is.
func TestAddLinkUsageDoesNotClaimNameIsRequired(t *testing.T) {
	if got := newAddNounCmd(kindmeta.KindLink).Use; strings.Contains(got, "--name … ") {
		t.Errorf("add link Use = %q, want --name shown as optional", got)
	}
	if got := newAddNounCmd(kindmeta.KindProject).Use; !strings.Contains(got, "--name … ") {
		t.Errorf("add project Use = %q, want --name shown as required, unlike link", got)
	}
}

// TestAddLinkUnderEachParentKind exercises the confirmed chain shape
// (parent noun first) across all three link-capable parents.
func TestAddLinkUnderEachParentKind(t *testing.T) {
	chdirToTestTree(t)
	mustAddViaCLI(t, "project", "acme")
	mustAddViaCLI(t, "area", "health")
	mustAddViaCLI(t, "resource", "papers")

	addLink(t, "project.acme.jira-epic")
	addLink(t, "area.health.blog", "--type", "blog", "--ref", "https://example.com")
	addLink(t, "resource.papers.feed", "--type", "rss-feed", "--ref", "https://example.com/feed")

	for _, chain := range []string{"project.acme.jira-epic", "area.health.blog", "resource.papers.feed"} {
		if out, err := execRead(newShowCmd(), []string{"link", chain}); err != nil {
			t.Errorf("show link %s: %v (%s)", chain, err, out)
		}
	}
}

// TestShowParentGroupsLinksByDirection is the bead's own acceptance
// criterion: `show <parent>` renders its links grouped by direction,
// alongside what it already shows.
func TestShowParentGroupsLinksByDirection(t *testing.T) {
	chdirToTestTree(t)
	mustAddViaCLI(t, "project", "acme")
	addLink(t, "project.acme.jira-epic", "--direction", "output")
	addLink(t, "project.acme.status-page", "--type", "rss-feed", "--ref", "https://example.com/status", "--direction", "input")

	out, err := execRead(newShowCmd(), []string{"project", "acme"})
	if err != nil {
		t.Fatalf("show project acme: %v (%s)", err, out)
	}
	if !strings.Contains(out, "links") {
		t.Fatalf("show output = %q, want a links section", out)
	}
	if !strings.Contains(out, "output") || !strings.Contains(out, "input") {
		t.Errorf("show output = %q, want both direction groups", out)
	}
	if !strings.Contains(out, "jira-epic") || !strings.Contains(out, "status-page") {
		t.Errorf("show output = %q, want both links listed", out)
	}
}

// TestShowNestedAreaLinksDoNotLeakToTheAncestor confirms showLinks' Direct
// scoping (para-6g7): an area's own show summary must not include a nested
// sub-area's links.
func TestShowNestedAreaLinksDoNotLeakToTheAncestor(t *testing.T) {
	chdirToTestTree(t)
	mustAddViaCLI(t, "area", "health")
	mustAddViaCLI(t, "area", "health.training")
	addLink(t, "area.health.training.blog", "--type", "blog", "--ref", "https://example.com")

	out, err := execRead(newShowCmd(), []string{"area", "health"})
	if err != nil {
		t.Fatalf("show area health: %v (%s)", err, out)
	}
	if strings.Contains(out, "links") {
		t.Errorf("show area health = %q, want no links section — the link belongs to health.training", out)
	}
}

// TestSetLinkCannotChangeType is §15's RequiredFixed row, reused from
// key-result for link's own opaque tag.
func TestSetLinkCannotChangeType(t *testing.T) {
	chdirToTestTree(t)
	mustAddViaCLI(t, "project", "acme")
	addLink(t, "project.acme.jira-epic")

	out, err := execSet([]string{"link", "project.acme.jira-epic", "--type", "jira-issue"})
	if err == nil {
		t.Fatalf("set link --type succeeded, want a refusal (output: %s)", out)
	}
}

// TestAddLinkChainCompletion is para-6g7's own completion (complete.go's
// linkParentChainCandidates): `add link <TAB>` offers all three parent
// kinds' existing chains, each prefixed with its own selector word.
func TestAddLinkChainCompletion(t *testing.T) {
	chdirToTestTree(t)
	mustAddViaCLI(t, "project", "acme")
	mustAddViaCLI(t, "area", "health")

	got, _ := completeAddChain(kindmeta.KindLink)(nil, nil, "")
	for _, want := range []string{"project.acme.", "area.health."} {
		if !slices.Contains(got, want) {
			t.Errorf("completeAddChain(link) = %v, want it to include %q", got, want)
		}
	}
}
