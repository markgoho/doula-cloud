package staffauth_test

import (
	"context"
	"database/sql"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"doula-cloud/api/internal/authn"
	"doula-cloud/api/internal/staffauth"
	"doula-cloud/api/internal/testdb"
)

// fakeVerifier is a test double for authn.Verifier: real Identity
// Platform tokens can't be minted without a live GCP project, so tests
// inject this instead of doula-cloud/api/internal/authn.FirebaseVerifier.
type fakeVerifier struct {
	uid string
	err error
}

func (f fakeVerifier) VerifyIDToken(_ context.Context, _ string) (*authn.VerifiedToken, error) {
	if f.err != nil {
		return nil, f.err
	}
	return &authn.VerifiedToken{UID: f.uid}, nil
}

var errBadToken = errors.New("invalid token")

// newServer wires the middleware in front of a handler that echoes the
// resolved Staff/Practice ids and confirms a usable *sql.Tx was placed on
// the request context, so tests can assert on the middleware's contract
// with downstream handlers, not just the HTTP status code.
func newServer(verifier authn.Verifier, db *testdb.DB) *httptest.Server {
	mux := http.NewServeMux()
	mux.Handle("/practices/{practiceId}/ping", staffauth.Middleware(verifier, db.App)(
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			staffID, _ := staffauth.StaffID(r.Context())
			practiceID, _ := staffauth.PracticeID(r.Context())
			tx, ok := staffauth.Tx(r.Context())
			if !ok || tx == nil {
				http.Error(w, "no tx on context", http.StatusInternalServerError)
				return
			}
			w.Header().Set("X-Staff-Id", staffID)
			w.Header().Set("X-Practice-Id", practiceID)
			w.WriteHeader(http.StatusOK)
		}),
	))
	return httptest.NewServer(mux)
}

// seedStaffWithMembership inserts a Practice, a Staff row bound to
// identityUID, and a practice_memberships row linking them, using the
// superuser Admin connection (which bypasses RLS) so fixture setup itself
// isn't gated by the policies under test. It composes the seed* helpers
// from rls_test.go.
func seedStaffWithMembership(t *testing.T, db *testdb.DB, identityUID string) (staffID, practiceID string) {
	t.Helper()

	practiceID = seedPractice(t, db, "Test Practice")
	staffID = seedStaff(t, db, identityUID)
	seedMembership(t, db, practiceID, staffID)
	return staffID, practiceID
}

func TestMiddleware_MissingToken(t *testing.T) {
	db := testdb.New(t)
	srv := newServer(fakeVerifier{}, db)
	defer srv.Close()

	req, err := http.NewRequestWithContext(t.Context(), http.MethodGet, srv.URL+"/practices/00000000-0000-0000-0000-000000000000/ping", nil)
	if err != nil {
		t.Fatalf("build request: %v", err)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusUnauthorized)
	}
}

func TestMiddleware_EmptyBearerToken(t *testing.T) {
	db := testdb.New(t)
	srv := newServer(fakeVerifier{}, db)
	defer srv.Close()

	req, _ := http.NewRequestWithContext(t.Context(), http.MethodGet, srv.URL+"/practices/00000000-0000-0000-0000-000000000000/ping", nil)
	req.Header.Set("Authorization", "Bearer ")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusUnauthorized)
	}
}

func TestMiddleware_TokenVerificationFailure(t *testing.T) {
	db := testdb.New(t)
	srv := newServer(fakeVerifier{err: errBadToken}, db)
	defer srv.Close()

	req, _ := http.NewRequestWithContext(t.Context(), http.MethodGet, srv.URL+"/practices/00000000-0000-0000-0000-000000000000/ping", nil)
	req.Header.Set("Authorization", "Bearer some-token")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusUnauthorized)
	}
}

func TestMiddleware_InvalidPracticeID(t *testing.T) {
	db := testdb.New(t)
	srv := newServer(fakeVerifier{uid: someUID}, db)
	defer srv.Close()

	req, _ := http.NewRequestWithContext(t.Context(), http.MethodGet, srv.URL+"/practices/not-a-uuid/ping", nil)
	req.Header.Set("Authorization", "Bearer some-token")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusBadRequest)
	}
}

func TestMiddleware_PopulationResolutionFailure(t *testing.T) {
	db := testdb.New(t)
	// A verified uid with no matching staff row: population resolution
	// fails even though the token itself is valid.
	srv := newServer(fakeVerifier{uid: "unknown-uid"}, db)
	defer srv.Close()

	req, _ := http.NewRequestWithContext(t.Context(), http.MethodGet, srv.URL+"/practices/00000000-0000-0000-0000-000000000000/ping", nil)
	req.Header.Set("Authorization", "Bearer some-token")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusForbidden)
	}
}

func TestMiddleware_NoPracticeMembership(t *testing.T) {
	db := testdb.New(t)
	const identityUID = "staff-without-membership"
	_, _ = seedStaffWithMembership(t, db, identityUID)

	// A different, unrelated Practice: the caller is a known Staff member,
	// but not of this Practice.
	var otherPracticeID string
	if err := db.Admin.QueryRowContext(t.Context(),
		`INSERT INTO practices (name) VALUES ('Other Practice') RETURNING id`,
	).Scan(&otherPracticeID); err != nil {
		t.Fatalf("seed other practice: %v", err)
	}

	srv := newServer(fakeVerifier{uid: identityUID}, db)
	defer srv.Close()

	req, _ := http.NewRequestWithContext(t.Context(), http.MethodGet, srv.URL+"/practices/"+otherPracticeID+"/ping", nil)
	req.Header.Set("Authorization", "Bearer some-token")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusForbidden)
	}
}

func TestMiddleware_Success(t *testing.T) {
	db := testdb.New(t)
	const identityUID = "staff-with-membership"
	staffID, practiceID := seedStaffWithMembership(t, db, identityUID)

	srv := newServer(fakeVerifier{uid: identityUID}, db)
	defer srv.Close()

	req, _ := http.NewRequestWithContext(t.Context(), http.MethodGet, srv.URL+"/practices/"+practiceID+"/ping", nil)
	req.Header.Set("Authorization", "Bearer some-token")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}
	if got := resp.Header.Get("X-Staff-Id"); got != staffID {
		t.Fatalf("X-Staff-Id = %q, want %q", got, staffID)
	}
	if got := resp.Header.Get("X-Practice-Id"); got != practiceID {
		t.Fatalf("X-Practice-Id = %q, want %q", got, practiceID)
	}

	var lastPracticeID string
	if err := db.Admin.QueryRowContext(t.Context(), `SELECT last_practice_id FROM staff WHERE id = $1`, staffID).Scan(&lastPracticeID); err != nil {
		t.Fatalf("query last_practice_id: %v", err)
	}
	if lastPracticeID != practiceID {
		t.Fatalf("last_practice_id = %q, want %q", lastPracticeID, practiceID)
	}
}

// TestMiddleware_FailClosedWithoutSessionVar proves the RLS backstop
// directly: querying practice_memberships as the app_runtime role with no
// app.current_practice_id set returns zero rows, not an error and not
// real data, even though the row genuinely exists.
func TestMiddleware_FailClosedWithoutSessionVar(t *testing.T) {
	db := testdb.New(t)
	seedStaffWithMembership(t, db, "fail-closed-uid")

	var count int
	if err := db.App.QueryRowContext(t.Context(), `SELECT count(*) FROM practice_memberships`).Scan(&count); err != nil {
		t.Fatalf("query practice_memberships as app role: %v", err)
	}
	if count != 0 {
		t.Fatalf("expected 0 rows with no session variable set, got %d", count)
	}
}

func TestRequireTx(t *testing.T) {
	t.Run("tx present", func(t *testing.T) {
		db := testdb.New(t)
		rec := httptest.NewRecorder()
		var gotTx *sql.Tx
		var gotPracticeID string
		var gotOK bool
		h := staffauth.Middleware(fakeVerifier{uid: someUID}, db.App)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			gotTx, gotPracticeID, gotOK = staffauth.RequireTx(w, r)
		}))

		_, practiceID := seedStaffWithMembership(t, db, someUID)
		testReq := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/practices/"+practiceID+"/ping", nil)
		testReq.SetPathValue("practiceId", practiceID)
		testReq.Header.Set("Authorization", "Bearer token")
		h.ServeHTTP(rec, testReq)

		if !gotOK {
			t.Fatalf("expected ok=true, got false")
		}
		if gotTx == nil {
			t.Fatalf("expected non-nil tx")
		}
		if gotPracticeID != practiceID {
			t.Fatalf("practiceID = %q, want %q", gotPracticeID, practiceID)
		}
	})

	t.Run("tx missing", func(t *testing.T) {
		rec := httptest.NewRecorder()
		req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/", nil)
		tx, practiceID, ok := staffauth.RequireTx(rec, req)
		if ok {
			t.Fatalf("expected ok=false, got true (tx=%v, practiceID=%q)", tx, practiceID)
		}
		if rec.Code != http.StatusInternalServerError {
			t.Fatalf("status = %d, want %d", rec.Code, http.StatusInternalServerError)
		}
	})
}

func TestParseUUID(t *testing.T) {
	t.Run("valid uuid", func(t *testing.T) {
		rec := httptest.NewRecorder()
		if !staffauth.ParseUUID(rec, "practice", "00000000-0000-0000-0000-000000000000") {
			t.Fatalf("expected true for valid uuid")
		}
	})

	t.Run("invalid uuid", func(t *testing.T) {
		rec := httptest.NewRecorder()
		if staffauth.ParseUUID(rec, "practice", "not-a-uuid") {
			t.Fatalf("expected false for invalid uuid")
		}
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("status = %d, want %d", rec.Code, http.StatusBadRequest)
		}
		if got := rec.Body.String(); got != "invalid practice id\n" {
			t.Fatalf("body = %q, want %q", got, "invalid practice id\n")
		}
	})
}
