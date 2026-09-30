package dbSchema

import (
	"errors"
	"fmt"
)

// Error codes. A code classifies a failure so callers can decide what to do
// with it without matching on messages.
const (
	// EINVALID marks a schema this package cannot express as a table. It is
	// a problem with one schema, not with the run, so callers are expected
	// to report it and carry on with the schemas that remain.
	EINVALID = "invalid"

	// EINTERNAL marks a failure in this package itself. Callers should stop.
	EINTERNAL = "internal"
)

// Error is the one error type this package returns.
//
// A leaf error carries Code and Message and says what went wrong. A wrapping
// error carries Op and Err and says where. The two are never mixed on one
// value, so a chain reads as a single logical stack trace:
//
//	dbSchema.BuildTableFromSchema: dbSchema.buildColumnFromProperty: no data type for property
//
// metadata
type Error struct {
	Code    string // machine-readable; set only on leaf errors
	Message string // human-readable; set only on leaf errors
	Op      string // "package.Function"; set only on wrapping errors
	Err     error  // nested cause; set only on wrapping errors
}

// ErrorCode returns the code of the innermost *Error in err's chain, or
// EINTERNAL for an error this package did not classify. It returns "" for a
// nil error.
func ErrorCode(err error) string {
	var e *Error

	switch {
	case err == nil:
		return ""
	case !errors.As(err, &e):
		return EINTERNAL
	case e.Code != "":
		return e.Code
	case e.Err != nil:
		return ErrorCode(e.Err)
	default:
		return EINTERNAL
	}
}

// ErrorMessage returns the message of the innermost *Error in err's chain,
// without the chain of operations that wraps it, for reporting to a user. It
// returns "" for a nil error.
func ErrorMessage(err error) string {
	var e *Error

	switch {
	case err == nil:
		return ""
	case !errors.As(err, &e):
		return err.Error()
	case e.Message != "":
		return e.Message
	case e.Err != nil:
		return ErrorMessage(e.Err)
	default:
		return "internal error"
	}
}

func (e *Error) Error() string {
	if e.Op != "" {
		return fmt.Sprintf("%s: %v", e.Op, e.Err)
	}

	return e.Message
}

// Unwrap exposes the nested cause to errors.Is and errors.As.
func (e *Error) Unwrap() error { return e.Err }
