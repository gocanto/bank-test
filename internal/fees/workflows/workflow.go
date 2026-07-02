package workflows

import (
	"time"

	"go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/workflow"
	"gocanto.sh/bank/internal/fees/domain"
)

const (
	QuerySummary      = "summary"
	UpdateAddLineItem = "add_line_item"
	UpdateCloseBill   = "close_bill"
	DefaultTaskQueue  = "gocanto-fees"
	WorkflowNameBill  = "Bill"
)

func Bill(ctx workflow.Context, req domain.CreateBill) (domain.Bill, error) {
	now := workflow.Now(ctx)
	bill, err := domain.NewBill(req, now)

	if err != nil {
		return domain.Bill{}, appError(err)
	}

	if err := workflow.SetQueryHandler(ctx, QuerySummary, func() (domain.Bill, error) {
		return bill.Summary(), nil
	}); err != nil {
		return domain.Bill{}, err
	}

	if err := workflow.SetUpdateHandlerWithOptions(ctx, UpdateAddLineItem, func(ctx workflow.Context, req domain.AddLineItem) (domain.Bill, error) {
		updated, err := bill.AddLineItem(req, workflow.Now(ctx))

		if err != nil {
			return domain.Bill{}, appError(err)
		}

		bill = updated

		return bill.Summary(), nil
	}, workflow.UpdateHandlerOptions{
		Validator: func(ctx workflow.Context, req domain.AddLineItem) error {
			if err := bill.ValidateAddLineItem(req); err != nil {
				return appError(err)
			}

			return nil
		},
	}); err != nil {
		return domain.Bill{}, err
	}

	if err := workflow.SetUpdateHandlerWithOptions(ctx, UpdateCloseBill, func(ctx workflow.Context) (domain.Bill, error) {
		updated, err := bill.Close(workflow.Now(ctx))

		if err != nil {
			return domain.Bill{}, appError(err)
		}

		bill = updated

		return bill.Summary(), nil
	}, workflow.UpdateHandlerOptions{
		Validator: func(ctx workflow.Context) error {
			if err := bill.ValidateClose(); err != nil {
				return appError(err)
			}

			return nil
		},
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
			return domain.Bill{}, appError(err)
		}
	}

	return bill.Summary(), nil
}

// appError wraps a domain error into a non-retryable Temporal application error
// whose Type carries the domain classification code. The code, unlike the
// wrapped Go error, survives the gRPC boundary so the API layer can map the
// failure onto the correct HTTP status.
func appError(err error) error {
	return temporal.NewNonRetryableApplicationError(err.Error(), domain.ErrorCode(err), err)
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
