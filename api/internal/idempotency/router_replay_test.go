package idempotency_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"doula-cloud/api/internal/authntest"
	"doula-cloud/api/internal/idempotency"
	"doula-cloud/api/internal/staffauth"
	"doula-cloud/api/internal/testdb"
)

// TestRouter_ReplayableAppliesMiddlewareAndWrapItself is #836's own
// package test for the module actually being deepened here: a feature
// Mount no longer assembles staffauth.Middleware(db)(idempotency.Wrap(h))
// by hand -- Router.Replayable does both itself. This proves it at
// Router's real interface (a *staffauth.GatedRouter, the same Mounter
// production uses) rather than by scanning source for the two calls,
// which is what routes_practice_test.go's guardrails used to do before
// this ticket.
func TestRouter_ReplayableAppliesMiddlewareAndWrapItself(t *testing.T) {
	db := testdb.New(t)
	calls := 0

	mux := http.NewServeMux()
	g := staffauth.NewGatedRouter(mux, db.App)
	rt := idempotency.NewRouter(g, db.App)
	rt.Replayable("POST /practices/{practiceId}/widgets", false, countingHandler(&calls, http.StatusCreated))
	srv := httptest.NewServer(mux)
	defer srv.Close()

	// No session cookie at all: if Router.Replayable did not apply
	// Middleware itself, this would reach countingHandler directly.
	req, err := http.NewRequestWithContext(t.Context(), http.MethodPost, srv.URL+"/practices/nonexistent/widgets", nil)
	if err != nil {
		t.Fatalf("build request: %v", err)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("unauthenticated request: %v", err)
	}
	_ = resp.Body.Close()
	if resp.StatusCode == http.StatusCreated {
		t.Fatalf("unauthenticated request reached the handler (status %d) -- Router.Replayable did not apply staffauth.Middleware", resp.StatusCode)
	}
	if calls != 0 {
		t.Fatalf("handler ran %d times before authentication, want 0", calls)
	}

	uid := "e2e-router-replay-uid"
	session := authntest.SeedSession(t, db.App, uid)
	practiceID := testdb.SeedPractice(t, db, "Router Replay Practice")
	testdb.SeedStaffAtPractice(t, db, practiceID, uid, []string{"owner"}, "employee")

	first := postWidget(t, srv, session, practiceID, "router-replay-key")
	defer first.Body.Close()
	second := postWidget(t, srv, session, practiceID, "router-replay-key")
	defer second.Body.Close()

	if calls != 1 {
		t.Fatalf("handler ran %d times across two requests carrying the same Idempotency-Key, want 1 -- Router.Replayable did not apply idempotency.Wrap", calls)
	}
	if second.StatusCode != first.StatusCode {
		t.Fatalf("replayed status = %d, want %d (the first response's own status)", second.StatusCode, first.StatusCode)
	}
}
