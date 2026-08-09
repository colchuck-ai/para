package paraerr_test

import (
	"errors"
	"fmt"
	"testing"

	"github.com/colchuck-ai/para/internal/paraerr"
)

func TestExitCode(t *testing.T) {
	cases := []struct {
		name string
		err  error
		want int
	}{
		{"nil is success", nil, 0},
		{"advisory-only error is exit 2", paraerr.New(paraerr.KindAdvisory, "advisory finding"), 2},
		{"validation error is exit 1", paraerr.New(paraerr.KindValidation, "bad input"), 1},
		{"usage error is exit 1", paraerr.New(paraerr.KindUsage, "bad flag"), 1},
		{"not-found error is exit 1", paraerr.New(paraerr.KindNotFound, "no such locator"), 1},
		{"conflict error is exit 1", paraerr.New(paraerr.KindConflict, "already exists"), 1},
		{"internal error is exit 1", paraerr.New(paraerr.KindInternal, "unreachable"), 1},
		{"wrapped advisory error is exit 2", fmt.Errorf("outer: %w", paraerr.New(paraerr.KindAdvisory, "inner")), 2},
		{"plain non-paraerr error is exit 1", errors.New("boom"), 1},
		{"a status error carries its own code", paraerr.Status(2), 2},
		{"a status error of 1 is exit 1", paraerr.Status(1), 1},
		{"a wrapped status error carries its code", fmt.Errorf("outer: %w", paraerr.Status(2)), 2},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := paraerr.ExitCode(tc.err); got != tc.want {
				t.Errorf("ExitCode(%v) = %d, want %d", tc.err, got, tc.want)
			}
		})
	}
}

func TestErrorMessage(t *testing.T) {
	base := errors.New("underlying")
	err := paraerr.Wrap(paraerr.KindValidation, base, "field is invalid")

	if got, want := err.Error(), "field is invalid: underlying"; got != want {
		t.Errorf("Error() = %q, want %q", got, want)
	}
	if !errors.Is(err, base) {
		t.Error("errors.Is should find the wrapped base error")
	}
}

func TestNewHasNoWrappedError(t *testing.T) {
	err := paraerr.New(paraerr.KindUsage, "bad flag")
	if got, want := err.Error(), "bad flag"; got != want {
		t.Errorf("Error() = %q, want %q", got, want)
	}
	if errors.Unwrap(err) != nil {
		t.Error("New() should not wrap an underlying error")
	}
}

// TestIsStatus pins the other half of Status's contract: the command runner
// asks this before printing a message, and a real error must never answer yes.
func TestIsStatus(t *testing.T) {
	if !paraerr.IsStatus(paraerr.Status(2)) {
		t.Error("IsStatus(Status(2)) = false, want true")
	}
	if !paraerr.IsStatus(fmt.Errorf("outer: %w", paraerr.Status(1))) {
		t.Error("IsStatus of a wrapped status error = false, want true")
	}
	if paraerr.IsStatus(paraerr.New(paraerr.KindValidation, "bad input")) {
		t.Error("IsStatus of a validation error = true, want false")
	}
	if paraerr.IsStatus(nil) {
		t.Error("IsStatus(nil) = true, want false")
	}
}

func TestKindString(t *testing.T) {
	cases := []struct {
		kind paraerr.Kind
		want string
	}{
		{paraerr.KindUsage, "usage"},
		{paraerr.KindNotFound, "not-found"},
		{paraerr.KindValidation, "validation"},
		{paraerr.KindConflict, "conflict"},
		{paraerr.KindInternal, "internal"},
		{paraerr.KindAdvisory, "advisory"},
	}
	for _, tc := range cases {
		if got := tc.kind.String(); got != tc.want {
			t.Errorf("Kind(%d).String() = %q, want %q", tc.kind, got, tc.want)
		}
	}
}
