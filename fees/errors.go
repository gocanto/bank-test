package fees

import (
	"errors"

	"encore.dev/beta/errs"
	"go.temporal.io/api/serviceerror"
	"go.temporal.io/sdk/temporal"
	errorsx "gocanto.sh/bank/internal/errors"
	"gocanto.sh/bank/internal/fees/domain"
)

// fail classifies a fees error and wraps it into an Encore error. A nil err
// returns nil so handlers can pass results through unconditionally.
func fail(err error) error {
	if err == nil {
		return nil
	}

	return errorsx.Fail(err, classify(err))
}

// classify maps a fees domain or Temporal error onto a generic Fault. It is the
// fees-specific half of error handling; the generic code/status mapping lives
// in internal/errors.
func classify(err error) errorsx.Fault {
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
		return errorsx.Describe(errs.InvalidArgument, err.Error())
	case errors.Is(err, domain.ErrDuplicateLineItem):
		return errorsx.Describe(errs.AlreadyExists, err.Error())
	case errors.As(err, &alreadyStarted):
		return errorsx.Describe(errs.AlreadyExists, "bill already exists")
	case errors.As(err, &notFound):
		return errorsx.Describe(errs.NotFound, "bill not found")
	case errors.Is(err, domain.ErrBillClosed), errors.Is(err, domain.ErrBillAlreadyClosed):
		return errorsx.Describe(errs.FailedPrecondition, err.Error())
	default:
		return errorsx.Describe(errs.Unknown, err.Error())
	}
}
