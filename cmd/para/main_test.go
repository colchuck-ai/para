package main

import (
	"fmt"
	"os"
	"testing"

	"github.com/rogpeppe/go-internal/testscript"
)

// TestMain lets this test binary re-exec itself as `para` (rogpeppe/go-internal's
// standard testscript pattern), so script tests run the real command
// dispatch and exit-code path, not a mock.
func TestMain(m *testing.M) {
	testscript.Main(m, map[string]func(){
		"para": func() { os.Exit(run()) },
	})
}

// noConditions refuses every testscript condition, which is what the scripts
// are now entitled to expect.
//
// Until Phase 14 every §26 command sat behind a `[para:phaseN]` gate, flipped on
// as its phase landed (§0.1). The ground rules say what the end of that is:
// "when every gate is on, the CLI is spec-complete by construction". Every gate
// is on, so the gates are gone and the scripts are plain conformance scripts —
// and a `[para:phaseN]` reappearing in one is now an error rather than a stanza
// that quietly does not run, which is the failure mode a gate that outlived its
// phase would have.
func noConditions(cond string) (bool, error) {
	return false, fmt.Errorf("unknown testscript condition %q: the phase gates were removed in Phase 14", cond)
}

func TestScripts(t *testing.T) {
	// Every script from Phase 8 on writes journal files named for the instant
	// they were written and ACTIVITY.md sections dated by it, so the whole
	// suite needs a fixed clock — which production deliberately refuses to read
	// (§0.2). Without the tag the assertions would be about the host's wall
	// clock, so the honest thing is to say the suite did not run. `make test`
	// supplies the tag; so does CI.
	if !testHooksEnabled {
		t.Skip("script tests need -tags para_testhooks so PARA_NOW fixes the clock; run `make test`")
	}
	testscript.Run(t, testscript.Params{
		Dir:       "../../testdata/script",
		Condition: noConditions,
	})
}
