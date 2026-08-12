package cli_test

import (
	"strings"
	"testing"
)

// TestRemoveContainerRefusalUsesTheDottedAddress is para-jtr's own live
// repro: mutate.relocatable's container/stub/not-exist refusals used to
// print the old plural-bucket locator form directly (e.g.
// "projects.acme.objectives is a container ..."), not the R24 dotted
// address form every other CLI message uses.
func TestRemoveContainerRefusalUsesTheDottedAddress(t *testing.T) {
	root := plantTree(t, map[string]string{
		".para/state.toml":          "",
		"projects/.para/state.toml": "name = \"Projects\"\n",
	})
	mustRun(t, root, "add", "project", "acme", "--name", "Acme", "--description", "x")

	code, _, stderr := run(t, root, "remove", "container", "acme.objectives", "--force")
	if code == 0 {
		t.Fatal("remove container acme.objectives: want a refusal, got none")
	}
	if !strings.Contains(stderr, "container.acme.objectives is a container") {
		t.Errorf("stderr = %q, want it to name container.acme.objectives (R24's dotted form)", stderr)
	}
	if strings.Contains(stderr, "projects.acme.objectives") {
		t.Errorf("stderr = %q, still has the old plural-bucket form", stderr)
	}
}
