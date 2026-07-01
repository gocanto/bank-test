package fees

import (
	"context"
	"strings"
	"time"

	"go.temporal.io/api/enums/v1"
	"go.temporal.io/sdk/client"
	"gocanto.sh/bank/fees/domain"
	billstore "gocanto.sh/bank/fees/storage/bills"
	"gocanto.sh/bank/fees/workflows"
)

//encore:api public method=POST path=/v1/bank/bills
func (s *Service) Create(ctx context.Context, req *domain.CreateBill) (*Response[domain.Bill], error) {
	if req == nil {
		return nil, fail(domain.ErrInvalidBillID)
	}

	billID := strings.TrimSpace(req.BillID)
	workflowID := workflows.WorkflowID(billID)
	initialBill, err := domain.NewBill(*req, time.Now())

	if err != nil {
		return nil, fail(err)
	}

	run, err := s.client.ExecuteWorkflow(ctx, client.StartWorkflowOptions{
		ID:                       workflowID,
		TaskQueue:                taskQueue(),
		WorkflowIDReusePolicy:    enums.WORKFLOW_ID_REUSE_POLICY_REJECT_DUPLICATE,
		WorkflowExecutionTimeout: req.PeriodEnd.Sub(req.PeriodStart) + 24*time.Hour,
	}, workflows.WorkflowNameBill, *req)

	if err != nil {
		return nil, fail(err)
	}

	_ = run

	if err := persist(ctx, s.store, initialBill.Summary()); err != nil {
		return nil, fail(err)
	}

	return respond(initialBill.Summary()), nil
}

//encore:api public method=POST path=/v1/bank/bills/:billID/line-items
func (s *Service) AddLineItem(ctx context.Context, billID string, req *domain.AddLineItem) (*Response[domain.Bill], error) {
	if req == nil {
		return nil, fail(domain.ErrInvalidLineItemID)
	}

	handle, err := s.client.UpdateWorkflow(ctx, client.UpdateWorkflowOptions{
		WorkflowID:   workflows.WorkflowID(billID),
		UpdateName:   workflows.UpdateAddLineItem,
		Args:         []any{*req},
		WaitForStage: client.WorkflowUpdateStageCompleted,
	})

	if err != nil {
		return nil, fail(err)
	}

	var bill domain.Bill

	if err := handle.Get(ctx, &bill); err != nil {
		return nil, fail(err)
	}

	if err := persist(ctx, s.store, bill); err != nil {
		return nil, fail(err)
	}

	return respond(bill), nil
}

//encore:api public method=POST path=/v1/bank/bills/:billID/close
func (s *Service) Close(ctx context.Context, billID string) (*Response[domain.Bill], error) {
	handle, err := s.client.UpdateWorkflow(ctx, client.UpdateWorkflowOptions{
		WorkflowID:   workflows.WorkflowID(billID),
		UpdateName:   workflows.UpdateCloseBill,
		WaitForStage: client.WorkflowUpdateStageCompleted,
	})

	if err != nil {
		return nil, fail(err)
	}

	var bill domain.Bill

	if err := handle.Get(ctx, &bill); err != nil {
		return nil, fail(err)
	}

	if err := persist(ctx, s.store, bill); err != nil {
		return nil, fail(err)
	}

	return respond(bill), nil
}

//encore:api public method=GET path=/v1/bank/bills/:billID
func (s *Service) Get(ctx context.Context, billID string) (*Response[domain.Bill], error) {
	cached, cachedErr := stored[domain.Bill](ctx, s.store, billID, billstore.ErrNotFound)

	if cachedErr == nil && cached.State == domain.StateClosed {
		return respond(cached), nil
	}

	bill, err := query[domain.Bill](ctx, s.client, workflows.WorkflowID(billID), workflows.QuerySummary)

	if err != nil {
		if cachedErr == nil {
			return respond(cached), nil
		}

		return nil, fail(err)
	}

	if err := persist(ctx, s.store, bill); err != nil {
		return nil, fail(err)
	}

	return respond(bill), nil
}
