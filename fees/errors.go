package fees

import (
	"errors"
	"net/http"

	"encore.dev/beta/errs"
	"go.temporal.io/api/serviceerror"
	"go.temporal.io/sdk/temporal"
	"gocanto.sh/bank/internal/fees/domain"
)

type details struct {
	StatusCode int `json:"status_code"`
}

type fault struct {
	code       errs.ErrCode
	message    string
	statusCode int
}

func (details) ErrDetails() {}

func fail(err error) error {
	if err == nil {
		return nil
	}

	response := classify(err)

	return errs.B().
		Code(response.code).
		Cause(err).
		Msg(response.message).
		Details(details{StatusCode: response.statusCode}).
		Err()
}

func classify(err error) fault {
	var applicationErr *temporal.ApplicationError

	if errors.As(err, &applicationErr) && applicationErr.Unwrap() != nil {
		err = applicationErr.Unwrap()
	}

	var notFound *serviceerror.NotFound

	var alreadyStarted *serviceerror.WorkflowExecutionAlreadyStarted

	switch {
	case errors.Is(err, domain.ErrInvalidBillID),
		errors.Is(err, domain.ErrInvalidLineItemID),
		errors.Is(err, domain.ErrInvalidDescription),
		errors.Is(err, domain.ErrInvalidPeriod),
		errors.Is(err, domain.ErrInvalidAmount),
		errors.Is(err, domain.ErrInvalidCurrency):
		return describe(errs.InvalidArgument, err.Error())
	case errors.Is(err, domain.ErrDuplicateLineItem):
		return describe(errs.AlreadyExists, err.Error())
	case errors.As(err, &alreadyStarted):
		return describe(errs.AlreadyExists, "bill already exists")
	case errors.As(err, &notFound):
		return describe(errs.NotFound, "bill not found")
	case errors.Is(err, domain.ErrBillClosed), errors.Is(err, domain.ErrBillAlreadyClosed):
		return describe(errs.FailedPrecondition, err.Error())
	default:
		return describe(errs.Unknown, err.Error())
	}
}

func describe(code errs.ErrCode, message string) fault {
	return fault{
		code:       code,
		message:    message,
		statusCode: status(code),
	}
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
