package staffauth_test

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"doula-cloud/api/internal/apierr"
	"doula-cloud/api/internal/authntest"
	"doula-cloud/api/internal/staffauth"
	"doula-cloud/api/internal/testdb"
)

// newServer wires the middleware in front of a handler that echoes the
// resolved Staff/Practice ids and confirms a usable *sql.Tx was placed on
// the request context, so tests can assert on the middleware's contract
// with downstream handlers, not just the HTTP status code. It also seeds
// a live session for uid and hands back the token its __session cookie
// carries, since #151 that cookie is the only credential the
// middleware reads.
func newServer(t *testing.T, db *testdb.DB, uid string) (srv *httptest.Server, session string) {
	t.Helper()
	mux := http.NewServeMux()
	mux.Handle("/practices/{practiceId}/ping", staffauth.Middleware(db.App)(
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
	return httptest.NewServer(mux), authntest.SeedSession(t, db.App, uid)
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

// seedOwnerMembership mirrors seedStaffWithMembership but promotes the
// seeded Staff member to the 'owner' role -- the only role
// RequireOwner-gated actions accept as authorization.
func seedOwnerMembership(t *testing.T, db *testdb.DB, identityUID string) (staffID, practiceID string) {
	t.Helper()
	staffID, practiceID = seedStaffWithMembership(t, db, identityUID)
	if _, err := db.Admin.ExecContext(t.Context(), `UPDATE practice_memberships SET roles = '{owner}' WHERE staff_id = $1`, staffID); err != nil {
		t.Fatalf("promote to owner: %v", err)
	}
	return staffID, practiceID
}

// emptyUUID is a well-formed Practice id that matches nothing, for the
// tests whose request never gets far enough for it to matter.
const emptyUUID = "00000000-0000-0000-0000-000000000000"

// pingURL is the middleware-guarded route every test in this file hits.
func pingURL(srv *httptest.Server, practiceID string) string {
	return srv.URL + "/practices/" + practiceID + "/ping"
}

// get issues a GET with whatever credential setup applies to req.
func get(t *testing.T, url string, setup func(*http.Request)) *http.Response {
	t.Helper()
	req, err := http.NewRequestWithContext(t.Context(), http.MethodGet, url, nil)
	if err != nil {
		t.Fatalf("build request: %v", err)
	}
	setup(req)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	return resp
}

// assertStatus fails the test unless resp carries want.
func assertStatus(t *testing.T, resp *http.Response, want int) {
	t.Helper()
	if resp.StatusCode != want {
		t.Fatalf("status = %d, want %d", resp.StatusCode, want)
	}
}

func TestMiddleware_MissingCredential(t *testing.T) {
	db := testdb.New(t)
	srv, _ := newServer(t, db, "no-cookie-sent")
	defer srv.Close()

	resp := get(t, pingURL(srv, emptyUUID), func(*http.Request) {})
	defer resp.Body.Close()
	assertStatus(t, resp, http.StatusUnauthorized)
}

// TestMiddleware_BearerTokenAloneIsRejected is #151's AC on the Staff
// app: a request carrying only a Bearer ID token gets a 401. The Staff
// member behind the token exists and is a member of the Practice, so a
// 401 can only mean the header was never read.
func TestMiddleware_BearerTokenAloneIsRejected(t *testing.T) {
	db := testdb.New(t)
	const identityUID = "staff-holding-only-a-bearer-token"
	_, practiceID := seedStaffWithMembership(t, db, identityUID)

	srv, _ := newServer(t, db, identityUID)
	defer srv.Close()

	resp := get(t, pingURL(srv, practiceID), func(req *http.Request) {
		req.Header.Set("Authorization", "Bearer would-verify-fine")
	})
	defer resp.Body.Close()
	assertStatus(t, resp, http.StatusUnauthorized)
}

// TestMiddleware_UnknownSession covers a cookie that names no live
// session -- the shape a stale or forged cookie arrives in.
func TestMiddleware_UnknownSession(t *testing.T) {
	db := testdb.New(t)
	srv, _ := newServer(t, db, "irrelevant")
	defer srv.Close()

	resp := get(t, pingURL(srv, emptyUUID), func(req *http.Request) {
		authntest.AddSessionCookie(req, "never-issued")
	})
	defer resp.Body.Close()
	assertStatus(t, resp, http.StatusUnauthorized)
}

func TestMiddleware_InvalidPracticeID(t *testing.T) {
	db := testdb.New(t)
	srv, session := newServer(t, db, someUID)
	defer srv.Close()

	resp := get(t, pingURL(srv, "not-a-uuid"), func(req *http.Request) {
		authntest.AddSessionCookie(req, session)
	})
	defer resp.Body.Close()
	assertStatus(t, resp, http.StatusBadRequest)
}

func TestMiddleware_PopulationResolutionFailure(t *testing.T) {
	db := testdb.New(t)
	// A verified uid with no matching staff row: population resolution
	// fails even though the token itself is valid.
	srv, session := newServer(t, db, "unknown-uid")
	defer srv.Close()

	resp := get(t, pingURL(srv, emptyUUID), func(req *http.Request) {
		authntest.AddSessionCookie(req, session)
	})
	defer resp.Body.Close()
	assertStatus(t, resp, http.StatusForbidden)
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

	srv, session := newServer(t, db, identityUID)
	defer srv.Close()

	resp := get(t, pingURL(srv, otherPracticeID), func(req *http.Request) {
		authntest.AddSessionCookie(req, session)
	})
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusForbidden)
	}
}

func TestMiddleware_Success(t *testing.T) {
	db := testdb.New(t)
	const identityUID = "staff-with-membership"
	staffID, practiceID := seedStaffWithMembership(t, db, identityUID)

	srv, session := newServer(t, db, identityUID)
	defer srv.Close()

	resp := get(t, pingURL(srv, practiceID), func(req *http.Request) {
		authntest.AddSessionCookie(req, session)
	})
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
		h := staffauth.Middleware(db.App)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			gotTx, gotPracticeID, gotOK = staffauth.RequireTx(w, r)
		}))

		_, practiceID := seedStaffWithMembership(t, db, someUID)
		testReq := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/practices/"+practiceID+"/ping", nil)
		testReq.SetPathValue("practiceId", practiceID)
		authntest.AddSessionCookie(testReq, authntest.SeedSession(t, db.App, someUID))
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
		var out apierr.APIError
		if err := json.NewDecoder(rec.Body).Decode(&out); err != nil {
			t.Fatalf("decode body: %v", err)
		}
		if out.Message != "invalid practice id" {
			t.Fatalf("message = %q, want %q", out.Message, "invalid practice id")
		}
	})
}

// TestMiddleware_RecordsThatThePracticeWasHere proves the durable contact
// record #420 needs: an authenticated request stamps staff.last_active_at,
// and a second request the same day leaves it where it is. New York
// escheats an unspent Credit balance at three years' dormancy and accepts
// a verifiable login as the contact that stops the clock -- a request log
// would have rotated away long before then.
func TestMiddleware_RecordsThatThePracticeWasHere(t *testing.T) {
	db := testdb.New(t)
	const identityUID = "staff-recording-contact"
	staffID, practiceID := seedStaffWithMembership(t, db, identityUID)

	srv, session := newServer(t, db, identityUID)
	defer srv.Close()

	lastActive := func() time.Time {
		t.Helper()
		var seen sql.NullTime
		if err := db.Admin.QueryRowContext(t.Context(),
			`SELECT last_active_at FROM staff WHERE id = $1`, staffID).Scan(&seen); err != nil {
			t.Fatalf("query last_active_at: %v", err)
		}
		if !seen.Valid {
			t.Fatal("last_active_at is unset after an authenticated request")
		}
		return seen.Time
	}

	resp := get(t, pingURL(srv, practiceID), func(req *http.Request) {
		authntest.AddSessionCookie(req, session)
	})
	defer resp.Body.Close()
	first := lastActive()

	resp2 := get(t, pingURL(srv, practiceID), func(req *http.Request) {
		authntest.AddSessionCookie(req, session)
	})
	defer resp2.Body.Close()
	if second := lastActive(); !second.Equal(first) {
		t.Fatalf("last_active_at moved from %v to %v within the same day", first, second)
	}
}
