package cli_test

import (
	"strings"
	"testing"
)

// mustRun runs args against root and fails the test on a nonzero exit,
// for fixture-building steps this file's own tests depend on rather than
// are testing.
func mustRun(t *testing.T, root string, args ...string) {
	t.Helper()
	code, stdout, stderr := run(t, root, args...)
	if code != 0 {
		t.Fatalf("%v: exit %d, stdout = %q, stderr = %q", args, code, stdout, stderr)
	}
}

// mixedKindTree plants a project, its objective, and its key-result — R23's
// own worked example — with a fixed `created` so every row's age column
// reads "today" against fixedClock, and no other bucket populated, so an
// unscoped `list` returns exactly these three.
func mixedKindTree(t *testing.T) string {
	t.Helper()
	root := plantTree(t, map[string]string{
		".para/state.toml":          "",
		"projects/.para/state.toml": "name = \"Projects\"\n",
	})

	mustRun(t, root, "add", "project", "acme",
		"--name", "Acme", "--description", "x", "--status", "in-progress",
		"--created", "2026-08-05T00:00:00Z")
	mustRun(t, root, "add", "objective", "acme.q1-growth",
		"--name", "Grow signups", "--description", "x", "--status", "in-progress",
		"--created", "2026-08-05T00:00:00Z")
	mustRun(t, root, "add", "key-result", "acme.q1-growth.signups",
		"--name", "Weekly signups", "--target", "2000/12000", "--type", "ratio",
		"--created", "2026-08-05T00:00:00Z")
	return root
}

// TestListMixedKindColumnsAreAligned is R23's own worked example: a project,
// its objective, and its key-result print as three rows of one table, byte
// for byte, the noun and the chain each their own column rather than folded
// into one dotted address, aligned to the widest entry above each of them.
func TestListMixedKindColumnsAreAligned(t *testing.T) {
	root := mixedKindTree(t)

	code, stdout, stderr := run(t, root, "list")
	if code != 0 {
		t.Fatalf("list: exit %d, stderr = %q", code, stderr)
	}
	want := "project     acme                    in-progress  today\n" +
		"objective   acme.q1-growth          in-progress  today\n" +
		"key-result  acme.q1-growth.signups  —            today\n" +
		"showing 3 of 3\n"
	if stdout != want {
		t.Errorf("list =\n%q\nwant\n%q", stdout, want)
	}
}

// TestListKindFilterWithScopeReturnsOnlyThatKind is the epic's own
// done-when line: `para list key-result project acme` returns only the
// key-results inside project acme, R19's `<noun> <noun> <chain>` row read
// end to end through the real command. A second project with its own
// key-result proves the scope is doing the narrowing — `list key-result`
// alone would return both.
func TestListKindFilterWithScopeReturnsOnlyThatKind(t *testing.T) {
	root := mixedKindTree(t)
	mustRun(t, root, "add", "project", "other",
		"--name", "Other", "--description", "x", "--status", "in-progress",
		"--created", "2026-08-05T00:00:00Z")
	mustRun(t, root, "add", "objective", "other.q1",
		"--name", "Other objective", "--description", "x", "--status", "in-progress",
		"--created", "2026-08-05T00:00:00Z")
	mustRun(t, root, "add", "key-result", "other.q1.metric",
		"--name", "Other metric", "--target", "10/20", "--type", "ratio",
		"--created", "2026-08-05T00:00:00Z")

	code, stdout, stderr := run(t, root, "list", "key-result", "project", "acme")
	if code != 0 {
		t.Fatalf("list key-result project acme: exit %d, stderr = %q", code, stderr)
	}
	want := "key-result  acme.q1-growth.signups  —  today\n" +
		"showing 1 of 1\n"
	if stdout != want {
		t.Errorf("list key-result project acme =\n%q\nwant\n%q", stdout, want)
	}

	// Unscoped, the kind filter alone reaches both.
	code, stdout, stderr = run(t, root, "list", "key-result")
	if code != 0 {
		t.Fatalf("list key-result: exit %d, stderr = %q", code, stderr)
	}
	if !strings.Contains(stdout, "acme.q1-growth.signups") || !strings.Contains(stdout, "other.q1.metric") {
		t.Errorf("list key-result = %q, want both key-results tree-wide", stdout)
	}
}

// TestListChainColumnAtArbitraryDepth is areas' and resources' own shape
// (R4): unlike project/objective/key-result's fixed arity, they nest to any
// depth, and the chain column (R23) prints every segment of that nesting
// exactly as address.FromLocator resolves it, not just the leaf id.
func TestListChainColumnAtArbitraryDepth(t *testing.T) {
	root := plantTree(t, map[string]string{
		".para/state.toml":       "",
		"areas/.para/state.toml": "name = \"Areas\"\n",
	})
	mustRun(t, root, "add", "area", "health",
		"--name", "Health", "--description", "x",
		"--created", "2026-08-05T00:00:00Z")
	mustRun(t, root, "add", "area", "health.training",
		"--name", "Training", "--description", "x",
		"--created", "2026-08-05T00:00:00Z")

	code, stdout, stderr := run(t, root, "list")
	if code != 0 {
		t.Fatalf("list: exit %d, stderr = %q", code, stderr)
	}
	wantChains := map[string]bool{"health": false, "health.training": false}
	for _, line := range strings.Split(strings.TrimRight(stdout, "\n"), "\n") {
		fields := strings.Fields(line)
		if len(fields) >= 2 {
			if _, ok := wantChains[fields[1]]; ok {
				wantChains[fields[1]] = true
			}
		}
	}
	for chain, found := range wantChains {
		if !found {
			t.Errorf("list = %q, want a row whose chain column is %q", stdout, chain)
		}
	}
}

// TestListCountAndHiddenLinesAreUnchanged is P20.6: the two lines below the
// table answer different questions — "is that everything" and "why isn't
// it" — and stay two lines with their existing wording, unaffected by the
// noun column or the kind filter this phase added.
func TestListCountAndHiddenLinesAreUnchanged(t *testing.T) {
	root := mixedKindTree(t)
	mustRun(t, root, "set", "project", "acme", "--status", "done")

	code, stdout, stderr := run(t, root, "list")
	if code != 0 {
		t.Fatalf("list: exit %d, stderr = %q", code, stderr)
	}
	want := "showing 0 of 0\n3 hidden (done) — --all to include\n"
	if stdout != want {
		t.Errorf("list =\n%q\nwant\n%q", stdout, want)
	}

	code, stdout, stderr = run(t, root, "list", "--all")
	if code != 0 {
		t.Fatalf("list --all: exit %d, stderr = %q", code, stderr)
	}
	if strings.Contains(stdout, "hidden") {
		t.Errorf("list --all = %q, want no hidden line", stdout)
	}
	if !strings.Contains(stdout, "showing 3 of 3") {
		t.Errorf("list --all = %q, want the count line to report all 3", stdout)
	}
}

// TestListContainerIsRefusedAsAFilter is R20, read end to end through the
// real command rather than the parser alone: `container` is a legal noun
// everywhere else an address is read, but never in list's filter position.
func TestListContainerIsRefusedAsAFilter(t *testing.T) {
	root := mixedKindTree(t)

	code, _, stderr := run(t, root, "list", "container")
	if code == 0 {
		t.Fatal("list container: want a refusal, got none")
	}
	if want := "not a legal filter"; !strings.Contains(stderr, want) {
		t.Errorf("list container: stderr = %q, want it to contain %q", stderr, want)
	}
}
