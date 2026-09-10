package apierrtest_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"doula-cloud/api/internal/apierr"
	"doula-cloud/api/internal/apierrtest"
)

// TestDecode_ReadsTheWholeEnvelope proves the helper returns all three of
// section 7's fields, not only the code -- the reason #488's call sites
// wanted the whole struct back rather than a string.
func TestDecode_ReadsTheWholeEnvelope(t *testing.T) {
	rec := httptest.NewRecorder()
	apierr.Write(rec, http.StatusConflict, apierr.CodeConflict, "already sent",
		map[string]string{"amountCents": "Enter an amount"})

	got := apierrtest.Decode(t, rec.Result())

	if got.Code != apierr.CodeConflict {
		t.Fatalf("code = %q, want %q", got.Code, apierr.CodeConflict)
	}
	if got.Message != "already sent" {
		t.Fatalf("message = %q, want %q", got.Message, "already sent")
	}
	if got.Details["amountCents"] != "Enter an amount" {
		t.Fatalf("details = %+v, want an amountCents entry", got.Details)
	}
}

// TestDecode_LeavesDetailsNilWhenAbsent covers the shape the large
// majority of refusals carry -- no details at all, which the omitempty
// tag keeps off the wire entirely.
func TestDecode_LeavesDetailsNilWhenAbsent(t *testing.T) {
	rec := httptest.NewRecorder()
	apierr.WriteError(rec, "engagement not found", http.StatusNotFound)

	got := apierrtest.Decode(t, rec.Result())

	if got.Code != apierr.CodeNotFound || got.Details != nil {
		t.Fatalf("envelope = %+v, want NOT_FOUND with no details", got)
	}
}
