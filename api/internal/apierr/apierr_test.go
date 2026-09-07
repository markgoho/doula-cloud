package apierr_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
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

// testPayloadName is the name value TestWriteJSON and TestDecodeJSON's
// round-trip bodies carry -- one literal shared across both so goconst
// does not flag it as three independent copies.
const testPayloadName = "offer"

func TestWriteJSON(t *testing.T) {
	rec := httptest.NewRecorder()
	type payload struct {
		Name string `json:"name"`
	}
	apierr.WriteJSON(rec, http.StatusCreated, payload{Name: testPayloadName})

	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusCreated)
	}
	if got := rec.Header().Get("Content-Type"); got != "application/json" {
		t.Fatalf("Content-Type = %q, want application/json", got)
	}
	var out payload
	if err := json.NewDecoder(rec.Body).Decode(&out); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	if out.Name != testPayloadName {
		t.Fatalf("body = %+v, want {%s}", out, testPayloadName)
	}
}

func TestDecodeJSON(t *testing.T) {
	type reqBody struct {
		Name string `json:"name"`
	}

	t.Run("valid body decodes", func(t *testing.T) {
		rec := httptest.NewRecorder()
		req := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/", strings.NewReader(`{"name":"`+testPayloadName+`"}`))

		var out reqBody
		if ok := apierr.DecodeJSON(rec, req, &out); !ok {
			t.Fatalf("DecodeJSON returned false, want true")
		}
		if out.Name != testPayloadName {
			t.Fatalf("decoded = %+v, want {%s}", out, testPayloadName)
		}
	})

	t.Run("malformed body writes 400 and returns false", func(t *testing.T) {
		rec := httptest.NewRecorder()
		req := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/", strings.NewReader(`{not json`))

		var out reqBody
		if ok := apierr.DecodeJSON(rec, req, &out); ok {
			t.Fatalf("DecodeJSON returned true, want false")
		}
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("status = %d, want %d", rec.Code, http.StatusBadRequest)
		}
		var out2 apierr.APIError
		if err := json.NewDecoder(rec.Body).Decode(&out2); err != nil {
			t.Fatalf("decode body: %v", err)
		}
		if out2.Code != string(apierr.CodeInvalidArgument) || out2.Message != "invalid request body" {
			t.Fatalf("body = %+v, want {INVALID_ARGUMENT invalid request body}", out2)
		}
	})

	t.Run("oversized body writes 413 payload-too-large and returns false", func(t *testing.T) {
		rec := httptest.NewRecorder()
		oversized := bytes.Repeat([]byte("a"), apierr.MaxRequestBodyBytes+1)
		body := append([]byte(`{"name":"`), append(oversized, []byte(`"}`)...)...)
		req := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/", bytes.NewReader(body))

		var out reqBody
		if ok := apierr.DecodeJSON(rec, req, &out); ok {
			t.Fatalf("DecodeJSON returned true, want false")
		}
		if rec.Code != http.StatusRequestEntityTooLarge {
			t.Fatalf("status = %d, want %d", rec.Code, http.StatusRequestEntityTooLarge)
		}
		var out2 apierr.APIError
		if err := json.NewDecoder(rec.Body).Decode(&out2); err != nil {
			t.Fatalf("decode body: %v", err)
		}
		if out2.Code != string(apierr.CodePayloadTooLarge) {
			t.Fatalf("code = %q, want %q", out2.Code, apierr.CodePayloadTooLarge)
		}
	})
}
