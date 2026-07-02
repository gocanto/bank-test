// Package errorsx holds the generic, domain-agnostic mapping from an error to an
// Encore error response: an errs.ErrCode, a client-facing message, and the HTTP
// status carried in the response details. Callers classify their own
// domain-specific errors into a Fault and hand it to Fail; nothing here depends
// on any particular bounded context.
package errorsx

import (
	"net/http"

	"encore.dev/beta/errs"
)

// Details is attached to every error response so the HTTP status is available
// to clients alongside the Encore error code.
type Details struct {
	StatusCode int `json:"status_code"`
}

// Fault is a classified error: the Encore code, the client-facing message, and
// the HTTP status derived from the code.
type Fault struct {
	Code       errs.ErrCode
	Message    string
	StatusCode int
}

func (Details) ErrDetails() {}

// Describe builds a Fault, filling StatusCode from the Encore code.
func Describe(code errs.ErrCode, message string) Fault {
	return Fault{
		Code:       code,
		Message:    message,
		StatusCode: status(code),
	}
}

// Fail wraps err into an Encore error using the classified Fault. A nil err
// returns nil so callers can pass results through unconditionally.
func Fail(err error, fault Fault) error {
	if err == nil {
		return nil
	}

	return errs.B().
		Code(fault.Code).
		Cause(err).
		Msg(fault.Message).
		Details(Details{StatusCode: fault.StatusCode}).
		Err()
}

func status(code errs.ErrCode) int {
	switch code {
	case errs.InvalidArgument, errs.FailedPrecondition:
		return http.StatusBadRequest
	case errs.AlreadyExists:
		return http.StatusConflict
	case errs.NotFound:
		return http.StatusNotFound
	default:
		return http.StatusInternalServerError
	}
}
