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

// Status is an error that carries nothing but an exit code, for the command
// that has already said everything it has to say on stdout.
//
// `doctor` is the only caller and §21.2 is why it needs one: its findings *are*
// its output, and the exit code is a second channel that says how to read them
// — "CI gates on 1 and ignores 2; an agent learns from the code alone whether
// judgement is required". A tree with three findings is not a command that
// failed, so printing "error: 3 findings" beneath a report that already lists
// them would be noise §26's own transcript does not show.
//
// It is spelled as an error rather than as a second return value because the
// exit code has to travel out through cobra's RunE, which has room for exactly
// one thing.
func Status(code int) error { return statusError{code: code} }

// statusError is Status's implementation. Its message exists for the %v of a
// stray log line and is never what a user reads.
type statusError struct{ code int }

func (e statusError) Error() string { return fmt.Sprintf("exit status %d", e.code) }

// IsStatus reports whether err is one of Status's — the test the command
// runner makes before printing an error message.
func IsStatus(err error) bool {
	var se statusError
	return errors.As(err, &se)
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
	var se statusError
	if errors.As(err, &se) {
		return se.code
	}
	var perr *Error
	if errors.As(err, &perr) && perr.Kind == KindAdvisory {
		return 2
	}
	return 1
}
