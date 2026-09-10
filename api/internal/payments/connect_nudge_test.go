package payments_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"doula-cloud/api/internal/authntest"
	"doula-cloud/api/internal/idempotency"
	"doula-cloud/api/internal/mail"
	"doula-cloud/api/internal/outbox"
	"doula-cloud/api/internal/payments"
	"doula-cloud/api/internal/staffauth"
	"doula-cloud/api/internal/tasknudge"
	"doula-cloud/api/internal/testdb"
)

const (
	connectNudgePath          = "/payments/connect/nudge"
	connectNudgeSubject       = "Doula Cloud: your Practice still has to connect Stripe"
	connectNudgeActionName    = "stripe_connect_nudge_requested"
	testConnectNudgeAccountID = "acct_already_connected"
)

// newConnectNudgeServer mounts this package's whole surface with a
// recording Enqueuer, so a test can assert on ADR-0013's nudge as well as
// on the outbox row -- the one thing newConnectServer's NoOpEnqueuer
// cannot show.
func newConnectNudgeServer(t *testing.T, db *testdb.DB, uid string) (srv *httptest.Server, session string, enq *tasknudge.FakeEnqueuer) {
	t.Helper()
	mux := http.NewServeMux()
	g := staffauth.NewGatedRouter(mux, db.App)
	ir := idempotency.NewRouter(g, db.App)
	enq = &tasknudge.FakeEnqueuer{}
	// The nudge never calls Stripe -- not_connected is a null column, not
	// a retrieve -- so the fake client is here only because Mount asks
	// for one.
	payments.Mount(g, ir, payments.NewFakeClient(), enq)
	return httptest.NewServer(mux), authntest.SeedSession(t, db.App, uid), enq
}

func postConnectNudge(t *testing.T, srv *httptest.Server, session, practiceID string) *http.Response {
	t.Helper()
	req, err := http.NewRequestWithContext(t.Context(), http.MethodPost,
		srv.URL+"/api/practices/"+practiceID+connectNudgePath, nil)
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

// nudgeRefusalMessage reads docs/api-design.md section 7's structured
// body back as the sentence a person actually meets on the screen.
func nudgeRefusalMessage(t *testing.T, resp *http.Response) string {
	t.Helper()
	var body struct {
		Message string `json:"message"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatalf("decode refusal: %v", err)
	}
	return body.Message
}

func newTestConnectNudgeWorker(sender mail.Sender) payments.ConnectNudgeWorker {
	return payments.ConnectNudgeWorker{Mailer: outbox.Mailer{Sender: sender, Now: time.Now, AppBaseURL: testPayoutAppBaseURL, From: testOutboxFrom, ReplyTo: testOutboxReplyTo}}
}

// runConnectNudgeWorker mirrors runPayoutWorker: it sets the trusted
// session var outbox.ProcessHandler would otherwise set after its own
// secret check, and commits.
func runConnectNudgeWorker(t *testing.T, db *testdb.DB, w payments.ConnectNudgeWorker) {
	t.Helper()
	tx, err := db.App.BeginTx(t.Context(), nil)
	if err != nil {
		t.Fatalf("begin: %v", err)
	}
	defer tx.Rollback()
	if _, err := tx.ExecContext(t.Context(), `SELECT set_config('app.notification_worker_trusted', 'true', true)`); err != nil {
		t.Fatalf("set trusted session var: %v", err)
	}
	if err := w.ProcessPending(t.Context(), tx); err != nil {
		t.Fatalf("ProcessPending: %v", err)
	}
	if err := tx.Commit(); err != nil {
		t.Fatalf("commit: %v", err)
	}
}

func connectNudgeRowCount(t *testing.T, db *testdb.DB, practiceID string) int {
	t.Helper()
	var count int
	if err := db.Admin.QueryRowContext(t.Context(),
		`SELECT count(*) FROM connect_nudge_outbox WHERE practice_id = $1`, practiceID,
	).Scan(&count); err != nil {
		t.Fatalf("count connect_nudge_outbox: %v", err)
	}
	return count
}

// backdateConnectNudge moves a Practice's existing nudge rows back by
// days, which is how a test reaches the far side of the cooldown without
// a clock of its own -- the bound is evaluated against Postgres's now().
func backdateConnectNudge(t *testing.T, db *testdb.DB, practiceID string, days int) {
	t.Helper()
	if _, err := db.Admin.ExecContext(t.Context(),
		`UPDATE connect_nudge_outbox SET created_at = created_at - make_interval(days => $2), status = 'sent'
		 WHERE practice_id = $1`, practiceID, days,
	); err != nil {
		t.Fatalf("backdate connect_nudge_outbox: %v", err)
	}
}

func setConnectAccountID(t *testing.T, db *testdb.DB, practiceID, accountID string) {
	t.Helper()
	if _, err := db.Admin.ExecContext(t.Context(),
		`UPDATE practices SET stripe_connect_account_id = $2 WHERE id = $1`, practiceID, accountID,
	); err != nil {
		t.Fatalf("set connect account id: %v", err)
	}
}

// TestPostConnectNudgeHandler_QueuesRecordsAndNudges is #917's happy
// path in one place, because the three things it asserts are one act:
// the Notification is queued, the Activity log says who asked, and
// ADR-0013's nudge fires so the mail does not wait on Cloud Scheduler's
// cadence.
func TestPostConnectNudgeHandler_QueuesRecordsAndNudges(t *testing.T) {
	db := testdb.New(t)
	const uid = "connect-nudge-admin"
	practiceID, staffID := testdb.SeedStaffAtNewPractice(t, db, uid, []string{adminRole}, "employee")

	srv, session, enq := newConnectNudgeServer(t, db, uid)
	defer srv.Close()

	resp := postConnectNudge(t, srv, session, practiceID)
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusAccepted {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusAccepted)
	}

	var requestedBy, rowStatus string
	if err := db.Admin.QueryRowContext(t.Context(),
		`SELECT requested_by_staff_id, status FROM connect_nudge_outbox WHERE practice_id = $1`, practiceID,
	).Scan(&requestedBy, &rowStatus); err != nil {
		t.Fatalf("query connect_nudge_outbox: %v", err)
	}
	if requestedBy != staffID {
		t.Fatalf("requested_by_staff_id = %q, want the Admin who asked (%q)", requestedBy, staffID)
	}
	if rowStatus != testPayoutStatusPending {
		t.Fatalf("status = %q, want %s", rowStatus, testPayoutStatusPending)
	}

	var actor string
	if err := db.Admin.QueryRowContext(t.Context(),
		`SELECT actor_staff_id FROM activity WHERE practice_id = $1 AND action = $2`,
		practiceID, connectNudgeActionName,
	).Scan(&actor); err != nil {
		t.Fatalf("query activity: %v", err)
	}
	if actor != staffID {
		t.Fatalf("activity actor = %q, want %q", actor, staffID)
	}

	if got := enq.Calls(); len(got) != 1 || got[0] != tasknudge.ConnectNudge {
		t.Fatalf("nudge calls = %v, want one %s", got, tasknudge.ConnectNudge)
	}
}

// TestPostConnectNudgeHandler_RefusesASecondAskInsideTheCooldown is
// #917's own acceptance criterion that repeats are bounded by a recorded
// rule rather than by the sender's restraint -- and that the bound is
// across senders, not per sender, so a second Admin pressing it an hour
// later mails nobody a second time.
func TestPostConnectNudgeHandler_RefusesASecondAskInsideTheCooldown(t *testing.T) {
	db := testdb.New(t)
	const firstUID = "connect-nudge-first"
	const secondUID = "connect-nudge-second"
	practiceID, _ := testdb.SeedStaffAtNewPractice(t, db, firstUID, []string{adminRole}, "employee")
	testdb.SeedStaffAtPractice(t, db, practiceID, secondUID, []string{adminRole}, "employee")

	srv, session, _ := newConnectNudgeServer(t, db, firstUID)
	defer srv.Close()

	first := postConnectNudge(t, srv, session, practiceID)
	_ = first.Body.Close()
	if first.StatusCode != http.StatusAccepted {
		t.Fatalf("first status = %d, want %d", first.StatusCode, http.StatusAccepted)
	}

	secondSession := authntest.SeedSession(t, db.App, secondUID)
	second := postConnectNudge(t, srv, secondSession, practiceID)
	defer func() { _ = second.Body.Close() }()
	if second.StatusCode != http.StatusConflict {
		t.Fatalf("second status = %d, want %d", second.StatusCode, http.StatusConflict)
	}
	if got := nudgeRefusalMessage(t, second); got != payments.MsgConnectNudgeTooSoon {
		t.Fatalf("refusal = %q, want %q", got, payments.MsgConnectNudgeTooSoon)
	}
	if got := connectNudgeRowCount(t, db, practiceID); got != 1 {
		t.Fatalf("rows = %d, want 1 -- the cooldown must not queue a second Notification", got)
	}
}

// TestPostConnectNudgeHandler_AllowsAnotherAskOnceTheCooldownHasPassed is
// the other half of the bound, and the reason ADR-0035 chose a cooldown
// over #343's "one per episode": a colleague chasing again a fortnight
// later is a reasonable thing to want, and "once, ever" would refuse it
// forever.
func TestPostConnectNudgeHandler_AllowsAnotherAskOnceTheCooldownHasPassed(t *testing.T) {
	db := testdb.New(t)
	const uid = "connect-nudge-cooldown-passed"
	practiceID, _ := testdb.SeedStaffAtNewPractice(t, db, uid, []string{adminRole}, "employee")

	srv, session, _ := newConnectNudgeServer(t, db, uid)
	defer srv.Close()

	first := postConnectNudge(t, srv, session, practiceID)
	_ = first.Body.Close()
	backdateConnectNudge(t, db, practiceID, payments.ConnectNudgeCooldownDays+1)

	second := postConnectNudge(t, srv, session, practiceID)
	defer func() { _ = second.Body.Close() }()
	if second.StatusCode != http.StatusAccepted {
		t.Fatalf("second status = %d, want %d once the cooldown has passed", second.StatusCode, http.StatusAccepted)
	}
	if got := connectNudgeRowCount(t, db, practiceID); got != 2 {
		t.Fatalf("rows = %d, want 2", got)
	}
}

// TestPostConnectNudgeHandler_RefusesAPracticeThatAlreadyConnected pins
// the boundary ADR-0035 says actually matters: the condition, not the
// role. The ordinary way to meet this is an Owner finishing onboarding
// in another tab while this screen still shows the older answer.
func TestPostConnectNudgeHandler_RefusesAPracticeThatAlreadyConnected(t *testing.T) {
	db := testdb.New(t)
	const uid = "connect-nudge-already-connected"
	practiceID, _ := testdb.SeedStaffAtNewPractice(t, db, uid, []string{adminRole}, "employee")
	setConnectAccountID(t, db, practiceID, testConnectNudgeAccountID)

	srv, session, _ := newConnectNudgeServer(t, db, uid)
	defer srv.Close()

	resp := postConnectNudge(t, srv, session, practiceID)
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusConflict {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusConflict)
	}
	if got := nudgeRefusalMessage(t, resp); got != payments.MsgConnectNudgeAlreadyConnected {
		t.Fatalf("refusal = %q, want %q", got, payments.MsgConnectNudgeAlreadyConnected)
	}
	if got := connectNudgeRowCount(t, db, practiceID); got != 0 {
		t.Fatalf("rows = %d, want 0", got)
	}
}

// TestPostConnectNudgeHandler_RefusesADoula holds ADR-0008's read table
// at the boundary: a Doula cannot read Stripe Connect state, so she
// cannot tell an Owner about it either.
func TestPostConnectNudgeHandler_RefusesADoula(t *testing.T) {
	db := testdb.New(t)
	const uid = "connect-nudge-doula"
	practiceID, _ := testdb.SeedStaffAtNewPractice(t, db, uid, []string{doulaRole}, "employee")

	srv, session, _ := newConnectNudgeServer(t, db, uid)
	defer srv.Close()

	resp := postConnectNudge(t, srv, session, practiceID)
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusForbidden)
	}
	if got := connectNudgeRowCount(t, db, practiceID); got != 0 {
		t.Fatalf("rows = %d, want 0", got)
	}
}

// TestConnectNudgeWorker_MailsEveryOwnerAndNamesNobody covers both the
// recipient rule ADR-0035 reuses from #343 and ADR-0009's content rule:
// every current Owner is mailed, no non-Owner is, and the body carries
// no Practice name and no sender name.
func TestConnectNudgeWorker_MailsEveryOwnerAndNamesNobody(t *testing.T) {
	db := testdb.New(t)
	const uid = "connect-nudge-worker-admin"
	const practiceName = "Willow Birth Collective"
	practiceID, _ := testdb.SeedStaffAtNewPractice(t, db, uid, []string{adminRole}, "employee")
	if _, err := db.Admin.ExecContext(t.Context(),
		`UPDATE practices SET name = $2 WHERE id = $1`, practiceID, practiceName,
	); err != nil {
		t.Fatalf("name the practice: %v", err)
	}
	testdb.SeedStaffAtPractice(t, db, practiceID, "connect-nudge-owner-one", []string{ownerRole}, "employee")
	testdb.SeedStaffAtPractice(t, db, practiceID, "connect-nudge-owner-two", []string{ownerRole}, "employee")
	testdb.SeedStaffAtPractice(t, db, practiceID, "connect-nudge-bystander", []string{doulaRole}, "employee")

	srv, session, _ := newConnectNudgeServer(t, db, uid)
	defer srv.Close()
	resp := postConnectNudge(t, srv, session, practiceID)
	_ = resp.Body.Close()

	sender := &mail.FakeSender{}
	runConnectNudgeWorker(t, db, newTestConnectNudgeWorker(sender))

	sent := sender.Sent()
	if len(sent) != 2 {
		t.Fatalf("sent %d messages, want 2 (one per Owner, and no Admin or Doula)", len(sent))
	}
	wantLink := testPayoutAppBaseURL + "/practices/" + practiceID + "/settings/payments"
	wantRecipients := map[string]bool{
		"connect-nudge-owner-one@example.com": false,
		"connect-nudge-owner-two@example.com": false,
	}
	for _, msg := range sent {
		if msg.Subject != connectNudgeSubject {
			t.Fatalf("subject = %q, want %q", msg.Subject, connectNudgeSubject)
		}
		if _, ok := wantRecipients[msg.To]; !ok {
			t.Fatalf("To = %q, want one of the two Owners", msg.To)
		}
		wantRecipients[msg.To] = true
		if !strings.Contains(msg.Text, wantLink) {
			t.Fatalf("body %q does not contain link %q", msg.Text, wantLink)
		}
		// ADR-0009/ADR-0011: no Practice name anywhere, and no sender
		// name either -- who asked lives in the Activity log, not here.
		if strings.Contains(msg.Text, practiceName) || strings.Contains(msg.Subject, practiceName) {
			t.Fatalf("Notification names the Practice: %q / %q", msg.Subject, msg.Text)
		}
		if strings.Contains(msg.Text, uid) {
			t.Fatalf("Notification names the sender: %q", msg.Text)
		}
	}
	for to, got := range wantRecipients {
		if !got {
			t.Fatalf("Owner %q was never mailed", to)
		}
	}

	// #917's "to whom": the roster this attempt resolved, written onto the
	// row so it is still answerable after the roster moves.
	var notified int
	if err := db.Admin.QueryRowContext(t.Context(),
		`SELECT cardinality(notified_owner_staff_ids) FROM connect_nudge_outbox WHERE practice_id = $1`,
		practiceID,
	).Scan(&notified); err != nil {
		t.Fatalf("query notified_owner_staff_ids: %v", err)
	}
	if notified != 2 {
		t.Fatalf("notified_owner_staff_ids holds %d ids, want 2 (the two Owners mailed)", notified)
	}
	var everyIDIsAnOwner bool
	if err := db.Admin.QueryRowContext(t.Context(),
		`SELECT NOT EXISTS (
			SELECT 1 FROM connect_nudge_outbox o, unnest(o.notified_owner_staff_ids) AS notified_id
			WHERE o.practice_id = $1
			  AND NOT EXISTS (
				SELECT 1 FROM practice_memberships pm
				WHERE pm.practice_id = $1 AND pm.staff_id = notified_id AND 'owner' = ANY(pm.roles)
			  )
		 )`, practiceID,
	).Scan(&everyIDIsAnOwner); err != nil {
		t.Fatalf("check notified ids are Owners: %v", err)
	}
	if !everyIDIsAnOwner {
		t.Fatal("notified_owner_staff_ids names somebody who is not an Owner of this Practice")
	}
}

// TestConnectNudgeWorker_SkipsAPracticeThatConnectedBeforeTheSend is the
// send-time recheck: a retry can be a day after the press (ADR-0010's
// backoff), and no Owner should be told to do something already done.
func TestConnectNudgeWorker_SkipsAPracticeThatConnectedBeforeTheSend(t *testing.T) {
	db := testdb.New(t)
	const uid = "connect-nudge-raced"
	practiceID, _ := testdb.SeedStaffAtNewPractice(t, db, uid, []string{adminRole}, "employee")
	testdb.SeedStaffAtPractice(t, db, practiceID, "connect-nudge-raced-owner", []string{ownerRole}, "employee")

	srv, session, _ := newConnectNudgeServer(t, db, uid)
	defer srv.Close()
	resp := postConnectNudge(t, srv, session, practiceID)
	_ = resp.Body.Close()

	setConnectAccountID(t, db, practiceID, testConnectNudgeAccountID)

	sender := &mail.FakeSender{}
	runConnectNudgeWorker(t, db, newTestConnectNudgeWorker(sender))

	if len(sender.Sent()) != 0 {
		t.Fatalf("sent %d messages, want 0 -- the Practice connected before the send", len(sender.Sent()))
	}
	var status string
	if err := db.Admin.QueryRowContext(t.Context(),
		`SELECT status FROM connect_nudge_outbox WHERE practice_id = $1`, practiceID,
	).Scan(&status); err != nil {
		t.Fatalf("query connect_nudge_outbox: %v", err)
	}
	if status != testPayoutStatusSent {
		t.Fatalf("status = %q, want %s (marked sent with nothing to mail)", status, testPayoutStatusSent)
	}
}

// TestConnectNudgeWorker_ZeroOwnersMarksSentWithNoMail mirrors the payout
// worker's own case: SendAll marks a row sent when there is nobody left
// to notify, rather than retrying an empty recipient list five times.
func TestConnectNudgeWorker_ZeroOwnersMarksSentWithNoMail(t *testing.T) {
	db := testdb.New(t)
	const uid = "connect-nudge-ownerless"
	practiceID, _ := testdb.SeedStaffAtNewPractice(t, db, uid, []string{adminRole}, "employee")

	srv, session, _ := newConnectNudgeServer(t, db, uid)
	defer srv.Close()
	resp := postConnectNudge(t, srv, session, practiceID)
	_ = resp.Body.Close()

	sender := &mail.FakeSender{}
	runConnectNudgeWorker(t, db, newTestConnectNudgeWorker(sender))

	if len(sender.Sent()) != 0 {
		t.Fatalf("sent %d messages, want 0", len(sender.Sent()))
	}
	var status string
	if err := db.Admin.QueryRowContext(t.Context(),
		`SELECT status FROM connect_nudge_outbox WHERE practice_id = $1`, practiceID,
	).Scan(&status); err != nil {
		t.Fatalf("query connect_nudge_outbox: %v", err)
	}
	if status != testPayoutStatusSent {
		t.Fatalf("status = %q, want %s", status, testPayoutStatusSent)
	}
	// An empty array rather than NULL: nobody was addressed, which is a
	// different fact from a row no attempt has reached yet.
	var notified int
	if err := db.Admin.QueryRowContext(t.Context(),
		`SELECT cardinality(notified_owner_staff_ids) FROM connect_nudge_outbox WHERE practice_id = $1`,
		practiceID,
	).Scan(&notified); err != nil {
		t.Fatalf("query notified_owner_staff_ids: %v", err)
	}
	if notified != 0 {
		t.Fatalf("notified_owner_staff_ids holds %d ids, want an empty array", notified)
	}
}
