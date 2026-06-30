package workflows

import (
	"time"

	"encore.app/fees/domain"
	"go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/workflow"
)

const (
	QuerySummary       = "summary"
	UpdateAddLineItem  = "add_line_item"
	UpdateCloseBill    = "close_bill"
	DefaultTaskQueue   = "pavebank-fees"
	WorkflowNameBill   = "Bill"
	workflowUpdateNote = "updated by workflow"
)

func Bill(ctx workflow.Context, req domain.CreateBill) (domain.Bill, error) {
	now := workflow.Now(ctx)
	bill, err := domain.NewBill(req, now)

	if err != nil {
		return domain.Bill{}, temporal.NewNonRetryableApplicationError(err.Error(), "VALIDATION", err)
	}

	if err := workflow.SetQueryHandler(ctx, QuerySummary, func() (domain.Bill, error) {
		return bill.Summary(), nil
	}); err != nil {
		return domain.Bill{}, err
	}

	if err := workflow.SetUpdateHandler(ctx, UpdateAddLineItem, func(ctx workflow.Context, req domain.AddLineItem) (domain.Bill, error) {
		updated, err := bill.AddLineItem(req, workflow.Now(ctx))

		if err != nil {
			return domain.Bill{}, temporal.NewNonRetryableApplicationError(err.Error(), "VALIDATION", err)
		}

		bill = updated

		return bill.Summary(), nil
	}); err != nil {
		return domain.Bill{}, err
	}

	if err := workflow.SetUpdateHandler(ctx, UpdateCloseBill, func(ctx workflow.Context) (domain.Bill, error) {
		updated, err := bill.Close(workflow.Now(ctx))

		if err != nil {
			return domain.Bill{}, temporal.NewNonRetryableApplicationError(err.Error(), "VALIDATION", err)
		}

		bill = updated

		return bill.Summary(), nil
	}); err != nil {
		return domain.Bill{}, err
	}

	sleepFor := req.PeriodEnd.Sub(now)

	if sleepFor > 0 {
		if err := workflow.Sleep(ctx, sleepFor); err != nil {
			return domain.Bill{}, err
		}
	}

	if bill.State != domain.StateClosed {
		if _, err := bill.Close(workflow.Now(ctx)); err != nil {
			return domain.Bill{}, temporal.NewNonRetryableApplicationError(err.Error(), "VALIDATION", err)
		}
	}

	return bill.Summary(), nil
}

func WorkflowID(billID string) string {
	return "bill:" + billID
}

func ShortPeriod(start time.Time) domain.CreateBill {
	return domain.CreateBill{
		BillID:      "bill-short",
		PeriodStart: start,
		PeriodEnd:   start.Add(time.Second),
	}
}
