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
	"doula-cloud/api/internal/fixture"
	"doula-cloud/api/internal/idempotency"
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

// pingEnv is a Practice with the middleware in front of the routes these
// tests hit, and one Staff member at the screen (an employee Doula unless a
// test says otherwise). ping echoes the resolved Staff and Practice ids
// and confirms a usable *sql.Tx reached the handler; the deletion routes
// and the session read are #871's lockout tests' stand-ins for
// practicedeletion's and PracticeSessionHandler's own routes.
type pingEnv struct {
	*fixture.Scene
}

func newPingEnv(t *testing.T, opts ...fixture.SceneOption) *pingEnv {
	t.Helper()
	return &pingEnv{fixture.NewScene(t, pingRoutes, opts...)}
}

func pingRoutes(g *staffauth.GatedRouter, _ *idempotency.Router, app *sql.DB) {
	echo := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
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
	})
	g.Get("/api/practices/{practiceId}/ping", staffauth.AnyStaff, echo)
	g.Get("/api/practices/{practiceId}/deletion", staffauth.AnyStaff, echo)
	g.Get("/api/practices/{practiceId}/session", staffauth.AnyStaff, staffauth.PracticeSessionHandler())
	g.Write("DELETE /api/practices/{practiceId}/deletion", staffauth.Middleware(app)(echo))
	g.Write("POST /api/practices/{practiceId}/deletion", staffauth.Middleware(app)(echo))
}

// at is the route plus path of practiceID, which need not be the
// Practice the Staff member works at.
func (e *pingEnv) at(practiceID, path string) string {
	return e.URL("/api/practices/" + practiceID + path)
}

// pingURL is the ping route of the Practice the Staff member works at.
func (e *pingEnv) pingURL() string { return e.at(e.Practice.ID, "/ping") }

// refusalCode requires a 403 and returns the code of its error envelope.
func refusalCode(t *testing.T, method, url string, opts ...reqtest.Option) apierr.Code {
	t.Helper()
	return apierrtest.Expect(t, method, url, http.StatusForbidden, opts...).Code
}

// TestMiddleware_Refuses covers each way a request is turned away before
// it reaches the handler: no cookie, a Bearer token alone (#151: the
// header is never read, so a 401 is all it can mean when the Staff member
// behind it exists), a cookie naming no live session, a Practice id that
// is not a uuid, a verified uid with no staff row, a known Staff member
// who is not of this Practice, and a well-formed Practice id that does not
// exist -- setPracticeAndCheckMembership's LEFT JOIN against practices
// finds no row at all, distinct from finding the Practice but no
// membership, though both answer the same outward 403.
func TestMiddleware_Refuses(t *testing.T) {
	cases := []struct {
		name string
		want int
		// request returns the route and credentials the request carries.
		request func(e *pingEnv) (url string, opts []reqtest.Option)
	}{
		{"no credential", http.StatusUnauthorized, func(e *pingEnv) (string, []reqtest.Option) {
			return e.at(emptyUUID, "/ping"), nil
		}},
		{"a Bearer token alone", http.StatusUnauthorized, func(e *pingEnv) (string, []reqtest.Option) {
			return e.pingURL(), []reqtest.Option{reqtest.Header("Authorization", "Bearer would-verify-fine")}
		}},
		{"a session that was never issued", http.StatusUnauthorized, func(e *pingEnv) (string, []reqtest.Option) {
			return e.at(emptyUUID, "/ping"), []reqtest.Option{reqtest.Session("never-issued")}
		}},
		{"a Practice id that is not a uuid", http.StatusBadRequest, func(e *pingEnv) (string, []reqtest.Option) {
			return e.at("not-a-uuid", "/ping"), []reqtest.Option{e.SignedIn}
		}},
		{"a verified uid with no staff row", http.StatusForbidden, func(e *pingEnv) (string, []reqtest.Option) {
			session := authntest.SeedSession(e.T, e.DB.App, "unknown-uid")
			return e.at(emptyUUID, "/ping"), []reqtest.Option{reqtest.Session(session)}
		}},
		{"a Practice she is not a member of", http.StatusForbidden, func(e *pingEnv) (string, []reqtest.Option) {
			return e.at(fixture.Practice(e.T, e.DB, fixture.PracticeName("Other Practice")).ID, "/ping"), []reqtest.Option{e.SignedIn}
		}},
		{"a Practice that does not exist", http.StatusForbidden, func(e *pingEnv) (string, []reqtest.Option) {
			return e.at(emptyUUID, "/ping"), []reqtest.Option{e.SignedIn}
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			e := newPingEnv(t)
			url, opts := tc.request(e)
			reqtest.ExpectStatus(t, http.MethodGet, url, tc.want, opts...)
		})
	}
}

func TestMiddleware_Success(t *testing.T) {
	e := newPingEnv(t)

	resp := reqtest.Do(t, http.MethodGet, e.pingURL(), e.SignedIn)
	defer func() { _ = resp.Body.Close() }()

	reqtest.RequireStatus(t, resp, http.StatusOK)
	if got := resp.Header.Get("X-Staff-Id"); got != e.Staff.ID {
		t.Fatalf("X-Staff-Id = %q, want %q", got, e.Staff.ID)
	}
	if got := resp.Header.Get("X-Practice-Id"); got != e.Practice.ID {
		t.Fatalf("X-Practice-Id = %q, want %q", got, e.Practice.ID)
	}

	lastPracticeID := testdb.Scalar[string](t, e.DB.Admin, `SELECT last_practice_id FROM staff WHERE id = $1`, e.Staff.ID)
	if lastPracticeID != e.Practice.ID {
		t.Fatalf("last_practice_id = %q, want %q", lastPracticeID, e.Practice.ID)
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
	e := newPingEnv(t)
	e.Exec(`UPDATE staff SET last_practice_id = $1, last_active_at = now() WHERE id = $2`, e.Practice.ID, e.Staff.ID)
	xminBefore := testdb.Scalar[string](t, e.DB.Admin, `SELECT xmin::text FROM staff WHERE id = $1`, e.Staff.ID)

	reqtest.ExpectStatus(t, http.MethodGet, e.pingURL(), http.StatusOK, e.SignedIn)

	xminAfter := testdb.Scalar[string](t, e.DB.Admin, `SELECT xmin::text FROM staff WHERE id = $1`, e.Staff.ID)
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
	e := newPingEnv(t)
	second := fixture.Practice(t, e.DB, fixture.PracticeName("Second Practice"))
	seedMembership(t, e.DB, second.ID, e.Staff.ID)

	yesterday := time.Now().Add(-25 * time.Hour)
	e.Exec(`UPDATE staff SET last_practice_id = $1, last_active_at = $2 WHERE id = $3`, e.Practice.ID, yesterday, e.Staff.ID)

	reqtest.ExpectStatus(t, http.MethodGet, e.at(second.ID, "/ping"), http.StatusOK, e.SignedIn)

	var lastPracticeID string
	var lastActiveAt time.Time
	if err := e.DB.Admin.QueryRowContext(t.Context(),
		`SELECT last_practice_id, last_active_at FROM staff WHERE id = $1`, e.Staff.ID,
	).Scan(&lastPracticeID, &lastActiveAt); err != nil {
		t.Fatalf("read stamp: %v", err)
	}
	if lastPracticeID != second.ID {
		t.Fatalf("last_practice_id = %q, want %q", lastPracticeID, second.ID)
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
	cases := []struct {
		name     string
		staff    fixture.StaffOption
		switchOn bool
		enrolled bool
		want     int
	}{
		{"owner without second factor is admitted while the switch is off", fixture.Owner(), false, false, http.StatusOK},
		{"the switch bars an un-enrolled owner", fixture.Owner(), true, false, http.StatusForbidden},
		{"owner with second factor is admitted", fixture.Owner(), false, true, http.StatusOK},
		{"doula is never forced while the switch is off", fixture.Doula(), false, false, http.StatusOK},
		{"the switch bars an un-enrolled doula", fixture.Doula(), true, false, http.StatusForbidden},
		{"the switch admits an enrolled doula", fixture.Doula(), true, true, http.StatusOK},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			e := newPingEnv(t, fixture.SignedInAs(tc.staff))
			if tc.switchOn {
				setRequireMFA(t, e.DB, e.Practice.ID)
			}
			session := authntest.SeedSessionWithSecondFactor(t, e.DB.App, e.Staff.UID, tc.enrolled)

			if tc.want == http.StatusOK {
				reqtest.ExpectStatus(t, http.MethodGet, e.pingURL(), tc.want, reqtest.Session(session))
				return
			}
			if got := refusalCode(t, http.MethodGet, e.pingURL(), reqtest.Session(session)); got != apierr.CodeMFARequired {
				t.Fatalf("code = %q, want MFA_REQUIRED", got)
			}
		})
	}

	// TestMiddleware_MFAGate/split proves #606's own AC: a person whose
	// Membership at one Practice requires MFA and at the other does not is
	// barred from the first and admitted to the second while un-enrolled, one identity and
	// one session cookie throughout -- the fact GLOSSARY.md's per-Membership
	// role model is what forced the gate off session.CreateHandler in the
	// first place (#167's amendment).
	t.Run("split across two practices", func(t *testing.T) {
		e := newPingEnv(t, fixture.SignedInAs(fixture.Owner()))
		doulaPractice := fixture.Practice(t, e.DB, fixture.PracticeName("Second Practice"))
		seedMembership(t, e.DB, doulaPractice.ID, e.Staff.ID)
		// #1531: the split is the switch, never the role. She is Owner
		// at the Practice that has not thrown it, and only a Doula at
		// the one that has.
		setRequireMFA(t, e.DB, doulaPractice.ID)

		unenrolled := reqtest.Session(authntest.SeedSessionWithSecondFactor(t, e.DB.App, e.Staff.UID, false))
		reqtest.ExpectStatus(t, http.MethodGet, e.pingURL(), http.StatusOK, unenrolled)
		reqtest.ExpectStatus(t, http.MethodGet, e.at(doulaPractice.ID, "/ping"), http.StatusForbidden, unenrolled)

		enrolled := reqtest.Session(authntest.SeedSessionWithSecondFactor(t, e.DB.App, e.Staff.UID, true))
		reqtest.ExpectStatus(t, http.MethodGet, e.pingURL(), http.StatusOK, enrolled)
		reqtest.ExpectStatus(t, http.MethodGet, e.at(doulaPractice.ID, "/ping"), http.StatusOK, enrolled)
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
	e := newPingEnv(t)

	lastActive := func() time.Time {
		t.Helper()
		seen := testdb.Scalar[sql.NullTime](t, e.DB.Admin, `SELECT last_active_at FROM staff WHERE id = $1`, e.Staff.ID)
		if !seen.Valid {
			t.Fatal("last_active_at is unset after an authenticated request")
		}
		return seen.Time
	}

	reqtest.ExpectStatus(t, http.MethodGet, e.pingURL(), http.StatusOK, e.SignedIn)
	first := lastActive()

	reqtest.ExpectStatus(t, http.MethodGet, e.pingURL(), http.StatusOK, e.SignedIn)
	if second := lastActive(); !second.Equal(first) {
		t.Fatalf("last_active_at moved from %v to %v within the same day", first, second)
	}
}

// TestMiddleware_PendingDeletionLockout_Refuses covers what a Practice
// that has asked to be deleted turns away: every ordinary route, for an
// Owner and a non-Owner alike, and a POST to the one path the lockout
// otherwise leaves open. POST is not one of decision 3's sanctioned acts,
// even for an Owner on the exact .../deletion path -- unlike GET/DELETE,
// it stays refused here rather than reaching InitiateHandler's own
// (friendlier, but not this ticket's word) 409 for an already-pending
// Practice (isPracticeDeletionRoute's own stated exclusion).
func TestMiddleware_PendingDeletionLockout_Refuses(t *testing.T) {
	cases := []struct {
		name   string
		staff  fixture.StaffOption
		method string
		path   string
	}{
		{"an ordinary route, for the Owner", fixture.Owner(), http.MethodGet, "/ping"},
		{"an ordinary route, for a non-Owner", fixture.Doula(), http.MethodGet, "/ping"},
		{"a POST on the deletion route, for the Owner", fixture.Owner(), http.MethodPost, "/deletion"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			e := newPingEnv(t, fixture.SignedInAs(tc.staff))
			testdb.SeedPendingDeletion(t, e.DB, e.Practice.ID)

			if got := refusalCode(t, tc.method, e.at(e.Practice.ID, tc.path), e.SignedIn); got != apierr.CodePracticePendingDeletion {
				t.Fatalf("code = %q, want %s", got, apierr.CodePracticePendingDeletion)
			}
		})
	}
}

func TestMiddleware_PendingDeletionLockout_ExemptsOwnerOnDeletionRoutes(t *testing.T) {
	e := newPingEnv(t, fixture.SignedInAs(fixture.Owner()))
	testdb.SeedPendingDeletion(t, e.DB, e.Practice.ID)

	reqtest.ExpectStatus(t, http.MethodGet, e.at(e.Practice.ID, "/deletion"), http.StatusOK, e.SignedIn)
	reqtest.ExpectStatus(t, http.MethodDelete, e.at(e.Practice.ID, "/deletion"), http.StatusOK, e.SignedIn)
}

// TestMiddleware_PendingDeletionLockout_ExemptsSessionForEveryRole proves
// the one route the lockout leaves open regardless of role: app/'s
// +layout.ts load needs GET .../session to succeed for an Owner (to
// route her to the restore screen) and for a non-Owner (to show an
// accurate locked message) alike, carrying PendingDeletion: true either
// way -- unlike GET/DELETE .../deletion, which stays Owner-only.
func TestMiddleware_PendingDeletionLockout_ExemptsSessionForEveryRole(t *testing.T) {
	e := newPingEnv(t)
	testdb.SeedPendingDeletion(t, e.DB, e.Practice.ID)

	body := reqtest.Expect[staffauth.PracticeSessionResponse](t, http.MethodGet, e.at(e.Practice.ID, "/session"), http.StatusOK, e.SignedIn)
	if !body.PendingDeletion {
		t.Fatal("PendingDeletion = false, want true")
	}
}

func TestMiddleware_DeletedPracticeReadsAsNoMembership(t *testing.T) {
	e := newPingEnv(t, fixture.SignedInAs(fixture.Owner()))
	now := time.Now()
	testdb.SeedPracticeDeletion(t, e.DB, e.Practice.ID, now, now, &now)

	if got := refusalCode(t, http.MethodGet, e.at(e.Practice.ID, "/deletion"), e.SignedIn); got == apierr.CodePracticePendingDeletion {
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
