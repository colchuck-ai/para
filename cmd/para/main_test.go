package main

import (
	"fmt"
	"os"
	"strconv"
	"strings"
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

// implementedPhases gates §26 conformance commands behind a [para:phaseN]
// testscript condition, flipped on as each phase lands, per the
// implementation plan's ground rules (§0.1).
var implementedPhases = map[int]bool{
	0:  true,
	4:  true,
	7:  true,
	8:  true,
	9:  true,
	10: true,
	11: true,
	12: true,
}

const phaseConditionPrefix = "para:phase"

func phaseCondition(cond string) (bool, error) {
	if !strings.HasPrefix(cond, phaseConditionPrefix) {
		return false, fmt.Errorf("unknown testscript condition %q", cond)
	}
	n, err := strconv.Atoi(strings.TrimPrefix(cond, phaseConditionPrefix))
	if err != nil {
		return false, fmt.Errorf("invalid phase condition %q: %w", cond, err)
	}
	return implementedPhases[n], nil
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
		Condition: phaseCondition,
	})
}
