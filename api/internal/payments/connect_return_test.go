package payments_test

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"doula-cloud/api/internal/authntest"
	"doula-cloud/api/internal/clock"
	"doula-cloud/api/internal/idempotency"
	"doula-cloud/api/internal/payments"
	"doula-cloud/api/internal/staffauth"
	"doula-cloud/api/internal/tasknudge"
	"doula-cloud/api/internal/testdb"
)

const connectReturnActionName = "stripe_connect_form_returned"

// connectReturnInstant is the server's clock in these tests, so a test can
// prove the row's time is the clock's (#773), not the database's.
var connectReturnInstant = time.Date(2026, 10, 20, 14, 5, 0, 0, time.UTC)

// newConnectReturnServer mounts this package's whole surface behind a
// fixed clock.
func newConnectReturnServer(t *testing.T, db *testdb.DB, uid string) (srv *httptest.Server, session string) {
	t.Helper()
	mux := http.NewServeMux()
	g := staffauth.NewGatedRouter(mux, db.App)
	ir := idempotency.NewRouter(g, db.App)
	payments.Mount(g, ir, payments.NewFakeClient(), tasknudge.NoOpEnqueuer{}, db.App, testPublishableKey)
	handler := clock.Middleware(func() time.Time { return connectReturnInstant })(mux)
	return httptest.NewServer(handler), authntest.SeedSession(t, db.App, uid)
}

func postConnectReturn(t *testing.T, srv *httptest.Server, session, practiceID string) *http.Response {
	t.Helper()
	req, err := http.NewRequestWithContext(t.Context(), http.MethodPost,
		srv.URL+"/api/practices/"+practiceID+"/payments/connect/return", nil)
	if err != nil {
		t.Fatalf("build request: %v", err)
	}
	authntest.AddSessionCookie(req, session)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	return resp
}

func connectReturnCount(t *testing.T, db *testdb.DB, practiceID string) int {
	t.Helper()
	var n int
	if err := db.Admin.QueryRowContext(t.Context(),
		`SELECT count(*) FROM activity WHERE practice_id = $1 AND action = $2`, practiceID, connectReturnActionName,
	).Scan(&n); err != nil {
		t.Fatalf("count return rows: %v", err)
	}
	return n
}

// TestPostConnectReturnHandler_RecordsTheOwnerAndTheTime is #1812's third
// criterion: the return names the Practice, the Staff member and the
// time, on the server.
func TestPostConnectReturnHandler_RecordsTheOwnerAndTheTime(t *testing.T) {
	db := testdb.New(t)
	const uid = "connect-return-owner"
	practiceID, staffID := testdb.SeedStaffAtNewPractice(t, db, uid, []string{ownerRole}, "employee")
	setConnectAccountID(t, db, practiceID, "acct_returning")

	srv, session := newConnectReturnServer(t, db, uid)
	defer srv.Close()

	resp := postConnectReturn(t, srv, session, practiceID)
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusNoContent {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusNoContent)
	}

	var subjectKind, subjectID, actorKind, actorStaffID string
	var createdAt time.Time
	if err := db.Admin.QueryRowContext(t.Context(),
		`SELECT subject_kind, subject_id, actor_kind::text, actor_staff_id, created_at
		 FROM activity WHERE practice_id = $1 AND action = $2`, practiceID, connectReturnActionName,
	).Scan(&subjectKind, &subjectID, &actorKind, &actorStaffID, &createdAt); err != nil {
		t.Fatalf("query activity: %v", err)
	}
	if subjectKind != "practice" || subjectID != practiceID {
		t.Fatalf("subject = (%q, %q), want the Practice itself", subjectKind, subjectID)
	}
	if actorKind != "staff" || actorStaffID != staffID {
		t.Fatalf("actor = (%q, %q), want the Owner who came back (%q)", actorKind, actorStaffID, staffID)
	}
	if !createdAt.Equal(connectReturnInstant) {
		t.Fatalf("created_at = %v, want the server clock's %v", createdAt, connectReturnInstant)
	}
}

// TestPostConnectReturnHandler_RefusesAPracticeWithNoAccount: only a
// hand-typed address reaches the return with no Stripe account, and there
// is nothing to record.
func TestPostConnectReturnHandler_RefusesAPracticeWithNoAccount(t *testing.T) {
	db := testdb.New(t)
	const uid = "connect-return-no-account"
	practiceID, _ := testdb.SeedStaffAtNewPractice(t, db, uid, []string{ownerRole}, "employee")

	srv, session := newConnectReturnServer(t, db, uid)
	defer srv.Close()

	resp := postConnectReturn(t, srv, session, practiceID)
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusConflict {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusConflict)
	}
	if got := nudgeRefusalMessage(t, resp); got != payments.MsgConnectReturnNoAccount {
		t.Fatalf("message = %q, want %q", got, payments.MsgConnectReturnNoAccount)
	}
	if got := connectReturnCount(t, db, practiceID); got != 0 {
		t.Fatalf("return rows = %d, want 0", got)
	}
}

// TestPostConnectReturnHandler_RefusesAnAdmin: only an Owner opens the
// form, so only an Owner comes back from it.
func TestPostConnectReturnHandler_RefusesAnAdmin(t *testing.T) {
	db := testdb.New(t)
	const uid = "connect-return-admin"
	practiceID, _ := testdb.SeedStaffAtNewPractice(t, db, uid, []string{adminRole}, "employee")
	setConnectAccountID(t, db, practiceID, "acct_admin_return")

	srv, session := newConnectReturnServer(t, db, uid)
	defer srv.Close()

	resp := postConnectReturn(t, srv, session, practiceID)
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusForbidden)
	}
	if got := connectReturnCount(t, db, practiceID); got != 0 {
		t.Fatalf("return rows = %d, want 0", got)
	}
}

// reviewIntervalQuery is the read #1591 runs over the stored rows (#1812's
// fourth criterion), kept here so a test proves it gives the right
// answer. #1591 holds the same text. For each Practice whose card_payments
// became active:
//
//   - active_at: the first history row with card_payments active.
//     processed_at, not stripe_event_created_at, because a thin event's
//     statuses are true at the moment it was processed
//     (00127_stripe_connect_status_history.sql).
//   - returned_at: her first return from Stripe's form after the last row
//     that still had something outstanding, and at or before active_at.
//     Not her first return ever: a return can mean she left the form
//     early, and Stripe then still asked for more. Not her last: a reload
//     of the page while Stripe reviews records another landing.
//   - finished_at: the first row with nothing outstanding and
//     card_payments not yet active, the moment the data can prove she
//     finished the form.
//   - review_took: active_at minus her return, or minus finished_at where
//     no return was recorded.
const reviewIntervalQuery = `
WITH active AS (
    SELECT practice_id, min(processed_at) AS active_at
    FROM stripe_connect_status_history
    WHERE card_payments_status = 'active'
    GROUP BY practice_id
),
finished AS (
    SELECT h.practice_id, min(h.processed_at) AS finished_at
    FROM stripe_connect_status_history h
    JOIN active a ON a.practice_id = h.practice_id
    WHERE cardinality(h.requirements_due) = 0
      AND h.card_payments_status <> 'active'
      AND h.processed_at <= a.active_at
    GROUP BY h.practice_id
),
last_outstanding AS (
    SELECT h.practice_id, max(h.processed_at) AS outstanding_at
    FROM stripe_connect_status_history h
    JOIN active a ON a.practice_id = h.practice_id
    WHERE cardinality(h.requirements_due) > 0
      AND h.processed_at <= a.active_at
    GROUP BY h.practice_id
),
returned AS (
    SELECT act.practice_id, min(act.created_at) AS returned_at
    FROM activity act
    JOIN active a ON a.practice_id = act.practice_id
    LEFT JOIN last_outstanding o ON o.practice_id = act.practice_id
    WHERE act.subject_kind = 'practice'
      AND act.action = 'stripe_connect_form_returned'
      AND act.created_at <= a.active_at
      AND (o.outstanding_at IS NULL OR act.created_at >= o.outstanding_at)
    GROUP BY act.practice_id
)
SELECT a.practice_id, r.returned_at, f.finished_at, a.active_at,
       a.active_at - coalesce(r.returned_at, f.finished_at) AS review_took
FROM active a
LEFT JOIN returned r ON r.practice_id = a.practice_id
LEFT JOIN finished f ON f.practice_id = a.practice_id
ORDER BY a.active_at`

func seedHistory(t *testing.T, db *testdb.DB, practiceID, eventID, cardPayments string, requirements []string, processedAt time.Time) {
	t.Helper()
	if requirements == nil {
		requirements = []string{}
	}
	if _, err := db.Admin.ExecContext(t.Context(),
		`INSERT INTO stripe_connect_status_history
			(practice_id, stripe_event_id, card_payments_status, payouts_status, requirements_due, stripe_event_created_at, processed_at)
		 VALUES ($1, $2, $3, 'pending', $4, $5, $5)`,
		practiceID, eventID, cardPayments, requirements, processedAt,
	); err != nil {
		t.Fatalf("seed history: %v", err)
	}
}

func seedReturn(t *testing.T, db *testdb.DB, practiceID, staffID string, at time.Time) {
	t.Helper()
	if _, err := db.Admin.ExecContext(t.Context(),
		`INSERT INTO activity (practice_id, subject_kind, subject_id, action, diff, actor_kind, actor_staff_id, created_at)
		 VALUES ($1, 'practice', $1, $2, '{}', 'staff', $3, $4)`,
		practiceID, connectReturnActionName, staffID, at,
	); err != nil {
		t.Fatalf("seed return: %v", err)
	}
}

// TestReviewIntervalQuery_GivesBothMoments proves the query #1591 runs:
// the return wins where one was recorded, the first finished row stands
// in where none was, and neither an early exit, a reload during review,
// nor a return after activation moves her return.
func TestReviewIntervalQuery_GivesBothMoments(t *testing.T) {
	db := testdb.New(t)
	day := func(d, h int) time.Time { return time.Date(2026, 10, d, h, 0, 0, 0, time.UTC) }

	withReturn, owner := testdb.SeedStaffAtNewPractice(t, db, "review-with-return", []string{ownerRole}, "employee")
	seedReturn(t, db, withReturn, owner, day(1, 9)) // left the form early
	seedHistory(t, db, withReturn, "evt_a1", "restricted", []string{requirementMCC}, day(1, 10))
	seedReturn(t, db, withReturn, owner, day(2, 9)) // came back and finished
	seedHistory(t, db, withReturn, "evt_a2", "pending", nil, day(2, 10))
	seedReturn(t, db, withReturn, owner, day(3, 9)) // reloaded the page during review
	seedHistory(t, db, withReturn, "evt_a3", "active", nil, day(4, 9))
	seedReturn(t, db, withReturn, owner, day(5, 9)) // a later visit, after activation
	seedHistory(t, db, withReturn, "evt_a4", "active", nil, day(6, 9))

	noReturn := testdb.SeedPractice(t, db, "No Return Practice")
	seedHistory(t, db, noReturn, "evt_b1", "pending", nil, day(3, 12))
	seedHistory(t, db, noReturn, "evt_b2", "active", nil, day(5, 12))

	notActive := testdb.SeedPractice(t, db, "Still Pending Practice")
	seedHistory(t, db, notActive, "evt_c1", "pending", nil, day(3, 12))

	// review_took read as Postgres's own text, so the assertion does not
	// depend on how a driver renders an interval.
	rows, err := db.Admin.QueryContext(t.Context(),
		`SELECT practice_id, returned_at, finished_at, active_at, review_took::text FROM (`+reviewIntervalQuery+`) q`)
	if err != nil {
		t.Fatalf("run query: %v", err)
	}
	defer func() { _ = rows.Close() }()
	type result struct {
		practiceID string
		returnedAt *time.Time
		finishedAt *time.Time
		activeAt   time.Time
		reviewTook string
	}
	var got []result
	for rows.Next() {
		var r result
		if err := rows.Scan(&r.practiceID, &r.returnedAt, &r.finishedAt, &r.activeAt, &r.reviewTook); err != nil {
			t.Fatalf("scan: %v", err)
		}
		got = append(got, r)
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("iterate: %v", err)
	}

	if len(got) != 2 {
		t.Fatalf("rows = %d, want 2 (a Practice not yet active has no interval)", len(got))
	}
	first := got[0]
	if first.practiceID != withReturn || first.returnedAt == nil || !first.returnedAt.Equal(day(2, 9)) {
		t.Fatalf("first = %+v, want the return that followed the last outstanding row", first)
	}
	if !first.activeAt.Equal(day(4, 9)) || first.reviewTook != "2 days" {
		t.Fatalf("first = %+v, want active on the first active row and 2 days of review", first)
	}
	second := got[1]
	if second.practiceID != noReturn || second.returnedAt != nil || second.finishedAt == nil || !second.finishedAt.Equal(day(3, 12)) {
		t.Fatalf("second = %+v, want no return and the first finished row", second)
	}
	if second.reviewTook != "2 days" {
		t.Fatalf("second review_took = %q, want 2 days", second.reviewTook)
	}
}
