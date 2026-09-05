package apierr_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"doula-cloud/api/internal/apierr"
)

func TestCodeForStatus(t *testing.T) {
	tests := []struct {
		status int
		want   apierr.Code
	}{
		{http.StatusBadRequest, apierr.CodeInvalidArgument},
		{http.StatusUnprocessableEntity, apierr.CodeInvalidArgument},
		{http.StatusUnauthorized, apierr.CodeUnauthorized},
		{http.StatusForbidden, apierr.CodeForbidden},
		{http.StatusNotFound, apierr.CodeNotFound},
		{http.StatusPaymentRequired, apierr.CodePaymentRequired},
		{http.StatusConflict, apierr.CodeConflict},
		{http.StatusRequestEntityTooLarge, apierr.CodePayloadTooLarge},
		{http.StatusTooManyRequests, apierr.CodeRateLimited},
		{http.StatusInternalServerError, apierr.CodeInternal},
		{http.StatusTeapot, apierr.CodeInternal},
	}
	for _, tt := range tests {
		if got := apierr.CodeForStatus(tt.status); got != tt.want {
			t.Errorf("CodeForStatus(%d) = %q, want %q", tt.status, got, tt.want)
		}
	}
}

func TestWrite(t *testing.T) {
	t.Run("without details", func(t *testing.T) {
		rec := httptest.NewRecorder()
		apierr.Write(rec, http.StatusConflict, apierr.CodeFailedPrecondition, "website required", nil)

		if rec.Code != http.StatusConflict {
			t.Fatalf("status = %d, want %d", rec.Code, http.StatusConflict)
		}
		if got := rec.Header().Get("Content-Type"); got != "application/json" {
			t.Fatalf("Content-Type = %q, want application/json", got)
		}
		var out apierr.APIError
		if err := json.NewDecoder(rec.Body).Decode(&out); err != nil {
			t.Fatalf("decode body: %v", err)
		}
		if out.Code != string(apierr.CodeFailedPrecondition) || out.Message != "website required" || out.Details != nil {
			t.Fatalf("body = %+v, want {FAILED_PRECONDITION website required <nil>}", out)
		}
	})

	t.Run("with details", func(t *testing.T) {
		rec := httptest.NewRecorder()
		apierr.Write(rec, http.StatusBadRequest, apierr.CodeInvalidArgument, "invalid request body",
			map[string]string{"ownUrl": "Enter a web address in the correct format"})

		var out apierr.APIError
		if err := json.NewDecoder(rec.Body).Decode(&out); err != nil {
			t.Fatalf("decode body: %v", err)
		}
		if out.Details["ownUrl"] != "Enter a web address in the correct format" {
			t.Fatalf("details = %+v, want ownUrl entry", out.Details)
		}
	})
}

func TestWriteError(t *testing.T) {
	rec := httptest.NewRecorder()
	apierr.WriteError(rec, "engagement not found", http.StatusNotFound)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusNotFound)
	}
	var out apierr.APIError
	if err := json.NewDecoder(rec.Body).Decode(&out); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	if out.Code != string(apierr.CodeNotFound) || out.Message != "engagement not found" {
		t.Fatalf("body = %+v, want {NOT_FOUND engagement not found}", out)
	}
}
