package cli

import (
	"bytes"
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

// execMove runs args against a fresh `move` command in the cwd chdirToTestTree
// set up. Unlike add/set/unset, move never dispatches on a subcommand — the
// noun is a plain positional argument (R14: "move speaks its noun once") —
// so there is no HasParent()/Root() gotcha to work around here.
func execMove(args []string) (string, error) {
	cmd := newMoveCmd()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs(args)
	err := cmd.Execute()
	return out.String(), err
}

// mustAddArea adds an area (which may nest) through the real add dispatch.
func mustAddArea(t *testing.T, chain string) {
	t.Helper()
	root := &cobra.Command{Use: "para"}
	root.AddCommand(newAddCmd())
	var out bytes.Buffer
	root.SetOut(&out)
	root.SetErr(&out)
	root.SetArgs([]string{"add", "area", chain, "--name", chain, "--description", "x"})
	if err := root.Execute(); err != nil {
		t.Fatalf("add area %s: %v (%s)", chain, err, out.String())
	}
}

// TestMoveSameKindReparent is R14's own example: one noun, two chains, a
// same-kind reparent — the destination's parent (fitness) must already
// exist (§1.5's placement legality), so it is created first.
func TestMoveSameKindReparent(t *testing.T) {
	chdirToTestTree(t)
	mustAddArea(t, "health")
	mustAddArea(t, "health.training")
	mustAddArea(t, "fitness")

	out, err := execMove([]string{"area", "health.training", "fitness.training"})
	if err != nil {
		t.Fatalf("move area health.training fitness.training: %v (%s)", err, out)
	}
	if !strings.Contains(out, "area.health.training") || !strings.Contains(out, "area.fitness.training") {
		t.Errorf("move output = %q, want both the old and new address", out)
	}
}

// TestMoveDifferentKindRefused is §18.3's same-kind-only rule, reachable
// through the CLI only as a chain-arity mismatch now that a second noun has
// no spelling left to occupy (R14's own note: "naming a second noun to
// change kind is not a refusal move can even reach any more").
func TestMoveDifferentKindRefused(t *testing.T) {
	chdirToTestTree(t)
	mustAddArea(t, "health")

	// "objective" wants a two-segment chain; "health.new-name" is one
	// segment repeated as two, so R4's arity refusal fires before move ever
	// gets to ask whether an area can become an objective.
	_, err := execMove([]string{"objective", "health", "health.new-name"})
	if err == nil {
		t.Fatal("move objective health health.new-name: want a refusal, got none")
	}
}

// TestMoveArchivedBothEnds is R9: --archived on move means both ends are
// archived, checked against the noun spoken once for both chains.
func TestMoveArchivedBothEnds(t *testing.T) {
	chdirToTestTree(t)
	mustAddArea(t, "health")
	archiveArea(t, "health")

	out, err := execMove([]string{"area", "health", "wellness", "--archived"})
	if err != nil {
		t.Fatalf("move area health wellness --archived: %v (%s)", err, out)
	}
	if !strings.Contains(out, "archive.area.health") || !strings.Contains(out, "archive.area.wellness") {
		t.Errorf("move output = %q, want both ends under archive", out)
	}
}

// TestMoveArchivedFlagWrongSideIsNamed is R9's own worked example (§26):
// passing --archived for a source that is actually live is refused by
// name — "X is live" — rather than surfacing PlanMove's generic "does not
// exist", which would be equally true of a source that is simply missing.
func TestMoveArchivedFlagWrongSideIsNamed(t *testing.T) {
	chdirToTestTree(t)
	mustAddViaCLI(t, "project", "acme-migration")

	_, err := execMove([]string{"project", "acme-migration", "acme-renamed", "--archived"})
	if err == nil {
		t.Fatal("move project acme-migration acme-renamed --archived: want a refusal, got none")
	}
	want := "--archived means both ends are archived; project.acme-migration is live"
	if err.Error() != want {
		t.Errorf("error = %q, want %q", err.Error(), want)
	}
}

// archiveArea archives an area through the real `archive` command, which
// still takes the old single-locator form (P19.8 has not run yet) — used
// here only to put a real archived entity on disk for TestMoveArchivedBothEnds.
func archiveArea(t *testing.T, id string) {
	t.Helper()
	cmd := newArchiveCmd()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"areas." + id})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("archive areas.%s: %v (%s)", id, err, out.String())
	}
}
