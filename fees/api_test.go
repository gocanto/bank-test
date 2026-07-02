package fees

import (
	"errors"
	"net/http"
	"testing"

	"encore.dev/beta/errs"
	"go.temporal.io/api/serviceerror"
	"go.temporal.io/sdk/temporal"
	"gocanto.sh/bank/internal/fees/domain"
)

func TestClassifyUsesEncoreCodesAndHTTPStatusDetails(t *testing.T) {
	tests := []struct {
		name       string
		err        error
		wantCode   errs.ErrCode
		wantStatus int
		wantMsg    string
	}{
		{
			name:       "invalid bill",
			err:        domain.ErrInvalidBillID,
			wantCode:   errs.InvalidArgument,
			wantStatus: http.StatusBadRequest,
			wantMsg:    domain.ErrInvalidBillID.Error(),
		},
		{
			name:       "duplicate line item",
			err:        domain.ErrDuplicateLineItem,
			wantCode:   errs.AlreadyExists,
			wantStatus: http.StatusConflict,
			wantMsg:    domain.ErrDuplicateLineItem.Error(),
		},
		{
			name:       "workflow already started",
			err:        serviceerror.NewWorkflowExecutionAlreadyStarted("duplicate", "request-id", "run-id"),
			wantCode:   errs.AlreadyExists,
			wantStatus: http.StatusConflict,
			wantMsg:    "bill already exists",
		},
		{
			name:       "workflow not found",
			err:        serviceerror.NewNotFound("missing"),
			wantCode:   errs.NotFound,
			wantStatus: http.StatusNotFound,
			wantMsg:    "bill not found",
		},
		{
			name:       "closed bill",
			err:        domain.ErrBillClosed,
			wantCode:   errs.FailedPrecondition,
			wantStatus: http.StatusBadRequest,
			wantMsg:    domain.ErrBillClosed.Error(),
		},
		{
			// Simulates the gRPC boundary: a workflow update failure arrives as
			// an ApplicationError carrying only the code and message, with no
			// unwrappable domain error. Classification must rely on the code.
			name:       "closed bill over rpc",
			err:        temporal.NewNonRetryableApplicationError(domain.ErrBillClosed.Error(), domain.CodeBillClosed, nil),
			wantCode:   errs.FailedPrecondition,
			wantStatus: http.StatusBadRequest,
			wantMsg:    domain.ErrBillClosed.Error(),
		},
		{
			name:       "duplicate line item over rpc",
			err:        temporal.NewNonRetryableApplicationError(domain.ErrDuplicateLineItem.Error(), domain.CodeDuplicateLineItem, nil),
			wantCode:   errs.AlreadyExists,
			wantStatus: http.StatusConflict,
			wantMsg:    domain.ErrDuplicateLineItem.Error(),
		},
		{
			name:       "validation over rpc",
			err:        temporal.NewNonRetryableApplicationError(domain.ErrInvalidCurrency.Error(), domain.CodeValidation, nil),
			wantCode:   errs.InvalidArgument,
			wantStatus: http.StatusBadRequest,
			wantMsg:    domain.ErrInvalidCurrency.Error(),
		},
		{
			name:       "internal over rpc",
			err:        temporal.NewNonRetryableApplicationError("boom", domain.CodeInternal, nil),
			wantCode:   errs.Unknown,
			wantStatus: http.StatusInternalServerError,
			wantMsg:    "boom",
		},
		{
			name:       "unknown",
			err:        errors.New("boom"),
			wantCode:   errs.Unknown,
			wantStatus: http.StatusInternalServerError,
			wantMsg:    "boom",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := classify(tt.err)

			if got.Code != tt.wantCode {
				t.Fatalf("code = %v, want %v", got.Code, tt.wantCode)
			}

			if got.StatusCode != tt.wantStatus {
				t.Fatalf("statusCode = %d, want %d", got.StatusCode, tt.wantStatus)
			}

			if got.Message != tt.wantMsg {
				t.Fatalf("message = %q, want %q", got.Message, tt.wantMsg)
			}
		})
	}
}
