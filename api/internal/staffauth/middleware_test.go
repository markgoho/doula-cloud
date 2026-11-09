package staffauth_test

import (
	"context"
	"database/sql"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"doula-cloud/api/internal/apierr"
	"doula-cloud/api/internal/apierrtest"
	"doula-cloud/api/internal/authntest"
	"doula-cloud/api/internal/reqtest"
	"doula-cloud/api/internal/staffauth"
	"doula-cloud/api/internal/testdb"
)

// neverSuppressed stands in for mailsuppress.Active in every test that
// does not itself exercise the Staff-invite suppression check (#861):
// no address is ever blocked. Shared across this package's test files
// rather than repeated, since staffauth.Mount needs one in every server
// it wires up.
func neverSuppressed(context.Context, *sql.Tx, string) (bool, error) {
	return false, nil
}

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

// newPingOKServer mounts the middleware in front of a handler that only
// answers 200, for the tests whose subject is whether the middleware
// admits the caller at all.
func newPingOKServer(db *testdb.DB) *httptest.Server {
	mux := http.NewServeMux()
	mux.Handle("/practices/{practiceId}/ping", staffauth.Middleware(db.App)(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})))
	return httptest.NewServer(mux)
}

// seedOwnerMembership seeds a Practice and a Staff member via
// testdb.SeedStaffAtNewPractice, then promotes that member to the 'owner'
// role -- the only role RequireOwner-gated actions accept as
// authorization. Stays local: the promotion UPDATE is a fact only this
// package's owner-gated tests need, and testdb has no "seed an owner"
// export.
func seedOwnerMembership(t *testing.T, db *testdb.DB, identityUID string) (staffID, practiceID string) {
	t.Helper()
	practiceID, staffID = testdb.SeedStaffAtNewPractice(t, db, identityUID, []string{doulaRole}, employeeType)
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

func TestMiddleware_MissingCredential(t *testing.T) {
	db := testdb.New(t)
	srv, _ := newServer(t, db, "no-cookie-sent")
	defer srv.Close()

	resp := reqtest.Get(t, pingURL(srv, emptyUUID))
	defer resp.Body.Close()
	reqtest.RequireStatus(t, resp, http.StatusUnauthorized)
}

// TestMiddleware_BearerTokenAloneIsRejected is #151's AC on the Staff
// app: a request carrying only a Bearer ID token gets a 401. The Staff
// member behind the token exists and is a member of the Practice, so a
// 401 can only mean the header was never read.
func TestMiddleware_BearerTokenAloneIsRejected(t *testing.T) {
	db := testdb.New(t)
	const identityUID = "staff-holding-only-a-bearer-token"
	practiceID, _ := testdb.SeedStaffAtNewPractice(t, db, identityUID, []string{doulaRole}, employeeType)

	srv, _ := newServer(t, db, identityUID)
	defer srv.Close()

	resp := reqtest.Get(t, pingURL(srv, practiceID), reqtest.Header("Authorization", "Bearer would-verify-fine"))
	defer resp.Body.Close()
	reqtest.RequireStatus(t, resp, http.StatusUnauthorized)
}

// TestMiddleware_UnknownSession covers a cookie that names no live
// session -- the shape a stale or forged cookie arrives in.
func TestMiddleware_UnknownSession(t *testing.T) {
	db := testdb.New(t)
	srv, _ := newServer(t, db, "irrelevant")
	defer srv.Close()

	resp := reqtest.Get(t, pingURL(srv, emptyUUID), reqtest.Session("never-issued"))
	defer resp.Body.Close()
	reqtest.RequireStatus(t, resp, http.StatusUnauthorized)
}

func TestMiddleware_InvalidPracticeID(t *testing.T) {
	db := testdb.New(t)
	srv, session := newServer(t, db, someUID)
	defer srv.Close()

	resp := reqtest.Get(t, pingURL(srv, "not-a-uuid"), reqtest.Session(session))
	defer resp.Body.Close()
	reqtest.RequireStatus(t, resp, http.StatusBadRequest)
}

func TestMiddleware_PopulationResolutionFailure(t *testing.T) {
	db := testdb.New(t)
	// A verified uid with no matching staff row: population resolution
	// fails even though the token itself is valid.
	srv, session := newServer(t, db, "unknown-uid")
	defer srv.Close()

	resp := reqtest.Get(t, pingURL(srv, emptyUUID), reqtest.Session(session))
	defer resp.Body.Close()
	reqtest.RequireStatus(t, resp, http.StatusForbidden)
}

func TestMiddleware_NoPracticeMembership(t *testing.T) {
	db := testdb.New(t)
	const identityUID = "staff-without-membership"
	_, _ = testdb.SeedStaffAtNewPractice(t, db, identityUID, []string{doulaRole}, employeeType)

	// A different, unrelated Practice: the caller is a known Staff member,
	// but not of this Practice.
	otherPracticeID := testdb.Scalar[string](t, db.Admin, `INSERT INTO practices (name, timezone) VALUES ('Other Practice', 'America/New_York') RETURNING id`)

	srv, session := newServer(t, db, identityUID)
	defer srv.Close()

	resp := reqtest.Get(t, pingURL(srv, otherPracticeID), reqtest.Session(session))
	defer resp.Body.Close()

	reqtest.RequireStatus(t, resp, http.StatusForbidden)
}

// TestMiddleware_NoSuchPractice covers a well-formed but nonexistent
// Practice id -- setPracticeAndCheckMembership's LEFT JOIN against
// practices finds no row at all, distinct from finding the Practice but
// no membership (TestMiddleware_NoPracticeMembership above), though both
// answer the same outward 403.
func TestMiddleware_NoSuchPractice(t *testing.T) {
	db := testdb.New(t)
	const identityUID = "staff-with-no-such-practice"
	testdb.SeedStaff(t, db, identityUID)

	srv, session := newServer(t, db, identityUID)
	defer srv.Close()

	resp := reqtest.Get(t, pingURL(srv, emptyUUID), reqtest.Session(session))
	defer resp.Body.Close()
	reqtest.RequireStatus(t, resp, http.StatusForbidden)
}

func TestMiddleware_Success(t *testing.T) {
	db := testdb.New(t)
	const identityUID = "staff-with-membership"
	practiceID, staffID := testdb.SeedStaffAtNewPractice(t, db, identityUID, []string{doulaRole}, employeeType)

	srv, session := newServer(t, db, identityUID)
	defer srv.Close()

	resp := reqtest.Get(t, pingURL(srv, practiceID), reqtest.Session(session))
	defer resp.Body.Close()

	reqtest.RequireStatus(t, resp, http.StatusOK)
	if got := resp.Header.Get("X-Staff-Id"); got != staffID {
		t.Fatalf("X-Staff-Id = %q, want %q", got, staffID)
	}
	if got := resp.Header.Get("X-Practice-Id"); got != practiceID {
		t.Fatalf("X-Practice-Id = %q, want %q", got, practiceID)
	}

	lastPracticeID := testdb.Scalar[string](t, db.Admin, `SELECT last_practice_id FROM staff WHERE id = $1`, staffID)
	if lastPracticeID != practiceID {
		t.Fatalf("last_practice_id = %q, want %q", lastPracticeID, practiceID)
	}
}

// TestMiddleware_ActivityStampSkipsNoOpWrite is #1197's third acceptance
// criterion: once last_active_at is already set today and
// last_practice_id already names this Practice, the request writes
// nothing to the staff row at all. xmin is Postgres's own row-version
// counter -- it changes on every UPDATE, HOT or not, even one that
// leaves every column holding the value it already had -- so an
// unchanged xmin is direct proof no write happened, not just that the
// visible columns look the same.
func TestMiddleware_ActivityStampSkipsNoOpWrite(t *testing.T) {
	db := testdb.New(t)
	const identityUID = "staff-already-stamped-today"
	practiceID, staffID := testdb.SeedStaffAtNewPractice(t, db, identityUID, []string{doulaRole}, employeeType)

	if _, err := db.Admin.ExecContext(t.Context(),
		`UPDATE staff SET last_practice_id = $1, last_active_at = now() WHERE id = $2`,
		practiceID, staffID); err != nil {
		t.Fatalf("seed prior stamp: %v", err)
	}

	xminBefore := testdb.Scalar[string](t, db.Admin, `SELECT xmin::text FROM staff WHERE id = $1`, staffID)

	srv, session := newServer(t, db, identityUID)
	defer srv.Close()

	resp := reqtest.Get(t, pingURL(srv, practiceID), reqtest.Session(session))
	defer resp.Body.Close()
	reqtest.RequireStatus(t, resp, http.StatusOK)

	xminAfter := testdb.Scalar[string](t, db.Admin, `SELECT xmin::text FROM staff WHERE id = $1`, staffID)
	if xminAfter != xminBefore {
		t.Fatalf("staff row was written (xmin %s -> %s), want no write for the already-current case", xminBefore, xminAfter)
	}
}

// TestMiddleware_ActivityStampRefreshesPracticeAcrossDays is #1197's
// fourth acceptance criterion, the two halves it asks to keep: a
// same-day Staff member who switches Practice still gets the new
// last_practice_id (not the once-a-day skip TestMiddleware_ActivityStampSkipsNoOpWrite
// covers), and a stale last_active_at still gets refreshed rather than
// left at yesterday's stamp.
func TestMiddleware_ActivityStampRefreshesPracticeAcrossDays(t *testing.T) {
	db := testdb.New(t)
	const identityUID = "staff-switches-practice"
	firstPracticeID, staffID := testdb.SeedStaffAtNewPractice(t, db, identityUID, []string{doulaRole}, employeeType)
	secondPracticeID := testdb.SeedPractice(t, db, "Second Practice")
	if _, err := db.Admin.ExecContext(t.Context(),
		`INSERT INTO practice_memberships (practice_id, staff_id, roles, employment_type) VALUES ($1, $2, '{doula}'::practice_role[], $3)`,
		secondPracticeID, staffID, employeeType); err != nil {
		t.Fatalf("seed second membership: %v", err)
	}

	yesterday := time.Now().Add(-25 * time.Hour)
	if _, err := db.Admin.ExecContext(t.Context(),
		`UPDATE staff SET last_practice_id = $1, last_active_at = $2 WHERE id = $3`,
		firstPracticeID, yesterday, staffID); err != nil {
		t.Fatalf("seed prior stamp: %v", err)
	}

	srv, session := newServer(t, db, identityUID)
	defer srv.Close()

	resp := reqtest.Get(t, pingURL(srv, secondPracticeID), reqtest.Session(session))
	defer resp.Body.Close()
	reqtest.RequireStatus(t, resp, http.StatusOK)

	var lastPracticeID string
	var lastActiveAt time.Time
	if err := db.Admin.QueryRowContext(t.Context(),
		`SELECT last_practice_id, last_active_at FROM staff WHERE id = $1`, staffID,
	).Scan(&lastPracticeID, &lastActiveAt); err != nil {
		t.Fatalf("read stamp: %v", err)
	}
	if lastPracticeID != secondPracticeID {
		t.Fatalf("last_practice_id = %q, want %q", lastPracticeID, secondPracticeID)
	}
	if !lastActiveAt.After(yesterday.Add(time.Hour)) {
		t.Fatalf("last_active_at = %v, want refreshed to roughly now, not left at yesterday's stamp", lastActiveAt)
	}
}

// TestMiddleware_ActivityStampDoesNotSerializeConcurrentRequests is
// #1197's second acceptance criterion: it holds one request's Middleware
// transaction open (standing in for #1077's slow RLS-scoped read, a slow
// read that has nothing to do with staffauth) and shows a second
// request from the same Staff member completes anyway, rather than
// blocking on the first request's staff row lock for however long the
// first request takes.
func TestMiddleware_ActivityStampDoesNotSerializeConcurrentRequests(t *testing.T) {
	db := testdb.New(t)
	db.OptOutOfOneConnectionPool("two requests are in flight at once on purpose; each holds its own pool connection")
	const identityUID = "staff-concurrent-requests"
	practiceID, _ := testdb.SeedStaffAtNewPractice(t, db, identityUID, []string{doulaRole}, employeeType)

	entered := make(chan struct{})
	release := make(chan struct{})

	mux := http.NewServeMux()
	mux.Handle("/practices/{practiceId}/ping", staffauth.Middleware(db.App)(
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Header.Get("X-Test-Slow") == "1" {
				entered <- struct{}{}
				<-release
			}
			w.WriteHeader(http.StatusOK)
		}),
	))
	srv := httptest.NewServer(mux)
	defer srv.Close()
	session := authntest.SeedSession(t, db.App, identityUID)

	type result struct {
		resp *http.Response
		err  error
	}
	ping := func(slow bool) (*http.Response, error) {
		req, err := http.NewRequestWithContext(t.Context(), http.MethodGet, pingURL(srv, practiceID), nil)
		if err != nil {
			return nil, fmt.Errorf("build request: %w", err)
		}
		authntest.AddSessionCookie(req, session)
		if slow {
			req.Header.Set("X-Test-Slow", "1")
		}
		return http.DefaultClient.Do(req) //nolint:bodyclose // the body is closed by whichever select branch below receives this result
	}

	firstDone := make(chan result, 1)
	go func() {
		resp, err := ping(true) //nolint:bodyclose // closed after close(release) below
		firstDone <- result{resp, err}
	}()

	select {
	case <-entered:
	case <-time.After(5 * time.Second):
		t.Fatal("first request never reached its handler")
	}

	secondDone := make(chan result, 1)
	go func() {
		resp, err := ping(false) //nolint:bodyclose // closed in the select branch below
		secondDone <- result{resp, err}
	}()

	select {
	case r := <-secondDone:
		if r.err != nil {
			t.Fatalf("second request: %v", r.err)
		}
		defer r.resp.Body.Close()
		reqtest.RequireStatus(t, r.resp, http.StatusOK)
	case <-time.After(3 * time.Second):
		t.Fatal("second request serialized behind the first request's still-open transaction")
	}

	close(release)
	r := <-firstDone
	if r.err != nil {
		t.Fatalf("first request: %v", r.err)
	}
	defer r.resp.Body.Close()
	reqtest.RequireStatus(t, r.resp, http.StatusOK)
}

// TestMiddleware_FailClosedWithoutSessionVar proves the RLS backstop
// directly: querying practice_memberships as the app_runtime role with no
// app.current_practice_id set returns zero rows, not an error and not
// real data, even though the row genuinely exists.
func TestMiddleware_FailClosedWithoutSessionVar(t *testing.T) {
	db := testdb.New(t)
	testdb.SeedStaffAtNewPractice(t, db, "fail-closed-uid", []string{doulaRole}, employeeType)

	count := testdb.Scalar[int](t, db.App, `SELECT count(*) FROM practice_memberships`)
	if count != 0 {
		t.Fatalf("expected 0 rows with no session variable set, got %d", count)
	}
}

// setRequireMFA throws practiceID's switch (#606) directly via the Admin
// connection, bypassing the PUT handler under test elsewhere. Every test
// that needs the switch on seeds a Practice off (the migration's own
// default) and calls this -- nothing needs to reset it false, which is
// what a Practice already starts as.
func setRequireMFA(t *testing.T, db *testdb.DB, practiceID string) {
	t.Helper()
	if _, err := db.Admin.ExecContext(t.Context(),
		`UPDATE practices SET require_mfa_for_all_staff = true WHERE id = $1`, practiceID,
	); err != nil {
		t.Fatalf("set require_mfa_for_all_staff: %v", err)
	}
}

// TestMiddleware_MFAGate is the boundary's rule since #1531 (ADR-0026's
// amendment for #1492): refuse a Practice-scoped request only when the
// Practice's switch is on and the caller's session shows no second
// factor. A Membership that is Owner is not a reason.
func TestMiddleware_MFAGate(t *testing.T) {
	t.Run("owner without second factor is admitted while the switch is off", func(t *testing.T) {
		db := testdb.New(t)
		const identityUID = "owner-no-mfa"
		_, practiceID := seedOwnerMembership(t, db, identityUID)

		srv := newPingOKServer(db)
		defer srv.Close()

		session := authntest.SeedSessionWithSecondFactor(t, db.App, identityUID, false)
		resp := reqtest.Get(t, pingURL(srv, practiceID), reqtest.Session(session))
		defer resp.Body.Close()
		reqtest.RequireStatus(t, resp, http.StatusOK)
	})

	t.Run("the switch bars an un-enrolled owner", func(t *testing.T) {
		db := testdb.New(t)
		const identityUID = "owner-switch-on"
		_, practiceID := seedOwnerMembership(t, db, identityUID)
		setRequireMFA(t, db, practiceID)

		srv := newPingOKServer(db)
		defer srv.Close()

		session := authntest.SeedSessionWithSecondFactor(t, db.App, identityUID, false)
		resp := reqtest.Get(t, pingURL(srv, practiceID), reqtest.Session(session))
		defer resp.Body.Close()
		reqtest.RequireStatus(t, resp, http.StatusForbidden)

		body := apierrtest.Decode(t, resp)
		if body.Code != apierr.CodeMFARequired {
			t.Fatalf("code = %q, want MFA_REQUIRED", body.Code)
		}
	})

	t.Run("owner with second factor is admitted", func(t *testing.T) {
		db := testdb.New(t)
		const identityUID = "owner-with-mfa"
		_, practiceID := seedOwnerMembership(t, db, identityUID)

		srv := newPingOKServer(db)
		defer srv.Close()

		session := authntest.SeedSessionWithSecondFactor(t, db.App, identityUID, true)
		resp := reqtest.Get(t, pingURL(srv, practiceID), reqtest.Session(session))
		defer resp.Body.Close()
		reqtest.RequireStatus(t, resp, http.StatusOK)
	})

	t.Run("doula is never forced while the switch is off", func(t *testing.T) {
		db := testdb.New(t)
		const identityUID = "doula-no-mfa"
		practiceID, _ := testdb.SeedStaffAtNewPractice(t, db, identityUID, []string{doulaRole}, employeeType)

		srv := newPingOKServer(db)
		defer srv.Close()

		session := authntest.SeedSessionWithSecondFactor(t, db.App, identityUID, false)
		resp := reqtest.Get(t, pingURL(srv, practiceID), reqtest.Session(session))
		defer resp.Body.Close()
		reqtest.RequireStatus(t, resp, http.StatusOK)
	})

	t.Run("the switch bars an un-enrolled doula", func(t *testing.T) {
		db := testdb.New(t)
		const identityUID = "doula-switch-on"
		practiceID, _ := testdb.SeedStaffAtNewPractice(t, db, identityUID, []string{doulaRole}, employeeType)
		setRequireMFA(t, db, practiceID)

		srv := newPingOKServer(db)
		defer srv.Close()

		session := authntest.SeedSessionWithSecondFactor(t, db.App, identityUID, false)
		resp := reqtest.Get(t, pingURL(srv, practiceID), reqtest.Session(session))
		defer resp.Body.Close()
		reqtest.RequireStatus(t, resp, http.StatusForbidden)
	})

	t.Run("the switch admits an enrolled doula", func(t *testing.T) {
		db := testdb.New(t)
		const identityUID = "doula-switch-on-enrolled"
		practiceID, _ := testdb.SeedStaffAtNewPractice(t, db, identityUID, []string{doulaRole}, employeeType)
		setRequireMFA(t, db, practiceID)

		srv := newPingOKServer(db)
		defer srv.Close()

		session := authntest.SeedSessionWithSecondFactor(t, db.App, identityUID, true)
		resp := reqtest.Get(t, pingURL(srv, practiceID), reqtest.Session(session))
		defer resp.Body.Close()
		reqtest.RequireStatus(t, resp, http.StatusOK)
	})

	// TestMiddleware_MFAGate/split proves #606's own AC: a person whose
	// Membership at one Practice requires MFA and at the other does not is
	// barred from the first and admitted to the second while un-enrolled, one identity and
	// one session cookie throughout -- the fact GLOSSARY.md's per-Membership
	// role model is what forced the gate off session.CreateHandler in the
	// first place (#167's amendment).
	t.Run("split across two practices", func(t *testing.T) {
		db := testdb.New(t)
		const identityUID = "contractor-split"
		staffID, ownerPracticeID := seedOwnerMembership(t, db, identityUID)
		doulaPracticeID := testdb.SeedPractice(t, db, "Second Practice")
		seedMembership(t, db, doulaPracticeID, staffID)
		// #1531: the split is the switch, never the role. She is Owner
		// at the Practice that has not thrown it, and only a Doula at
		// the one that has.
		setRequireMFA(t, db, doulaPracticeID)

		srv := newPingOKServer(db)
		defer srv.Close()

		unenrolled := authntest.SeedSessionWithSecondFactor(t, db.App, identityUID, false)
		respA := reqtest.Get(t, pingURL(srv, ownerPracticeID), reqtest.Session(unenrolled))
		defer respA.Body.Close()
		reqtest.RequireStatus(t, respA, http.StatusOK)

		respB := reqtest.Get(t, pingURL(srv, doulaPracticeID), reqtest.Session(unenrolled))
		defer respB.Body.Close()
		reqtest.RequireStatus(t, respB, http.StatusForbidden)

		enrolled := authntest.SeedSessionWithSecondFactor(t, db.App, identityUID, true)
		respA2 := reqtest.Get(t, pingURL(srv, ownerPracticeID), reqtest.Session(enrolled))
		defer respA2.Body.Close()
		reqtest.RequireStatus(t, respA2, http.StatusOK)

		respB2 := reqtest.Get(t, pingURL(srv, doulaPracticeID), reqtest.Session(enrolled))
		defer respB2.Body.Close()
		reqtest.RequireStatus(t, respB2, http.StatusOK)
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
		out := apierrtest.Decode(t, rec.Result())
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
	practiceID, staffID := testdb.SeedStaffAtNewPractice(t, db, identityUID, []string{doulaRole}, employeeType)

	srv, session := newServer(t, db, identityUID)
	defer srv.Close()

	lastActive := func() time.Time {
		t.Helper()
		seen := testdb.Scalar[sql.NullTime](t, db.Admin, `SELECT last_active_at FROM staff WHERE id = $1`, staffID)
		if !seen.Valid {
			t.Fatal("last_active_at is unset after an authenticated request")
		}
		return seen.Time
	}

	resp := reqtest.Get(t, pingURL(srv, practiceID), reqtest.Session(session))
	defer resp.Body.Close()
	first := lastActive()

	resp2 := reqtest.Get(t, pingURL(srv, practiceID), reqtest.Session(session))
	defer resp2.Body.Close()
	if second := lastActive(); !second.Equal(first) {
		t.Fatalf("last_active_at moved from %v to %v within the same day", first, second)
	}
}

// deletionServer wires the middleware in front of both an ordinary ping
// route and a stand-in for practicedeletion's own
// GET/DELETE /api/practices/{practiceId}/deletion -- the one path
// isPracticeDeletionRoute names -- so #871's lockout tests can tell an
// Owner's exempted reach into that one route apart from every other
// route, which stays refused.
func deletionServer(db *testdb.DB) *httptest.Server {
	mux := http.NewServeMux()
	ok := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusOK) })
	mux.Handle("/practices/{practiceId}/ping", staffauth.Middleware(db.App)(ok))
	mux.Handle("GET /api/practices/{practiceId}/deletion", staffauth.Middleware(db.App)(ok))
	mux.Handle("DELETE /api/practices/{practiceId}/deletion", staffauth.Middleware(db.App)(ok))
	mux.Handle("POST /api/practices/{practiceId}/deletion", staffauth.Middleware(db.App)(ok))
	mux.Handle("GET /api/practices/{practiceId}/session", staffauth.Middleware(db.App)(staffauth.PracticeSessionHandler()))
	return httptest.NewServer(mux)
}

func markPracticePendingDeletion(t *testing.T, db *testdb.DB, practiceID string) {
	t.Helper()
	if _, err := db.Admin.ExecContext(t.Context(),
		`UPDATE practices SET deletion_requested_at = now(), deletion_finalize_at = now() + interval '30 days' WHERE id = $1`,
		practiceID,
	); err != nil {
		t.Fatalf("mark pending deletion: %v", err)
	}
}

func TestMiddleware_PendingDeletionLockout_BlocksOrdinaryRoutes(t *testing.T) {
	db := testdb.New(t)
	const identityUID = "owner-pending-blocked"
	_, practiceID := seedOwnerMembership(t, db, identityUID)
	markPracticePendingDeletion(t, db, practiceID)

	srv := deletionServer(db)
	defer srv.Close()

	session := authntest.SeedSessionWithSecondFactor(t, db.App, identityUID, true)
	resp := reqtest.Get(t, pingURL(srv, practiceID), reqtest.Session(session))
	defer resp.Body.Close()
	reqtest.RequireStatus(t, resp, http.StatusForbidden)

	body := apierrtest.Decode(t, resp)
	if body.Code != apierr.CodePracticePendingDeletion {
		t.Fatalf("code = %q, want %s", body.Code, apierr.CodePracticePendingDeletion)
	}
}

func TestMiddleware_PendingDeletionLockout_BlocksNonOwner(t *testing.T) {
	db := testdb.New(t)
	const identityUID = "doula-pending-blocked"
	practiceID, _ := testdb.SeedStaffAtNewPractice(t, db, identityUID, []string{doulaRole}, employeeType)
	markPracticePendingDeletion(t, db, practiceID)

	srv := deletionServer(db)
	defer srv.Close()

	session := authntest.SeedSession(t, db.App, identityUID)
	resp := reqtest.Get(t, pingURL(srv, practiceID), reqtest.Session(session))
	defer resp.Body.Close()
	reqtest.RequireStatus(t, resp, http.StatusForbidden)

	body := apierrtest.Decode(t, resp)
	if body.Code != apierr.CodePracticePendingDeletion {
		t.Fatalf("code = %q, want %s", body.Code, apierr.CodePracticePendingDeletion)
	}
}

func TestMiddleware_PendingDeletionLockout_ExemptsOwnerOnDeletionRoutes(t *testing.T) {
	db := testdb.New(t)
	const identityUID = "owner-pending-exempt"
	_, practiceID := seedOwnerMembership(t, db, identityUID)
	markPracticePendingDeletion(t, db, practiceID)

	srv := deletionServer(db)
	defer srv.Close()
	session := authntest.SeedSessionWithSecondFactor(t, db.App, identityUID, true)

	getResp := reqtest.Get(t, srv.URL+"/api/practices/"+practiceID+"/deletion", reqtest.Session(session))
	defer getResp.Body.Close()
	reqtest.RequireStatus(t, getResp, http.StatusOK)

	deleteResp := reqtest.Delete(t, srv.URL+"/api/practices/"+practiceID+"/deletion", reqtest.Session(session))
	defer deleteResp.Body.Close()
	reqtest.RequireStatus(t, deleteResp, http.StatusOK)
}

// TestMiddleware_PendingDeletionLockout_ExemptsSessionForEveryRole proves
// the one route the lockout leaves open regardless of role: app/'s
// +layout.ts load needs GET .../session to succeed for an Owner (to
// route her to the restore screen) and for a non-Owner (to show an
// accurate locked message) alike, carrying PendingDeletion: true either
// way -- unlike GET/DELETE .../deletion, which stays Owner-only.
func TestMiddleware_PendingDeletionLockout_ExemptsSessionForEveryRole(t *testing.T) {
	db := testdb.New(t)
	const identityUID = "doula-pending-session-read"
	practiceID, _ := testdb.SeedStaffAtNewPractice(t, db, identityUID, []string{doulaRole}, employeeType)
	markPracticePendingDeletion(t, db, practiceID)

	srv := deletionServer(db)
	defer srv.Close()
	session := authntest.SeedSession(t, db.App, identityUID)

	resp := reqtest.Get(t, srv.URL+"/api/practices/"+practiceID+"/session", reqtest.Session(session))
	defer resp.Body.Close()
	reqtest.RequireStatus(t, resp, http.StatusOK)

	body := reqtest.Decode[staffauth.PracticeSessionResponse](t, resp)
	if !body.PendingDeletion {
		t.Fatal("PendingDeletion = false, want true")
	}
}

// TestMiddleware_PendingDeletionLockout_RefusesOwnerPOSTOnDeletionRoute
// pins isPracticeDeletionRoute's own stated exclusion: POST is not one
// of decision 3's sanctioned actions, even for an Owner on the exact
// .../deletion path -- unlike GET/DELETE, it stays refused here rather
// than reaching InitiateHandler's own (friendlier, but not this
// ticket's word) 409 for an already-pending Practice.
func TestMiddleware_PendingDeletionLockout_RefusesOwnerPOSTOnDeletionRoute(t *testing.T) {
	db := testdb.New(t)
	const identityUID = "owner-pending-post-refused"
	_, practiceID := seedOwnerMembership(t, db, identityUID)
	markPracticePendingDeletion(t, db, practiceID)

	srv := deletionServer(db)
	defer srv.Close()
	session := authntest.SeedSessionWithSecondFactor(t, db.App, identityUID, true)

	resp := reqtest.Post(t, srv.URL+"/api/practices/"+practiceID+"/deletion", reqtest.Session(session))
	defer resp.Body.Close()
	reqtest.RequireStatus(t, resp, http.StatusForbidden)

	body := apierrtest.Decode(t, resp)
	if body.Code != apierr.CodePracticePendingDeletion {
		t.Fatalf("code = %q, want %s", body.Code, apierr.CodePracticePendingDeletion)
	}
}

func TestMiddleware_DeletedPracticeReadsAsNoMembership(t *testing.T) {
	db := testdb.New(t)
	const identityUID = "owner-finalized-blocked"
	_, practiceID := seedOwnerMembership(t, db, identityUID)
	if _, err := db.Admin.ExecContext(t.Context(),
		`UPDATE practices SET deletion_requested_at = now(), deleted_at = now() WHERE id = $1`, practiceID,
	); err != nil {
		t.Fatalf("mark deleted: %v", err)
	}

	srv := deletionServer(db)
	defer srv.Close()
	session := authntest.SeedSessionWithSecondFactor(t, db.App, identityUID, true)

	resp := reqtest.Get(t, srv.URL+"/api/practices/"+practiceID+"/deletion", reqtest.Session(session))
	defer resp.Body.Close()
	reqtest.RequireStatus(t, resp, http.StatusForbidden)

	body := apierrtest.Decode(t, resp)
	if body.Code == apierr.CodePracticePendingDeletion {
		t.Fatal("a finalized Practice must read as no membership, not as pending-deletion")
	}
}

// A client that hangs up once its request is handled still leaves its
// activity stamp: the stamp runs on a context the request's cancel does not
// reach.
func TestMiddleware_ActivityStampOutlivesADisconnectedClient(t *testing.T) {
	db := testdb.New(t)
	const identityUID = "staff-who-hangs-up"
	practiceID, staffID := testdb.SeedStaffAtNewPractice(t, db, identityUID, []string{doulaRole}, employeeType)
	session := authntest.SeedSession(t, db.App, identityUID)

	ctx, hangUp := context.WithCancel(t.Context())
	defer hangUp()
	mux := http.NewServeMux()
	mux.Handle("/practices/{practiceId}/ping", staffauth.Middleware(db.App)(
		http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusOK)
			hangUp()
		}),
	))
	req := httptest.NewRequestWithContext(ctx, http.MethodGet, "/practices/"+practiceID+"/ping", nil)
	authntest.AddSessionCookie(req, session)
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	lastPractice := testdb.Scalar[sql.NullString](t, db.Admin, `SELECT last_practice_id::text FROM staff WHERE id = $1`, staffID)
	if lastPractice.String != practiceID {
		t.Fatalf("last_practice_id = %q, want %q: the stamp was lost with the client", lastPractice.String, practiceID)
	}
}
