package cli_test

import (
	"bytes"
	"strings"
	"testing"
	"time"

	"github.com/colchuck-ai/para/internal/cli"
	"github.com/colchuck-ai/para/internal/clock"
)

func TestRunVersionFlagPrintsVersionAndExitsZero(t *testing.T) {
	var stdout, stderr bytes.Buffer
	clk := clock.Fixed{At: time.Date(2026, time.August, 4, 0, 0, 0, 0, time.UTC)}

	code := cli.Run(clk, []string{"--version"}, &stdout, &stderr)

	if code != 0 {
		t.Errorf("exit code = %d, want 0; stderr = %q", code, stderr.String())
	}
	if got := stdout.String(); !strings.HasPrefix(got, "para ") {
		t.Errorf("stdout = %q, want it to start with %q", got, "para ")
	}
	if stderr.String() != "" {
		t.Errorf("stderr = %q, want empty", stderr.String())
	}
}

func TestRunUnknownCommandIsUsageErrorExitingOne(t *testing.T) {
	var stdout, stderr bytes.Buffer

	code := cli.Run(clock.System{}, []string{"no-such-command"}, &stdout, &stderr)

	if code != 1 {
		t.Errorf("exit code = %d, want 1", code)
	}
}

func TestRunWithNoArgsPrintsHelpAndExitsZero(t *testing.T) {
	var stdout, stderr bytes.Buffer

	code := cli.Run(clock.System{}, []string{}, &stdout, &stderr)

	if code != 0 {
		t.Errorf("exit code = %d, want 0; stderr = %q", code, stderr.String())
	}
	if got := stdout.String(); !strings.Contains(got, "para") {
		t.Errorf("stdout = %q, want it to mention para", got)
	}
}
