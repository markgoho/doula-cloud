package staffauth_test

import (
	"context"
	"net/http"
	"testing"
	"time"

	"doula-cloud/api/internal/authn"
	"doula-cloud/api/internal/authntest"
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
