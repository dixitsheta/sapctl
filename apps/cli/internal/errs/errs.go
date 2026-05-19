// Package errs defines the centralized error type and locked exit codes for
// the sapctl CLI. Exit codes are part of the public CLI contract (ADR 0002)
// and MUST NOT change without a major-version bump.
package errs

import "errors"

// Exit codes locked by ADR 0002.
const (
	ExitSuccess     = 0
	ExitUserError   = 2
	ExitNotFound    = 3
	ExitConflict    = 4
	ExitAuth        = 5
	ExitRateLimited = 7
)

// Error carries an exit code, a stable machine tag, and a human-readable
// message. Wrapping is supported via errors.Is / errors.As.
type Error struct {
	Code    int    // process exit code (see constants above)
	Tag     string // stable machine identifier, e.g. "auth.token_expired"
	Message string // human-readable message
	Hint    string // optional remediation hint
	Wrapped error
}

func (e *Error) Error() string {
	if e.Wrapped != nil {
		return e.Message + ": " + e.Wrapped.Error()
	}
	return e.Message
}

func (e *Error) Unwrap() error { return e.Wrapped }

// New builds a sapctl Error.
func New(code int, tag, msg string) *Error {
	return &Error{Code: code, Tag: tag, Message: msg}
}

// Wrap attaches an inner error.
func Wrap(code int, tag, msg string, inner error) *Error {
	return &Error{Code: code, Tag: tag, Message: msg, Wrapped: inner}
}

// CodeOf returns the exit code attached to err, or ExitUserError if err is not
// a sapctl Error. Returns ExitSuccess only for nil.
func CodeOf(err error) int {
	if err == nil {
		return ExitSuccess
	}
	var e *Error
	if errors.As(err, &e) {
		return e.Code
	}
	return ExitUserError
}
