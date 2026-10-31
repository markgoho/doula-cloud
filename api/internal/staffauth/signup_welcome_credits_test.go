package staffauth_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"doula-cloud/api/internal/authntest"
	"doula-cloud/api/internal/clock"
	"doula-cloud/api/internal/idempotency"
	"doula-cloud/api/internal/staffauth"
	"doula-cloud/api/internal/tasknudge"
	"doula-cloud/api/internal/testdb"
	"doula-cloud/api/internal/welcomecredits"
)

// TestSignupHandler_EnqueuesTheWelcomeCreditsEmailOnce proves #1381's
// enqueue: one row, due fourteen days after the signup's own instant,
// and a retry after the first commit adds no second one.
func TestSignupHandler_EnqueuesTheWelcomeCreditsEmailOnce(t *testing.T) {
	db := testdb.New(t)
	signedUpAt := time.Date(2027, time.January, 4, 15, 30, 0, 0, time.UTC)

	mux := http.NewServeMux()
	g := staffauth.NewGatedRouter(mux, db.App)
	ir := idempotency.NewRouter(g, db.App)
	staffauth.Mount(g, ir, db.App, authntest.Verifier{UID: "welcome-owner", Email: jamieEmail},
		authntest.NewFakeAccountManager(), tasknudge.NoOpEnqueuer{}, neverSuppressed)
	srv := httptest.NewServer(clock.Middleware(func() time.Time { return signedUpAt })(mux))
	defer srv.Close()

	body := staffauth.SignupRequest{WorkState: "NY", FirstName: jamieName, LastName: ownerLastName}
	resp := postSignup(t, srv, "tok", body)
	var out staffauth.SignupResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	_ = resp.Body.Close()

	retry := postSignup(t, srv, "tok", body)
	_ = retry.Body.Close()
	if retry.StatusCode != http.StatusConflict {
		t.Fatalf("retry status = %d, want %d", retry.StatusCode, http.StatusConflict)
	}

	var count int
	var due, created time.Time
	var status string
	if err := db.Admin.QueryRowContext(t.Context(),
		`SELECT count(*) OVER (), next_attempt_at, created_at, status::text FROM welcome_credits_outbox WHERE practice_id = $1`,
		out.PracticeID,
	).Scan(&count, &due, &created, &status); err != nil {
		t.Fatalf("read welcome_credits_outbox: %v", err)
	}
	if count != 1 || status != "pending" {
		t.Fatalf("rows = %d status %q, want 1 pending", count, status)
	}
	if !due.Equal(signedUpAt.Add(welcomecredits.Delay)) || !created.Equal(signedUpAt) {
		t.Errorf("next_attempt_at = %v created_at = %v, want %v and %v", due, created, signedUpAt.Add(welcomecredits.Delay), signedUpAt)
	}
	if welcomecredits.Delay != 14*24*time.Hour {
		t.Errorf("Delay = %v, want fourteen days", welcomecredits.Delay)
	}
}
