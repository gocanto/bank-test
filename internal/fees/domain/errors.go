package domain

import "errors"

var (
	ErrInvalidBillID      = errors.New("bill id is required")
	ErrInvalidLineItemID  = errors.New("line item id is required")
	ErrInvalidDescription = errors.New("line item description is required")
	ErrInvalidPeriod      = errors.New("period end must be after period start")
	ErrInvalidAmount      = errors.New("amount must be positive")
	ErrInvalidCurrency    = errors.New("currency must be USD or GEL")
	ErrDuplicateLineItem  = errors.New("line item already exists")
	ErrBillClosed         = errors.New("bill is closed")
	ErrBillAlreadyClosed  = errors.New("bill is already closed")
)

// Stable, transport-safe classification codes for domain errors. Callers that
// cross a boundary where the wrapped Go error identity is lost (e.g. a Temporal
// update returning its result over gRPC) carry one of these codes alongside the
// message so the caller can recover the error's meaning without errors.Is.
const (
	CodeValidation        = "VALIDATION"
	CodeBillClosed        = "BILL_CLOSED"
	CodeDuplicateLineItem = "DUPLICATE_LINE_ITEM"
	CodeInternal          = "INTERNAL"
)

// ErrorCode maps a domain error to its stable classification code. Recognised
// validation failures return their specific code; anything unrecognised is
// treated as internal so it is not silently downgraded to a client error.
func ErrorCode(err error) string {
	switch {
	case errors.Is(err, ErrBillClosed), errors.Is(err, ErrBillAlreadyClosed):
		return CodeBillClosed
	case errors.Is(err, ErrDuplicateLineItem):
		return CodeDuplicateLineItem
	case errors.Is(err, ErrInvalidBillID),
		errors.Is(err, ErrInvalidLineItemID),
		errors.Is(err, ErrInvalidDescription),
		errors.Is(err, ErrInvalidPeriod),
		errors.Is(err, ErrInvalidAmount),
		errors.Is(err, ErrInvalidCurrency):
		return CodeValidation
	default:
		return CodeInternal
	}
}
