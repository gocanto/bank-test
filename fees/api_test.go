package fees

import (
	"errors"
	"net/http"
	"testing"

	"encore.dev/beta/errs"
	"go.temporal.io/api/serviceerror"
	"gocanto.sh/bank/fees/domain"
)

func TestAPIErrorResponseForUsesEncoreCodesAndHTTPStatusDetails(t *testing.T) {
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
			name:       "unknown",
			err:        errors.New("boom"),
			wantCode:   errs.Unknown,
			wantStatus: http.StatusInternalServerError,
			wantMsg:    "boom",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := apiErrorResponseFor(tt.err)

			if got.code != tt.wantCode {
				t.Fatalf("code = %v, want %v", got.code, tt.wantCode)
			}

			if got.statusCode != tt.wantStatus {
				t.Fatalf("statusCode = %d, want %d", got.statusCode, tt.wantStatus)
			}

			if got.message != tt.wantMsg {
				t.Fatalf("message = %q, want %q", got.message, tt.wantMsg)
			}
		})
	}
}

func TestAPIErrorDetailsExposeStatusCode(t *testing.T) {
	details := apiErrorDetails{StatusCode: http.StatusConflict}

	if details.StatusCode != http.StatusConflict {
		t.Fatalf("status code = %d, want %d", details.StatusCode, http.StatusConflict)
	}
}
