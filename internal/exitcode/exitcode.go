// Package exitcode defines the process exit codes used by the newer command
// groups (starting with `lsh s3`) and the error type that carries them.
//
// Historically every lsh failure exited with 1. Object storage commands are
// used heavily in scripts and CI, where distinguishing "not found" from
// "no credentials" from "refused for safety" matters, so they attach an
// explicit code to their errors. main() maps the error to os.Exit through
// Of(); commands that do not opt in keep exiting with 1.
package exitcode

import (
	"errors"
	"fmt"
)

const (
	// OK is a successful run (including empty listings).
	OK = 0
	// Generic is an unexpected error, or one or more transfers failed.
	Generic = 1
	// Usage is invalid usage: bad URI/flag, ambiguous bucket, --recursive
	// without --all, and other argument errors.
	Usage = 2
	// NotFound covers a missing bucket, object, access key or lifecycle rule.
	NotFound = 3
	// Credentials means no usable S3 credential was found, or the backend
	// rejected the credential (InvalidAccessKeyId, SignatureDoesNotMatch).
	Credentials = 4
	// Permission is an authenticated request the backend refused (403).
	Permission = 5
	// Partial means the operation completed for some objects and failed for
	// others (recursive delete, bucket emptying).
	Partial = 6
	// Refused is a safety refusal: non-empty bucket without --force,
	// --max-delete exceeded, object lock, prompt declined, or a confirmation
	// required in a non-interactive session without --yes.
	Refused = 7
	// Interrupted is returned after a SIGINT once cleanup finished.
	Interrupted = 130
)

// OptInAnnotation marks a cobra command whose subtree uses these exit codes.
// Command groups that predate them keep exiting 1 for every failure, so shared
// pre-run validation has to know which contract applies. Commands set it on
// themselves (see cmd/s3.Finalize) and callers walk up the parents.
const OptInAnnotation = "lsh.exit_codes"

// Error pairs an error with the exit code the process should terminate with.
type Error struct {
	Code int
	Err  error
}

func (e *Error) Error() string {
	if e.Err == nil {
		return fmt.Sprintf("exit %d", e.Code)
	}
	return e.Err.Error()
}

// Unwrap exposes the wrapped error to errors.Is/As.
func (e *Error) Unwrap() error { return e.Err }

// New wraps err with an exit code. A nil err yields nil.
func New(code int, err error) error {
	if err == nil {
		return nil
	}
	return &Error{Code: code, Err: err}
}

// Errorf formats an error and attaches an exit code.
func Errorf(code int, format string, a ...interface{}) error {
	return &Error{Code: code, Err: fmt.Errorf(format, a...)}
}

// Of returns the exit code carried by err (the outermost *Error in the chain
// wins, as errors.As walks from the outside in), Generic when err carries
// none, and OK for nil.
func Of(err error) int {
	if err == nil {
		return OK
	}
	var e *Error
	if errors.As(err, &e) {
		return e.Code
	}
	return Generic
}
