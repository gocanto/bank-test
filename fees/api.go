package fees

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"time"

	"encore.dev/beta/errs"
	"go.temporal.io/api/enums/v1"
	"go.temporal.io/api/serviceerror"
	"go.temporal.io/sdk/client"
	"go.temporal.io/sdk/temporal"
	"gocanto.sh/bank/fees/domain"
	billstore "gocanto.sh/bank/fees/storage/bills"
	"gocanto.sh/bank/fees/workflows"
)

type CreateBillRequest struct {
	BillID      string `json:"bill_id"`
	PeriodStart string `json:"period_start"`
	PeriodEnd   string `json:"period_end"`
}

type BillResponse struct {
	Bill domain.Bill `json:"bill"`
}

//encore:api public method=POST path=/v1/bank/bills

//encore:api public method=POST path=/v1/bank/bills/:billID/line-items

//encore:api public method=POST path=/v1/bank/bills/:billID/close

//encore:api public method=GET path=/v1/bank/bills/:billID

type apiErrorDetails struct {
	StatusCode int `json:"status_code"`
}

type apiErrorResponse struct {
	code       errs.ErrCode
	message    string
	statusCode int
}

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

	response := apiErrorResponseFor(err)

	return errs.B().
		Code(response.code).
		Cause(err).
		Msg(response.message).
		Details(apiErrorDetails{StatusCode: response.statusCode}).
		Err()
}

func (apiErrorDetails) ErrDetails() {}

func apiErrorResponseFor(err error) apiErrorResponse {
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
		return newAPIErrorResponse(errs.InvalidArgument, err.Error())
	case errors.Is(err, domain.ErrDuplicateLineItem):
		return newAPIErrorResponse(errs.AlreadyExists, err.Error())
	case errors.As(err, &alreadyStarted):
		return newAPIErrorResponse(errs.AlreadyExists, "bill already exists")
	case errors.As(err, &notFound):
		return newAPIErrorResponse(errs.NotFound, "bill not found")
	case errors.Is(err, domain.ErrBillClosed), errors.Is(err, domain.ErrBillAlreadyClosed):
		return newAPIErrorResponse(errs.FailedPrecondition, err.Error())
	default:
		return newAPIErrorResponse(errs.Unknown, err.Error())
	}
}

func newAPIErrorResponse(code errs.ErrCode, message string) apiErrorResponse {
	return apiErrorResponse{
		code:       code,
		message:    message,
		statusCode: apiHTTPStatus(code),
	}
}

func apiHTTPStatus(code errs.ErrCode) int {
	switch code {
	case errs.InvalidArgument, errs.FailedPrecondition:
		return http.StatusBadRequest
	case errs.AlreadyExists:
		return http.StatusConflict
	case errs.NotFound:
		return http.StatusNotFound
	default:
		return http.StatusInternalServerError
	}
}
