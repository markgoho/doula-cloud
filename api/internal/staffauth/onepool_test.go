package staffauth_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"doula-cloud/api/internal/authn"
	"doula-cloud/api/internal/authntest"
	"doula-cloud/api/internal/clock"
	"doula-cloud/api/internal/stackswitch"
	"doula-cloud/api/internal/staffauth"
	"doula-cloud/api/internal/testdb"
)

// TestMiddleware_CompletesOnAOneConnectionPool is #2432's guard: a Staff
// request that renews its session and stamps activity never holds two
// pool connections at once, so it finishes on a pool of one.
func TestMiddleware_CompletesOnAOneConnectionPool(t *testing.T) {
	db := testdb.New(t)
	const identityUID = "staff-on-one-connection"
	practiceID, staffID := testdb.SeedStaffAtNewPractice(t, db, identityUID, []string{doulaRole}, employeeType)
	srv, _ := newServer(t, db, identityUID)
	defer srv.Close()
	session := authntest.SeedSessionAt(t, db.App, identityUID, time.Now().Add(-7*time.Hour))

	db.App.SetMaxOpenConns(1)

	ctx, cancel := context.WithTimeout(t.Context(), 10*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, pingURL(srv, practiceID), nil)
	if err != nil {
		t.Fatalf("build request: %v", err)
	}
	authntest.AddSessionCookie(req, session)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("request did not complete on a one-connection pool: %v", err)
	}
	defer resp.Body.Close()
	assertStatus(t, resp, http.StatusOK)

	renewed := false
	for _, c := range resp.Cookies() {
		if c.Name == authn.SessionCookieName {
			renewed = true
		}
	}
	if !renewed {
		t.Fatal("session past half its life was not renewed")
	}

	var lastPracticeID string
	if err := db.Admin.QueryRowContext(t.Context(),
		`SELECT last_practice_id FROM staff WHERE id = $1`, staffID).Scan(&lastPracticeID); err != nil {
		t.Fatalf("read stamp: %v", err)
	}
	if lastPracticeID != practiceID {
		t.Fatalf("last_practice_id = %q, want %q", lastPracticeID, practiceID)
	}
}

// TestMiddleware_CompletesOnAOneConnectionPoolWithTheStackClock is the
// same guard with the e2e stack's clock, which reads sim.offset_row on
// every clock.Now: that read must not wait on the request's pool.
func TestMiddleware_CompletesOnAOneConnectionPoolWithTheStackClock(t *testing.T) {
	db := testdb.New(t)
	if _, err := db.Admin.ExecContext(t.Context(), `
		CREATE SCHEMA sim;
		CREATE TABLE sim.offset_row (
		    id boolean PRIMARY KEY DEFAULT true CHECK (id),
		    delta interval NOT NULL DEFAULT '0'
		);
		INSERT INTO sim.offset_row (id) VALUES (true);
		GRANT USAGE ON SCHEMA sim TO app_runtime;
		GRANT SELECT ON sim.offset_row TO app_runtime;`); err != nil {
		t.Fatalf("install sim schema: %v", err)
	}
	now, closeClock, err := stackswitch.Clock(t.Context(), func(name string) string {
		return map[string]string{
			stackswitch.ClockVar:       "1",
			stackswitch.MarkerVar:      "127.0.0.1:9099",
			stackswitch.DatabaseURLVar: db.AppURL,
		}[name]
	}, db.App)
	if err != nil {
		t.Fatalf("stack clock: %v", err)
	}
	t.Cleanup(func() { _ = closeClock() })

	const identityUID = "staff-on-one-connection-stack-clock"
	practiceID, _ := testdb.SeedStaffAtNewPractice(t, db, identityUID, []string{doulaRole}, employeeType)
	mux := http.NewServeMux()
	mux.Handle("/practices/{practiceId}/ping", staffauth.Middleware(db.App)(
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_ = clock.Now(r.Context())
			w.WriteHeader(http.StatusOK)
		}),
	))
	srv := httptest.NewServer(clock.Middleware(now)(mux))
	defer srv.Close()
	session := authntest.SeedSessionAt(t, db.App, identityUID, time.Now().Add(-7*time.Hour))

	db.App.SetMaxOpenConns(1)

	ctx, cancel := context.WithTimeout(t.Context(), 10*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, pingURL(srv, practiceID), nil)
	if err != nil {
		t.Fatalf("build request: %v", err)
	}
	authntest.AddSessionCookie(req, session)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("request did not complete on a one-connection pool with the stack clock: %v", err)
	}
	defer resp.Body.Close()
	assertStatus(t, resp, http.StatusOK)
}
