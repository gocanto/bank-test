package domain

import (
	"sort"
	"strings"
	"time"

	"github.com/gocanto/collection/kv"
	"github.com/gocanto/money/money"
	"github.com/oullin/workflow/store"
	oworkflow "github.com/oullin/workflow/workflow"
)

const (
	StateOpen   = "open"
	StateClosed = "closed"
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

func (b *Bill) GetState() string {
	if b == nil {
		return ""
	}

	return b.State
}

func (b *Bill) SetState(state string) {
	if b != nil {
		b.State = state
	}
}

func (b *Bill) AddLineItem(req AddLineItem, now time.Time) (*Bill, error) {
	if b.State == StateClosed {
		return nil, ErrBillClosed
	}

	req.ID = strings.TrimSpace(req.ID)
	req.Description = strings.TrimSpace(req.Description)

	if req.ID == "" {
		return nil, ErrInvalidLineItemID
	}

	if req.Description == "" {
		return nil, ErrInvalidDescription
	}

	if err := req.Amount.Validate(); err != nil {
		return nil, err
	}

	seen := map[string]any{}
	for _, item := range b.LineItems {
		kv.Set(seen, item.ID, true, false)
	}

	if kv.Has(seen, req.ID) {
		return nil, ErrDuplicateLineItem
	}

	b.LineItems = append(b.LineItems, LineItem{
		ID:          req.ID,
		Description: req.Description,
		Amount:      req.Amount,
		CreatedAt:   now.UTC(),
	})
	b.recalculateTotals()

	return b, nil
}

func (b *Bill) Close(now time.Time) (*Bill, error) {
	if b.State == StateClosed {
		return nil, ErrBillAlreadyClosed
	}

	engine, err := billStateMachine()
	if err != nil {
		return nil, err
	}

	if _, err := engine.Apply(b, "close", nil); err != nil {
		return nil, err
	}

	closedAt := now.UTC()
	b.ClosedAt = &closedAt
	b.recalculateTotals()

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

func (b *Bill) recalculateTotals() {
	grouped := map[string][]*money.Money{}

	for _, item := range b.LineItems {
		code := strings.ToUpper(item.Amount.Currency)
		grouped[code] = append(grouped[code], item.Amount.toLibraryMoney())
	}

	totals := make([]Total, 0, len(grouped))
	aggregator := money.NewAggregator(money.NewManager())

	for _, values := range grouped {
		sum, err := aggregator.Sum(values...)
		if err != nil {
			continue
		}

		amount, err := sum.Amount()
		if err != nil {
			continue
		}

		curr, err := sum.Currency()
		if err != nil {
			continue
		}

		totals = append(totals, Total{Amount: amount, Currency: curr.Code})
	}

	sort.Slice(totals, func(i, j int) bool {
		return totals[i].Currency < totals[j].Currency
	})

	b.Totals = totals
}

func billStateMachine() (*oworkflow.StateMachine[*Bill], error) {
	definition, err := oworkflow.NewDefinitionBuilder().
		AddPlace(StateOpen).
		AddPlace(StateClosed).
		SetInitialPlaces(StateOpen).
		AddTransition("close", []string{StateOpen}, []string{StateClosed}).
		Build()
	if err != nil {
		return nil, err
	}

	return oworkflow.NewStateMachine("bill", definition, &store.SingleState[*Bill]{
		Getter: (*Bill).GetState,
		Setter: (*Bill).SetState,
	}, nil)
}
