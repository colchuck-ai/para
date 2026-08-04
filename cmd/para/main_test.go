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
	0: true,
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
	testscript.Run(t, testscript.Params{
		Dir:       "../../testdata/script",
		Condition: phaseCondition,
	})
}
