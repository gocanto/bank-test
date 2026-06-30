package fees

import (
	"context"
	"errors"
	"strings"
	"time"

	"encore.app/fees/domain"
	billstore "encore.app/fees/storage/bills"
	"encore.app/fees/workflows"
	"encore.dev/beta/errs"
	"go.temporal.io/api/enums/v1"
	"go.temporal.io/api/serviceerror"
	"go.temporal.io/sdk/client"
	"go.temporal.io/sdk/temporal"
)

type CreateBillRequest struct {
	BillID      string `json:"bill_id"`
	PeriodStart string `json:"period_start"`
	PeriodEnd   string `json:"period_end"`
}

type BillResponse struct {
	Bill domain.Bill `json:"bill"`
}

//encore:api public method=POST path=/bills
func (s *Service) CreateBill(ctx context.Context, req *domain.CreateBill) (*BillResponse, error) {
	if req == nil {
		return nil, apiError(domain.ErrInvalidBillID)
	}

	billID := strings.TrimSpace(req.BillID)
	workflowID := workflows.WorkflowID(billID)
	initialBill, err := domain.NewBill(*req, time.Now())

	if err != nil {
		return nil, apiError(err)
	}

	run, err := s.client.ExecuteWorkflow(ctx, client.StartWorkflowOptions{
		ID:                       workflowID,
		TaskQueue:                taskQueue(),
		WorkflowIDReusePolicy:    enums.WORKFLOW_ID_REUSE_POLICY_REJECT_DUPLICATE,
		WorkflowExecutionTimeout: req.PeriodEnd.Sub(req.PeriodStart) + 24*time.Hour,
	}, workflows.WorkflowNameBill, *req)

	if err != nil {
		return nil, apiError(err)
	}

	_ = run

	if err := s.persistBill(ctx, initialBill.Summary()); err != nil {
		return nil, apiError(err)
	}

	return &BillResponse{Bill: initialBill.Summary()}, nil
}

//encore:api public method=POST path=/bills/:billID/line-items
func (s *Service) AddLineItem(ctx context.Context, billID string, req *domain.AddLineItem) (*BillResponse, error) {
	if req == nil {
		return nil, apiError(domain.ErrInvalidLineItemID)
	}

	handle, err := s.client.UpdateWorkflow(ctx, client.UpdateWorkflowOptions{
		WorkflowID:   workflows.WorkflowID(billID),
		UpdateName:   workflows.UpdateAddLineItem,
		Args:         []any{*req},
		WaitForStage: client.WorkflowUpdateStageCompleted,
	})

	if err != nil {
		return nil, apiError(err)
	}

	var bill domain.Bill

	if err := handle.Get(ctx, &bill); err != nil {
		return nil, apiError(err)
	}

	if err := s.persistBill(ctx, bill); err != nil {
		return nil, apiError(err)
	}

	return &BillResponse{Bill: bill}, nil
}

//encore:api public method=POST path=/bills/:billID/close
func (s *Service) CloseBill(ctx context.Context, billID string) (*BillResponse, error) {
	handle, err := s.client.UpdateWorkflow(ctx, client.UpdateWorkflowOptions{
		WorkflowID:   workflows.WorkflowID(billID),
		UpdateName:   workflows.UpdateCloseBill,
		WaitForStage: client.WorkflowUpdateStageCompleted,
	})

	if err != nil {
		return nil, apiError(err)
	}

	var bill domain.Bill

	if err := handle.Get(ctx, &bill); err != nil {
		return nil, apiError(err)
	}

	if err := s.persistBill(ctx, bill); err != nil {
		return nil, apiError(err)
	}

	return &BillResponse{Bill: bill}, nil
}

//encore:api public method=GET path=/bills/:billID
func (s *Service) GetBill(ctx context.Context, billID string) (*BillResponse, error) {
	stored, storedErr := s.storedBill(ctx, billID)

	if storedErr == nil && stored.State == domain.StateClosed {
		return &BillResponse{Bill: stored}, nil
	}

	bill, err := s.queryBill(ctx, billID)

	if err != nil {
		if storedErr == nil {
			return &BillResponse{Bill: stored}, nil
		}

		return nil, apiError(err)
	}

	if err := s.persistBill(ctx, bill); err != nil {
		return nil, apiError(err)
	}

	return &BillResponse{Bill: bill}, nil
}

func (s *Service) persistBill(ctx context.Context, bill domain.Bill) error {
	if s == nil || s.store == nil {
		return nil
	}

	return s.store.Save(ctx, bill)
}

func (s *Service) storedBill(ctx context.Context, billID string) (domain.Bill, error) {
	if s == nil || s.store == nil {
		return domain.Bill{}, billstore.ErrNotFound
	}

	bill, err := s.store.Find(ctx, billID)

	if errors.Is(err, billstore.ErrNotFound) {
		return domain.Bill{}, err
	}

	return bill, err
}

func (s *Service) queryBill(ctx context.Context, billID string) (domain.Bill, error) {
	response, err := s.client.QueryWorkflow(ctx, workflows.WorkflowID(billID), "", workflows.QuerySummary)

	if err != nil {
		var result domain.Bill
		run := s.client.GetWorkflow(ctx, workflows.WorkflowID(billID), "")

		if getErr := run.Get(ctx, &result); getErr == nil {
			return result, nil
		}

		return domain.Bill{}, err
	}

	var bill domain.Bill

	if err := response.Get(&bill); err != nil {
		return domain.Bill{}, err
	}

	return bill, nil
}

func apiError(err error) error {
	if err == nil {
		return nil
	}

	var applicationErr *temporal.ApplicationError

	if errors.As(err, &applicationErr) && applicationErr.Unwrap() != nil {
		err = applicationErr.Unwrap()
	}

	var notFound *serviceerror.NotFound

	var alreadyStarted *serviceerror.WorkflowExecutionAlreadyStarted

	switch {
	case errors.Is(err, domain.ErrInvalidBillID),
		errors.Is(err, domain.ErrInvalidLineItemID),
		errors.Is(err, domain.ErrInvalidDescription),
		errors.Is(err, domain.ErrInvalidPeriod),
		errors.Is(err, domain.ErrInvalidAmount),
		errors.Is(err, domain.ErrInvalidCurrency):
		return errs.B().Code(errs.InvalidArgument).Cause(err).Msg(err.Error()).Err()
	case errors.Is(err, domain.ErrDuplicateLineItem):
		return errs.B().Code(errs.AlreadyExists).Cause(err).Msg(err.Error()).Err()
	case errors.As(err, &alreadyStarted):
		return errs.B().Code(errs.AlreadyExists).Cause(err).Msg("bill already exists").Err()
	case errors.As(err, &notFound):
		return errs.B().Code(errs.NotFound).Cause(err).Msg("bill not found").Err()
	case errors.Is(err, domain.ErrBillClosed), errors.Is(err, domain.ErrBillAlreadyClosed):
		return errs.B().Code(errs.FailedPrecondition).Cause(err).Msg(err.Error()).Err()
	default:
		return errs.B().Code(errs.Unknown).Cause(err).Msg(err.Error()).Err()
	}
}
