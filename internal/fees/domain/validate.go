package domain

import (
	"strings"

	"github.com/gocanto/collection/kv"
)

// ValidateAddLineItem reports whether req may be added to the bill without
// mutating any state. It is the single source of truth for add-line-item rules,
// shared by AddLineItem and by the workflow's update validator so invalid
// updates can be rejected before they are admitted.
func (b *Bill) ValidateAddLineItem(req AddLineItem) error {
	if b.State == StateClosed {
		return ErrBillClosed
	}

	id := strings.TrimSpace(req.ID)

	if id == "" {
		return ErrInvalidLineItemID
	}

	if strings.TrimSpace(req.Description) == "" {
		return ErrInvalidDescription
	}

	if err := req.Amount.Validate(); err != nil {
		return err
	}

	seen := map[string]any{}

	for _, item := range b.LineItems {
		kv.Set(seen, item.ID, true, false)
	}

	if kv.Has(seen, id) {
		return ErrDuplicateLineItem
	}

	return nil
}

// ValidateClose reports whether the bill may be closed without mutating state.
// Shared by Close and by the workflow's close update validator.
func (b *Bill) ValidateClose() error {
	if b.State == StateClosed {
		return ErrBillAlreadyClosed
	}

	return nil
}
