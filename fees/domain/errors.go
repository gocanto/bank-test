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
