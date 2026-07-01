package domain

import (
	"errors"
	"testing"
	"time"
)

func testPeriod() (time.Time, time.Time) {
	start := time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC)

	return start, start.AddDate(0, 1, 0)
}

func TestBillHappyPaths(t *testing.T) {
	start, end := testPeriod()
	bill, err := NewBill(CreateBill{BillID: "bill-1", PeriodStart: start, PeriodEnd: end}, start)

	if err != nil {
		t.Fatalf("new bill: %v", err)
	}

	if bill.State != StateOpen {
		t.Fatalf("state = %q, want %q", bill.State, StateOpen)
	}

	usd, err := NewMoney(1250, "usd")

	if err != nil {
		t.Fatalf("new usd money: %v", err)
	}

	gel, err := NewMoney(700, "GEL")

	if err != nil {
		t.Fatalf("new gel money: %v", err)
	}

	if _, err := bill.AddLineItem(AddLineItem{ID: "li-1", Description: "USD fee", Amount: usd}, start); err != nil {
		t.Fatalf("add usd item: %v", err)
	}

	if _, err := bill.AddLineItem(AddLineItem{ID: "li-2", Description: "GEL fee", Amount: gel}, start); err != nil {
		t.Fatalf("add gel item: %v", err)
	}

	if len(bill.Totals) != 2 {
		t.Fatalf("totals len = %d, want 2", len(bill.Totals))
	}

	if _, err := bill.Close(end); err != nil {
		t.Fatalf("close bill: %v", err)
	}

	if bill.State != StateClosed {
		t.Fatalf("state = %q, want %q", bill.State, StateClosed)
	}

	if bill.ClosedAt == nil {
		t.Fatal("expected closed_at")
	}
}

func TestBillUnhappyPaths(t *testing.T) {
	start, end := testPeriod()

	tests := []struct {
		name string
		run  func() error
		want error
	}{
		{
			name: "invalid bill id",
			run: func() error {
				_, err := NewBill(CreateBill{PeriodStart: start, PeriodEnd: end}, start)

				return err
			},
			want: ErrInvalidBillID,
		},
		{
			name: "invalid period",
			run: func() error {
				_, err := NewBill(CreateBill{BillID: "bill", PeriodStart: end, PeriodEnd: start}, start)

				return err
			},
			want: ErrInvalidPeriod,
		},
		{
			name: "invalid currency",
			run: func() error {
				_, err := NewMoney(100, "EUR")

				return err
			},
			want: ErrInvalidCurrency,
		},
		{
			name: "invalid amount",
			run: func() error {
				_, err := NewMoney(0, "USD")

				return err
			},
			want: ErrInvalidAmount,
		},
		{
			name: "duplicate line item",
			run: func() error {
				bill, _ := NewBill(CreateBill{BillID: "bill", PeriodStart: start, PeriodEnd: end}, start)
				amount, _ := NewMoney(100, "USD")
				_, _ = bill.AddLineItem(AddLineItem{ID: "li", Description: "fee", Amount: amount}, start)
				_, err := bill.AddLineItem(AddLineItem{ID: "li", Description: "fee", Amount: amount}, start)

				return err
			},
			want: ErrDuplicateLineItem,
		},
		{
			name: "add after close",
			run: func() error {
				bill, _ := NewBill(CreateBill{BillID: "bill", PeriodStart: start, PeriodEnd: end}, start)
				_, _ = bill.Close(end)
				amount, _ := NewMoney(100, "USD")
				_, err := bill.AddLineItem(AddLineItem{ID: "li", Description: "fee", Amount: amount}, start)

				return err
			},
			want: ErrBillClosed,
		},
		{
			name: "close twice",
			run: func() error {
				bill, _ := NewBill(CreateBill{BillID: "bill", PeriodStart: start, PeriodEnd: end}, start)
				_, _ = bill.Close(end)
				_, err := bill.Close(end)

				return err
			},
			want: ErrBillAlreadyClosed,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := tt.run(); !errors.Is(err, tt.want) {
				t.Fatalf("error = %v, want %v", err, tt.want)
			}
		})
	}
}
