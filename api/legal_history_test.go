package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"doula-cloud/api/internal/legal"
	"doula-cloud/api/internal/legalnotice"
	"doula-cloud/api/internal/testdb"
)

// TestLegalHistory_PublishReadsDepsHistory is #1833's wiring: the
// publish route reads Deps.LegalDocuments when main sets it (the
// end-to-end stack's history, with a version the shipped one lacks),
// and the shipped history when it does not (Cloud Run).
func TestLegalHistory_PublishReadsDepsHistory(t *testing.T) {
	now := time.Date(2026, time.October, 24, 12, 0, 0, 0, time.UTC)
	terms := legal.Terms
	terms.Versions = append(append([]legal.Version(nil), legal.Terms.Versions...),
		legal.Version{Effective: "2026-12-01", Material: true, Change: "Says what a test changed."})

	publish := func(t *testing.T, history []legal.Document) int {
		t.Helper()
		deps := testDeps()
		deps.DB = testdb.New(t).App
		deps.Now = func() time.Time { return now }
		deps.LegalDocuments = history
		mux, _, _ := routes(deps)
		srv := httptest.NewServer(mux)
		defer srv.Close()

		body, err := json.Marshal(legalnotice.PublishRequest{Document: "terms", Version: "2026-12-01", PublishedBy: "Mark"})
		if err != nil {
			t.Fatalf("encode: %v", err)
		}
		req, err := http.NewRequestWithContext(t.Context(), http.MethodPost, srv.URL+"/api/internal/legal/publish", bytes.NewReader(body))
		if err != nil {
			t.Fatalf("build request: %v", err)
		}
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-Internal-Secret", testWorkerSecret)
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatalf("request: %v", err)
		}
		defer resp.Body.Close()
		return resp.StatusCode
	}

	if status := publish(t, []legal.Document{terms, legal.Privacy}); status != http.StatusOK {
		t.Fatalf("publish of the appended version = %d, want %d", status, http.StatusOK)
	}
	if status := publish(t, nil); status != http.StatusBadRequest {
		t.Fatalf("publish of a version the shipped history lacks = %d, want %d", status, http.StatusBadRequest)
	}
}
