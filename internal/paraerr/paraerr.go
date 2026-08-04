// Package paraerr defines para's error taxonomy and its mapping to process
// exit codes (design §0.5).
package paraerr

import (
	"errors"
	"fmt"
)

// Kind categorizes an Error for exit-code mapping and script-test assertions.
// Script tests assert on Kind, never on message prose.
type Kind int

const (
	// KindUnknown is the zero value and should not be constructed directly.
	KindUnknown Kind = iota
	// KindUsage marks a malformed invocation: bad flags, wrong argument shape.
	KindUsage
	// KindNotFound marks a locator, field, or file that does not exist.
	KindNotFound
	// KindValidation marks a value that failed a domain rule.
	KindValidation
	// KindConflict marks a collision: an existing locator, a duplicate id.
	KindConflict
	// KindInternal marks a bug in para itself.
	KindInternal
	// KindAdvisory marks a non-fatal finding (e.g. doctor's advisory-only
	// findings) that should exit 2 rather than 1.
	KindAdvisory
)

func (k Kind) String() string {
	switch k {
	case KindUsage:
		return "usage"
	case KindNotFound:
		return "not-found"
	case KindValidation:
		return "validation"
	case KindConflict:
		return "conflict"
	case KindInternal:
		return "internal"
	case KindAdvisory:
		return "advisory"
	default:
		return "unknown"
	}
}

// Error is para's error type: a Kind for exit-code mapping and script-test
// assertions, a human message, and an optional wrapped cause.
type Error struct {
	Kind Kind
	Msg  string
	Err  error
}

// New constructs an Error with no wrapped cause.
func New(kind Kind, msg string) *Error {
	return &Error{Kind: kind, Msg: msg}
}

// Newf constructs an Error with a formatted message.
func Newf(kind Kind, format string, args ...any) *Error {
	return &Error{Kind: kind, Msg: fmt.Sprintf(format, args...)}
}

// Wrap constructs an Error that wraps an underlying cause.
func Wrap(kind Kind, err error, msg string) *Error {
	return &Error{Kind: kind, Msg: msg, Err: err}
}

func (e *Error) Error() string {
	if e.Err != nil {
		return fmt.Sprintf("%s: %s", e.Msg, e.Err)
	}
	return e.Msg
}

func (e *Error) Unwrap() error {
	return e.Err
}

// ExitCode maps err to a process exit code per design §0.5:
//
//	0  success (err is nil)
//	1  any command error
//	2  advisory-only findings (e.g. doctor with no error-severity finding)
func ExitCode(err error) int {
	if err == nil {
		return 0
	}
	var perr *Error
	if errors.As(err, &perr) && perr.Kind == KindAdvisory {
		return 2
	}
	return 1
}
