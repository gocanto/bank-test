package domain

import (
	"sort"
	"strings"
	"time"

	"github.com/gocanto/collection/kv"
	"github.com/gocanto/money/money"
)

type LineItem struct {
	ID          string    `json:"id"`
	Description string    `json:"description"`
	Amount      Money     `json:"amount"`
	CreatedAt   time.Time `json:"created_at"`
}

type Total struct {
	Amount   int64  `json:"amount"`
	Currency string `json:"currency"`
}

type Bill struct {
	ID          string     `json:"id"`
	State       string     `json:"state"`
	PeriodStart time.Time  `json:"period_start"`
	PeriodEnd   time.Time  `json:"period_end"`
	LineItems   []LineItem `json:"line_items"`
	Totals      []Total    `json:"totals"`
	CreatedAt   time.Time  `json:"created_at"`
	ClosedAt    *time.Time `json:"closed_at,omitempty"`
}

type CreateBill struct {
	BillID      string    `json:"bill_id"`
	PeriodStart time.Time `json:"period_start"`
	PeriodEnd   time.Time `json:"period_end"`
}

type AddLineItem struct {
	ID          string `json:"id"`
	Description string `json:"description"`
	Amount      Money  `json:"amount"`
}

func NewBill(req CreateBill, now time.Time) (*Bill, error) {
	req.BillID = strings.TrimSpace(req.BillID)

	if req.BillID == "" {
		return nil, ErrInvalidBillID
	}

	if !req.PeriodEnd.After(req.PeriodStart) {
		return nil, ErrInvalidPeriod
	}

	return &Bill{
		ID:          req.BillID,
		State:       StateOpen,
		PeriodStart: req.PeriodStart.UTC(),
		PeriodEnd:   req.PeriodEnd.UTC(),
		LineItems:   []LineItem{},
		Totals:      []Total{},
		CreatedAt:   now.UTC(),
	}, nil
}

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

func (b *Bill) AddLineItem(req AddLineItem, now time.Time) (*Bill, error) {
	if err := b.ValidateAddLineItem(req); err != nil {
		return nil, err
	}

	b.LineItems = append(b.LineItems, LineItem{
		ID:          strings.TrimSpace(req.ID),
		Description: strings.TrimSpace(req.Description),
		Amount:      req.Amount,
		CreatedAt:   now.UTC(),
	})

	if err := b.recalculateTotals(); err != nil {
		return nil, err
	}

	return b, nil
}

// ValidateClose reports whether the bill may be closed without mutating state.
// Shared by Close and by the workflow's close update validator.
func (b *Bill) ValidateClose() error {
	if b.State == StateClosed {
		return ErrBillAlreadyClosed
	}

	return nil
}

func (b *Bill) Close(now time.Time) (*Bill, error) {
	if err := b.ValidateClose(); err != nil {
		return nil, err
	}

	engine, err := billStateMachine()

	if err != nil {
		return nil, err
	}

	if _, err := engine.Apply(b, "close", nil); err != nil {
		return nil, err
	}

	b.ClosedAt = new(now.UTC())

	if err := b.recalculateTotals(); err != nil {
		return nil, err
	}

	return b, nil
}

func (b *Bill) Summary() Bill {
	if b == nil {
		return Bill{}
	}

	summary := *b
	summary.LineItems = append([]LineItem(nil), b.LineItems...)
	summary.Totals = append([]Total(nil), b.Totals...)

	return summary
}

// recalculateTotals recomputes per-currency totals from the current line items.
// Any aggregation failure is returned rather than skipped: a dropped currency
// would silently under-report the amount being charged, which must never happen
// for a monetary total.
func (b *Bill) recalculateTotals() error {
	grouped := map[string][]*money.Money{}

	for _, item := range b.LineItems {
		code := strings.ToUpper(item.Amount.Currency)
		grouped[code] = append(grouped[code], item.Amount.library())
	}

	totals := make([]Total, 0, len(grouped))
	aggregator := money.NewAggregator(money.NewManager())

	for _, values := range grouped {
		sum, err := aggregator.Sum(values...)

		if err != nil {
			return err
		}

		amount, err := sum.Amount()

		if err != nil {
			return err
		}

		curr, err := sum.Currency()

		if err != nil {
			return err
		}

		totals = append(totals, Total{Amount: amount, Currency: curr.Code})
	}

	sort.Slice(totals, func(i, j int) bool {
		return totals[i].Currency < totals[j].Currency
	})

	b.Totals = totals

	return nil
}
