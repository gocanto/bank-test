package workflows

import (
	"testing"
	"time"

	"go.temporal.io/sdk/testsuite"
	"gocanto.sh/bank/internal/fees/domain"
)

func TestBillWorkflow_HappyPathManualClose(t *testing.T) {
	var suite testsuite.WorkflowTestSuite
	env := suite.NewTestWorkflowEnvironment()
	env.RegisterWorkflow(Bill)

	start := time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC)
	env.SetStartTime(start)

	end := start.AddDate(0, 1, 0)
	req := domain.CreateBill{BillID: "bill-1", PeriodStart: start, PeriodEnd: end}

	env.RegisterDelayedCallback(func() {
		amount, err := domain.NewMoney(1500, "USD")

		if err != nil {
			t.Fatalf("new money: %v", err)
		}

		env.UpdateWorkflow(UpdateAddLineItem, "", &testsuite.TestUpdateCallback{
			OnReject: func(err error) {
				t.Fatalf("add update rejected: %v", err)
			},
			OnComplete: func(success interface{}, err error) {
				if err != nil {
					t.Fatalf("add update failed: %v", err)
				}

				summary, ok := success.(domain.Bill)

				if !ok {
					t.Fatalf("add update result = %T, want domain.Bill", success)
				}

				if len(summary.LineItems) != 1 {
					t.Fatalf("line items = %d, want 1", len(summary.LineItems))
				}
			},
		}, domain.AddLineItem{
			ID:          "li-1",
			Description: "Card fee",
			Amount:      amount,
		})
	}, time.Second)

	env.RegisterDelayedCallback(func() {
		env.UpdateWorkflow(UpdateCloseBill, "", &testsuite.TestUpdateCallback{
			OnReject: func(err error) {
				t.Fatalf("close update rejected: %v", err)
			},
			OnComplete: func(success interface{}, err error) {
				if err != nil {
					t.Fatalf("close update failed: %v", err)
				}

				summary, ok := success.(domain.Bill)

				if !ok {
					t.Fatalf("close update result = %T, want domain.Bill", success)
				}

				if summary.State != domain.StateClosed {
					t.Fatalf("state = %q, want closed", summary.State)
				}
			},
		})
	}, 2*time.Second)

	env.ExecuteWorkflow(Bill, req)

	if !env.IsWorkflowCompleted() {
		t.Fatal("workflow did not complete")
	}

	if err := env.GetWorkflowError(); err != nil {
		t.Fatalf("workflow error: %v", err)
	}

	var result domain.Bill

	if err := env.GetWorkflowResult(&result); err != nil {
		t.Fatalf("workflow result: %v", err)
	}

	if result.State != domain.StateClosed {
		t.Fatalf("state = %q, want closed", result.State)
	}

	if len(result.LineItems) != 1 {
		t.Fatalf("line items = %d, want 1", len(result.LineItems))
	}
}

func TestBillWorkflow_AutoClose(t *testing.T) {
	var suite testsuite.WorkflowTestSuite
	env := suite.NewTestWorkflowEnvironment()
	env.RegisterWorkflow(Bill)

	start := time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC)
	env.SetStartTime(start)

	req := ShortPeriod(start)

	env.ExecuteWorkflow(Bill, req)

	if err := env.GetWorkflowError(); err != nil {
		t.Fatalf("workflow error: %v", err)
	}

	var result domain.Bill

	if err := env.GetWorkflowResult(&result); err != nil {
		t.Fatalf("workflow result: %v", err)
	}

	if result.State != domain.StateClosed {
		t.Fatalf("state = %q, want closed", result.State)
	}
}

func TestBillWorkflow_RejectsDuplicateLineItem(t *testing.T) {
	var suite testsuite.WorkflowTestSuite
	env := suite.NewTestWorkflowEnvironment()
	env.RegisterWorkflow(Bill)

	start := time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC)
	env.SetStartTime(start)

	req := domain.CreateBill{BillID: "bill-1", PeriodStart: start, PeriodEnd: start.AddDate(0, 1, 0)}
	amount, _ := domain.NewMoney(1500, "USD")

	env.RegisterDelayedCallback(func() {
		env.UpdateWorkflow(UpdateAddLineItem, "", &testsuite.TestUpdateCallback{
			OnReject: func(err error) {
				t.Fatalf("first update rejected: %v", err)
			},
			OnComplete: func(success interface{}, err error) {
				if err != nil {
					t.Fatalf("first update failed: %v", err)
				}
			},
		}, domain.AddLineItem{ID: "li-1", Description: "fee", Amount: amount})
	}, time.Second)

	env.RegisterDelayedCallback(func() {
		env.UpdateWorkflow(UpdateAddLineItem, "", &testsuite.TestUpdateCallback{
			OnReject: func(error) {},
			OnAccept: func() {},
			OnComplete: func(success interface{}, err error) {
				if err == nil {
					t.Fatalf("expected duplicate update to fail")
				}
			},
		}, domain.AddLineItem{ID: "li-1", Description: "fee", Amount: amount})
	}, 2*time.Second)

	env.ExecuteWorkflow(Bill, req)
}
