package fees

import (
	"context"
	"strings"
	"time"

	"go.temporal.io/api/enums/v1"
	"go.temporal.io/sdk/client"
	"gocanto.sh/bank/internal/database"
	"gocanto.sh/bank/internal/fees/billstore"
	"gocanto.sh/bank/internal/fees/domain"
	"gocanto.sh/bank/internal/fees/workflows"
	"gocanto.sh/bank/internal/response"
	"gocanto.sh/bank/internal/temporal"
)

//encore:api public method=POST path=/v1/bank/bills
func (s *Service) Create(ctx context.Context, req *domain.CreateBill) (*response.Response[domain.Bill], error) {
	if req == nil {
		return nil, fail(domain.ErrInvalidBillID)
	}

	billID := strings.TrimSpace(req.BillID)
	workflowID := workflows.WorkflowID(billID)
	initialBill, err := domain.NewBill(*req, time.Now())

	if err != nil {
		return nil, fail(err)
	}

	if _, err := s.client.ExecuteWorkflow(ctx, client.StartWorkflowOptions{
		ID:                       workflowID,
		TaskQueue:                taskQueue(),
		WorkflowIDReusePolicy:    enums.WORKFLOW_ID_REUSE_POLICY_REJECT_DUPLICATE,
		WorkflowExecutionTimeout: req.PeriodEnd.Sub(req.PeriodStart) + 24*time.Hour,
	}, workflows.WorkflowNameBill, *req); err != nil {
		return nil, fail(err)
	}

	// The workflow is the source of truth for the bill, including its creation
	// timestamp (set from workflow.Now). Read it back so the response and the
	// persisted snapshot match what later reads return. A query on a freshly
	// started workflow blocks only until its first task completes; fall back to
	// the locally built bill if it is not yet answerable.
	created := initialBill.Summary()

	if summary, ok := s.querySummary(ctx, workflowID); ok {
		created = summary
	}

	if err := database.Persist(ctx, s.store, created); err != nil {
		return nil, fail(err)
	}

	return response.Respond(created), nil
}

// querySummary returns the workflow's current bill summary. Unlike
// temporal.Query it never falls back to the workflow result, which would block
// until the bill's period ends, so it is safe to call on an open bill.
func (s *Service) querySummary(ctx context.Context, workflowID string) (domain.Bill, bool) {
	resp, err := s.client.QueryWorkflow(ctx, workflowID, "", workflows.QuerySummary)

	if err != nil {
		return domain.Bill{}, false
	}

	var summary domain.Bill

	if err := resp.Get(&summary); err != nil {
		return domain.Bill{}, false
	}

	return summary, true
}

//encore:api public method=POST path=/v1/bank/bills/:billID/line-items
func (s *Service) AddLineItem(ctx context.Context, billID string, req *domain.AddLineItem) (*response.Response[domain.Bill], error) {
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

	if err := database.Persist(ctx, s.store, bill); err != nil {
		return nil, fail(err)
	}

	return response.Respond(bill), nil
}

//encore:api public method=POST path=/v1/bank/bills/:billID/close
func (s *Service) Close(ctx context.Context, billID string) (*response.Response[domain.Bill], error) {
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

	if err := database.Persist(ctx, s.store, bill); err != nil {
		return nil, fail(err)
	}

	return response.Respond(bill), nil
}

//encore:api public method=GET path=/v1/bank/bills/:billID
func (s *Service) Get(ctx context.Context, billID string) (*response.Response[domain.Bill], error) {
	cached, cachedErr := database.Stored[domain.Bill](ctx, s.store, billID, billstore.ErrNotFound)

	if cachedErr == nil && cached.State == domain.StateClosed {
		return response.Respond(cached), nil
	}

	bill, err := temporal.Query[domain.Bill](ctx, s.client, workflows.WorkflowID(billID), workflows.QuerySummary)

	if err != nil {
		if cachedErr == nil {
			return response.Respond(cached), nil
		}

		return nil, fail(err)
	}

	if err := database.Persist(ctx, s.store, bill); err != nil {
		return nil, fail(err)
	}

	return response.Respond(bill), nil
}
