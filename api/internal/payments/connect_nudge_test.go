package payments_test

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"doula-cloud/api/internal/apierr"
	"doula-cloud/api/internal/fixture"
	"doula-cloud/api/internal/mail"
	"doula-cloud/api/internal/payments"
	"doula-cloud/api/internal/reqtest"
	"doula-cloud/api/internal/tasknudge"
	"doula-cloud/api/internal/testdb"
)

const (
	connectNudgePath          = "/payments/connect/nudge"
	connectNudgeSubject       = "DoulaCloud: your Practice still has to connect Stripe"
	connectNudgeActionName    = "stripe_connect_nudge_requested"
	testConnectNudgeAccountID = "acct_already_connected"
)

func postConnectNudge(t *testing.T, srv *httptest.Server, session, practiceID string) *http.Response {
	t.Helper()
	resp := reqtest.Post(t, srv.URL+"/api/practices/"+practiceID+connectNudgePath, reqtest.Session(session))
	return resp
}

func newTestConnectNudgeWorker(sender mail.Sender) payments.ConnectNudgeWorker {
	return payments.ConnectNudgeWorker{Sender: sender, Now: time.Now, AppBaseURL: testPayoutAppBaseURL, From: testOutboxFrom, ReplyTo: testOutboxReplyTo}
}

func connectNudgeRowCount(t *testing.T, db *testdb.DB, practiceID string) int {
	t.Helper()
	count := testdb.Scalar[int](t, db.Admin, `SELECT count(*) FROM connect_nudge_outbox WHERE practice_id = $1`, practiceID)
	return count
}

// backdateConnectNudge moves a Practice's existing nudge rows back by
// days, which is how a test on the real clock reaches the far side of the
// cooldown. The edge itself is TestPostConnectNudgeHandler_CooldownEdgeOnTheRequestClock's.
func backdateConnectNudge(t *testing.T, db *testdb.DB, practiceID string, days int) {
	t.Helper()
	if _, err := db.Admin.ExecContext(t.Context(),
		`UPDATE connect_nudge_outbox SET created_at = created_at - make_interval(days => $2), status = 'sent', sent_at = now()
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

// nudge asks the Practice's Owners to connect Stripe as the Staff member at
// the screen, and requires that the ask is accepted.
func (e *env) nudge() {
	e.T.Helper()
	e.expect(http.MethodPost, connectNudgePath, http.StatusAccepted)
}

// nudgeEnv is an Admin at a Practice with a recording Enqueuer, so a test
// can assert on ADR-0013's nudge as well as on the outbox row -- the one
// thing the no-op Enqueuer cannot show.
func nudgeEnv(t *testing.T) (*env, *tasknudge.FakeEnqueuer) {
	t.Helper()
	enq := &tasknudge.FakeEnqueuer{}
	return newEnv(t, signedInAs(fixture.Admin()), withEnqueuer(enq)), enq
}

// TestPostConnectNudgeHandler_QueuesRecordsAndNudges is #917's happy
// path in one place, because the three things it asserts are one act:
// the Notification is queued, the Activity log says who asked, and
// ADR-0013's nudge fires so the mail does not wait on Cloud Scheduler's
// cadence.
func TestPostConnectNudgeHandler_QueuesRecordsAndNudges(t *testing.T) {
	e, enq := nudgeEnv(t)

	e.nudge()

	var requestedBy, rowStatus string
	if err := e.DB.Admin.QueryRowContext(t.Context(),
		`SELECT requested_by_staff_id, status FROM connect_nudge_outbox WHERE practice_id = $1`, e.Practice.ID,
	).Scan(&requestedBy, &rowStatus); err != nil {
		t.Fatalf("query connect_nudge_outbox: %v", err)
	}
	if requestedBy != e.Staff.ID {
		t.Fatalf("requested_by_staff_id = %q, want the Admin who asked (%q)", requestedBy, e.Staff.ID)
	}
	if rowStatus != testPayoutStatusPending {
		t.Fatalf("status = %q, want %s", rowStatus, testPayoutStatusPending)
	}
	actor := testdb.Scalar[string](t, e.DB.Admin, `SELECT actor_staff_id FROM activity WHERE practice_id = $1 AND action = $2`,
		e.Practice.ID, connectNudgeActionName)
	if actor != e.Staff.ID {
		t.Fatalf("activity actor = %q, want %q", actor, e.Staff.ID)
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
	e, _ := nudgeEnv(t)
	e.nudge()

	out := reqtest.Expect[apierr.APIError](t, http.MethodPost, e.PracticeURL(connectNudgePath), http.StatusConflict, e.As(e.Colleague(fixture.Admin())))

	if out.Message != payments.MsgConnectNudgeTooSoon {
		t.Fatalf("refusal = %q, want %q", out.Message, payments.MsgConnectNudgeTooSoon)
	}
	if got := connectNudgeRowCount(t, e.DB, e.Practice.ID); got != 1 {
		t.Fatalf("rows = %d, want 1 -- the cooldown must not queue a second Notification", got)
	}
}

// TestPostConnectNudgeHandler_AllowsAnotherAskOnceTheCooldownHasPassed is
// the other half of the bound, and the reason ADR-0035 chose a cooldown
// over #343's "one per episode": a colleague chasing again a fortnight
// later is a reasonable thing to want, and "once, ever" would refuse it
// forever.
func TestPostConnectNudgeHandler_AllowsAnotherAskOnceTheCooldownHasPassed(t *testing.T) {
	e, _ := nudgeEnv(t)
	e.nudge()
	backdateConnectNudge(t, e.DB, e.Practice.ID, payments.ConnectNudgeCooldownDays+1)

	e.nudge()

	if got := connectNudgeRowCount(t, e.DB, e.Practice.ID); got != 2 {
		t.Fatalf("rows = %d, want 2", got)
	}
}

// TestPostConnectNudgeHandler_Refusals pins ADR-0035's boundary: the
// condition, not the role, for a Practice that already connected (the
// ordinary way to meet this is an Owner finishing onboarding in another
// tab while this screen still shows the older answer), and ADR-0008's
// read table for a Doula, who cannot read Stripe Connect state and so
// cannot tell an Owner about it either. Neither queues a Notification.
func TestPostConnectNudgeHandler_Refusals(t *testing.T) {
	for _, tc := range []struct {
		name        string
		staff       fixture.StaffOption
		connected   bool
		want        int
		wantMessage string
	}{
		{"a Practice that already connected", fixture.Admin(), true, http.StatusConflict, payments.MsgConnectNudgeAlreadyConnected},
		{caseDoula, fixture.Doula(), false, http.StatusForbidden, ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			e := newEnv(t, signedInAs(tc.staff))
			if tc.connected {
				setConnectAccountID(t, e.DB, e.Practice.ID, testConnectNudgeAccountID)
			}

			out := e.postRefused(connectNudgePath, tc.want)

			if tc.wantMessage != "" && out.Message != tc.wantMessage {
				t.Fatalf("refusal = %q, want %q", out.Message, tc.wantMessage)
			}
			if got := connectNudgeRowCount(t, e.DB, e.Practice.ID); got != 0 {
				t.Fatalf("rows = %d, want 0", got)
			}
		})
	}
}

// TestConnectNudgeWorker_MailsEveryOwnerAndNamesNobody covers both the
// recipient rule ADR-0035 reuses from #343 and ADR-0009's content rule:
// every current Owner is mailed, no non-Owner is, and the body carries
// no Practice name and no sender name.
func TestConnectNudgeWorker_MailsEveryOwnerAndNamesNobody(t *testing.T) {
	const practiceName = "Willow Birth Collective"
	e, _ := nudgeEnv(t)
	if _, err := e.DB.Admin.ExecContext(t.Context(),
		`UPDATE practices SET name = $2 WHERE id = $1`, e.Practice.ID, practiceName,
	); err != nil {
		t.Fatalf("name the practice: %v", err)
	}
	e.Colleague(fixture.Owner(), fixture.StaffUID("connect-nudge-owner-one"))
	e.Colleague(fixture.Owner(), fixture.StaffUID("connect-nudge-owner-two"))
	e.Colleague(fixture.Doula(), fixture.StaffUID("connect-nudge-bystander"))
	e.nudge()

	sender := &mail.FakeSender{}
	runWorker(t, e.DB, newTestConnectNudgeWorker(sender))

	sent := sender.Sent()
	if len(sent) != 2 {
		t.Fatalf("sent %d messages, want 2 (one per Owner, and no Admin or Doula)", len(sent))
	}
	wantLink := testPayoutAppBaseURL + "/practices/" + e.Practice.ID + "/settings/payments"
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
		if strings.Contains(msg.Text, e.Staff.UID) {
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
	notified := testdb.Scalar[int](t, e.DB.Admin, `SELECT cardinality(notified_owner_staff_ids) FROM connect_nudge_outbox WHERE practice_id = $1`,
		e.Practice.ID)
	if notified != 2 {
		t.Fatalf("notified_owner_staff_ids holds %d ids, want 2 (the two Owners mailed)", notified)
	}
	everyIDIsAnOwner := testdb.Scalar[bool](t, e.DB.Admin, `SELECT NOT EXISTS (
			SELECT 1 FROM connect_nudge_outbox o, unnest(o.notified_owner_staff_ids) AS notified_id
			WHERE o.practice_id = $1
			  AND NOT EXISTS (
				SELECT 1 FROM practice_memberships pm
				WHERE pm.practice_id = $1 AND pm.staff_id = notified_id AND 'owner' = ANY(pm.roles)
			  )
		 )`, e.Practice.ID)
	if !everyIDIsAnOwner {
		t.Fatal("notified_owner_staff_ids names somebody who is not an Owner of this Practice")
	}
}

// TestConnectNudgeWorker_SendsNothingWhenThereIsNobodyToTell proves a row
// is marked sent with nothing mailed in the two cases the send finds
// nothing to do. The first is the send-time recheck: a retry can be a day
// after the press (ADR-0010's backoff), and no Owner should be told to do
// something already done. The second mirrors the payout worker's own case:
// SendAll marks a row sent when there is nobody left to notify, rather
// than retrying an empty recipient list five times.
func TestConnectNudgeWorker_SendsNothingWhenThereIsNobodyToTell(t *testing.T) {
	for _, tc := range []struct {
		name  string
		setup func(*env)
	}{
		{"the Practice connected before the send", func(e *env) {
			e.Colleague(fixture.Owner())
			e.nudge()
			setConnectAccountID(e.T, e.DB, e.Practice.ID, testConnectNudgeAccountID)
		}},
		{"the Practice has no Owner", func(e *env) { e.nudge() }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			e, _ := nudgeEnv(t)
			tc.setup(e)

			sender := &mail.FakeSender{}
			runWorker(t, e.DB, newTestConnectNudgeWorker(sender))

			if len(sender.Sent()) != 0 {
				t.Fatalf("sent %d messages, want 0", len(sender.Sent()))
			}
			status := testdb.Scalar[string](t, e.DB.Admin, `SELECT status FROM connect_nudge_outbox WHERE practice_id = $1`, e.Practice.ID)
			if status != testOutboxStatusSkipped {
				t.Fatalf("status = %q, want %s (marked skipped with nothing to mail)", status, testOutboxStatusSkipped)
			}
		})
	}
}

// TestConnectNudgeWorker_ZeroOwnersRecordsAnEmptyRoster proves nobody
// addressed is an empty array rather than NULL: a different fact from a
// row no attempt has reached yet.
func TestConnectNudgeWorker_ZeroOwnersRecordsAnEmptyRoster(t *testing.T) {
	e, _ := nudgeEnv(t)
	e.nudge()

	runWorker(t, e.DB, newTestConnectNudgeWorker(&mail.FakeSender{}))

	notified := testdb.Scalar[int](t, e.DB.Admin, `SELECT cardinality(notified_owner_staff_ids) FROM connect_nudge_outbox WHERE practice_id = $1`,
		e.Practice.ID)
	if notified != 0 {
		t.Fatalf("notified_owner_staff_ids holds %d ids, want an empty array", notified)
	}
}
