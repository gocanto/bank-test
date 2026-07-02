package errors

import (
	"net/http"
	"testing"

	"encore.dev/beta/errs"
)

func TestDescribeMapsCodeToHTTPStatus(t *testing.T) {
	tests := []struct {
		name       string
		code       errs.ErrCode
		wantStatus int
	}{
		{name: "invalid argument", code: errs.InvalidArgument, wantStatus: http.StatusBadRequest},
		{name: "failed precondition", code: errs.FailedPrecondition, wantStatus: http.StatusBadRequest},
		{name: "already exists", code: errs.AlreadyExists, wantStatus: http.StatusConflict},
		{name: "not found", code: errs.NotFound, wantStatus: http.StatusNotFound},
		{name: "unknown", code: errs.Unknown, wantStatus: http.StatusInternalServerError},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Describe(tt.code, "message")

			if got.Code != tt.code {
				t.Fatalf("code = %v, want %v", got.Code, tt.code)
			}

			if got.Message != "message" {
				t.Fatalf("message = %q, want %q", got.Message, "message")
			}

			if got.StatusCode != tt.wantStatus {
				t.Fatalf("statusCode = %d, want %d", got.StatusCode, tt.wantStatus)
			}
		})
	}
}

func TestFailReturnsNilForNilError(t *testing.T) {
	// Fail short-circuits on a nil error before touching errs.B (which requires
	// the Encore runtime), so this is safe under plain `go test`.
	if err := Fail(nil, Describe(errs.Unknown, "boom")); err != nil {
		t.Fatalf("Fail(nil) = %v, want nil", err)
	}
}

func TestDetailsExposeStatusCode(t *testing.T) {
	got := Details{StatusCode: http.StatusConflict}

	if got.StatusCode != http.StatusConflict {
		t.Fatalf("status code = %d, want %d", got.StatusCode, http.StatusConflict)
	}
}
