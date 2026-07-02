package fees

import (
	"errors"

	"encore.dev/beta/errs"
	"go.temporal.io/api/serviceerror"
	"go.temporal.io/sdk/temporal"
	"gocanto.sh/bank/internal/errorsx"
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

	if errors.As(err, &applicationErr) {
		// Errors returned by a workflow update or its validator arrive here as
		// an ApplicationError whose Type carries the domain classification code;
		// the wrapped Go error does not survive the gRPC boundary, so match on
		// the code first and only fall through when it is unrecognised.
		if fault, ok := classifyCode(applicationErr); ok {
			return fault
		}

		if applicationErr.Unwrap() != nil {
			err = applicationErr.Unwrap()
		}
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

// classifyCode maps a transport-safe domain classification code carried on a
// Temporal ApplicationError onto a Fault. The bool is false when the code is not
// one the domain produces, so the caller can fall back to structural matching.
func classifyCode(appErr *temporal.ApplicationError) (errorsx.Fault, bool) {
	switch appErr.Type() {
	case domain.CodeValidation:
		return errorsx.Describe(errs.InvalidArgument, appErr.Message()), true
	case domain.CodeDuplicateLineItem:
		return errorsx.Describe(errs.AlreadyExists, appErr.Message()), true
	case domain.CodeBillClosed:
		return errorsx.Describe(errs.FailedPrecondition, appErr.Message()), true
	case domain.CodeInternal:
		return errorsx.Describe(errs.Unknown, appErr.Message()), true
	default:
		return errorsx.Fault{}, false
	}
}
