package staffinvite_test

import (
	"database/sql"
	"errors"
	"strings"
	"testing"
	"time"

	"doula-cloud/api/internal/mail"
	"doula-cloud/api/internal/staffinvite"
	"doula-cloud/api/internal/testdb"
)

const (
	testOutboxStatusPending = "pending"
	testOutboxStatusSent    = "sent"
	testAppBaseURL          = "https://app.example.test"
	testSenderAddr          = "a@b.test"
	testInvitedAddress      = "invited-staff@example.com"
)

// newTestWorker builds a Worker around sender with this file's stand-in
// AppBaseURL/From/ReplyTo -- every outbox test needs one, only the
// injected Sender and (occasionally) Now vary.
func newTestWorker(sender mail.Sender) staffinvite.Worker {
	return staffinvite.Worker{Sender: sender, Now: time.Now, AppBaseURL: testAppBaseURL, From: testSenderAddr, ReplyTo: "support@b.test"}
}

// seedOutboxRow inserts a pending staff_invite_outbox row for
// invitationID with the given attempt_count/next_attempt_at/token, using
// the superuser Admin connection -- the table carries no RLS (00038), so
// db.App would work too, but Admin matches this package's seeding
// convention.
func seedOutboxRow(t *testing.T, db *testdb.DB, invitationID, token string, attemptCount int, nextAttemptAt time.Time) string {
	t.Helper()
	var id string
	if err := db.Admin.QueryRowContext(t.Context(),
		`INSERT INTO staff_invite_outbox (invitation_id, invite_token, attempt_count, next_attempt_at)
		 VALUES ($1, $2, $3, $4) RETURNING id`,
		invitationID, token, attemptCount, nextAttemptAt,
	).Scan(&id); err != nil {
		t.Fatalf("seed outbox row: %v", err)
	}
	return id
}

// runWorker begins a tx on db.App, sets the trusted session var
// outbox.ProcessHandler would otherwise set after its secret check, runs
// ProcessPending, and commits -- exercising the worker exactly as the
// handler drives it, without going through HTTP.
func runWorker(t *testing.T, db *testdb.DB, w staffinvite.Worker) {
	t.Helper()
	tx, err := db.App.BeginTx(t.Context(), nil)
	if err != nil {
		t.Fatalf("begin: %v", err)
	}
	defer func() { _ = tx.Rollback() }()
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

func outboxRowState(t *testing.T, db *testdb.DB, id string) (status string, attemptCount int, inviteToken sql.NullString) {
	t.Helper()
	if err := db.Admin.QueryRowContext(t.Context(),
		`SELECT status, attempt_count, invite_token::text FROM staff_invite_outbox WHERE id = $1`, id,
	).Scan(&status, &attemptCount, &inviteToken); err != nil {
		t.Fatalf("query outbox row: %v", err)
	}
	return status, attemptCount, inviteToken
}

func TestWorker_ProcessPending_SendsDueRowAndMarksSent(t *testing.T) {
	db := testdb.New(t)
	practiceID := seedPractice(t, db, "Staff Invite Test Practice")
	invitationID := seedPracticeInvitation(t, db, practiceID, testInvitedAddress)
	const token = "11111111-1111-1111-1111-111111111111"
	outboxID := seedOutboxRow(t, db, invitationID, token, 0, time.Now().Add(-time.Minute))

	sender := &mail.FakeSender{}
	runWorker(t, db, newTestWorker(sender))

	status, _, inviteToken := outboxRowState(t, db, outboxID)
	if status != testOutboxStatusSent {
		t.Fatalf("status = %q, want %s", status, testOutboxStatusSent)
	}
	if inviteToken.Valid {
		t.Fatalf("invite_token = %v, want NULL once sent", inviteToken.String)
	}
	sent := sender.Sent()
	if len(sent) != 1 {
		t.Fatalf("sent %d messages, want 1", len(sent))
	}
	if sent[0].To != testInvitedAddress {
		t.Fatalf("To = %q, want %q", sent[0].To, testInvitedAddress)
	}
	if sent[0].ReplyTo != "support@b.test" {
		t.Fatalf("ReplyTo = %q, want the Platform-voice support address", sent[0].ReplyTo)
	}
	wantLink := testAppBaseURL + "/accept-invite?token=" + token
	if !strings.Contains(sent[0].Text, wantLink) {
		t.Fatalf("body %q does not contain link %q", sent[0].Text, wantLink)
	}
	if strings.Contains(sent[0].Subject, "Staff Invite Test Practice") || strings.Contains(sent[0].Text, "Staff Invite Test Practice") {
		t.Fatalf("subject/body names the Practice, violating ADR-0009's content rule: %q / %q", sent[0].Subject, sent[0].Text)
	}
}

func TestWorker_ProcessPending_SkipsRowNotYetDue(t *testing.T) {
	db := testdb.New(t)
	practiceID := seedPractice(t, db, "Not Due Practice")
	invitationID := seedPracticeInvitation(t, db, practiceID, testInvitedAddress)
	outboxID := seedOutboxRow(t, db, invitationID, "22222222-2222-2222-2222-222222222222", 0, time.Now().Add(time.Hour))

	sender := &mail.FakeSender{}
	runWorker(t, db, newTestWorker(sender))

	status, _, _ := outboxRowState(t, db, outboxID)
	if status != testOutboxStatusPending {
		t.Fatalf("status = %q, want %s (not due yet)", status, testOutboxStatusPending)
	}
	if len(sender.Sent()) != 0 {
		t.Fatalf("expected no send for a not-yet-due row")
	}
}

func TestWorker_ProcessPending_RetriesOnSendFailure(t *testing.T) {
	db := testdb.New(t)
	practiceID := seedPractice(t, db, "Retry Practice")
	invitationID := seedPracticeInvitation(t, db, practiceID, testInvitedAddress)
	outboxID := seedOutboxRow(t, db, invitationID, "33333333-3333-3333-3333-333333333333", 0, time.Now().Add(-time.Minute))

	sender := &mail.FakeSender{Err: errors.New("mailgun unavailable")}
	runWorker(t, db, newTestWorker(sender))

	status, attemptCount, _ := outboxRowState(t, db, outboxID)
	if status != testOutboxStatusPending || attemptCount != 1 {
		t.Fatalf("status/attempt_count = %q/%d, want %s/1", status, attemptCount, testOutboxStatusPending)
	}
}

func TestWorker_ProcessPending_DeadLettersAfterFinalAttempt(t *testing.T) {
	db := testdb.New(t)
	practiceID := seedPractice(t, db, "Dead Letter Practice")
	invitationID := seedPracticeInvitation(t, db, practiceID, testInvitedAddress)
	// One attempt short of the schedule's length -- this failure is the
	// last one before dead-letter.
	outboxID := seedOutboxRow(t, db, invitationID, "44444444-4444-4444-4444-444444444444", 4, time.Now().Add(-time.Minute))

	sender := &mail.FakeSender{Err: errors.New("mailgun unavailable")}
	runWorker(t, db, newTestWorker(sender))

	status, attemptCount, inviteToken := outboxRowState(t, db, outboxID)
	if status != "dead_lettered" || attemptCount != 5 {
		t.Fatalf("status/attempt_count = %q/%d, want dead_lettered/5", status, attemptCount)
	}
	if inviteToken.Valid {
		t.Fatalf("invite_token = %v, want NULL once dead-lettered -- it will never be sent", inviteToken.String)
	}
}

func TestWorker_ProcessPending_ExpiredInvitationSkipsSend(t *testing.T) {
	db := testdb.New(t)
	practiceID := seedPractice(t, db, "Expired Invitation Practice")
	invitationID := seedPracticeInvitation(t, db, practiceID, testInvitedAddress)
	// The Invitation's own window lapsed before the worker got to this
	// row -- nothing has swept its status to 'expired' (no such sweep
	// exists yet), so it still reads 'pending'.
	setInvitationExpiresAt(t, db, invitationID, time.Now().Add(-time.Minute))
	outboxID := seedOutboxRow(t, db, invitationID, "77777777-7777-7777-7777-777777777777", 0, time.Now().Add(-time.Minute))

	sender := &mail.FakeSender{}
	runWorker(t, db, newTestWorker(sender))

	status, _, _ := outboxRowState(t, db, outboxID)
	if status != testOutboxStatusSent {
		t.Fatalf("status = %q, want %s", status, testOutboxStatusSent)
	}
	if len(sender.Sent()) != 0 {
		t.Fatalf("expected no mail sent for an expired invitation")
	}
}

func TestWorker_ProcessPending_AlreadyResolvedSkipsSend(t *testing.T) {
	db := testdb.New(t)
	practiceID := seedPractice(t, db, "Already Accepted Practice")
	invitationID := seedPracticeInvitation(t, db, practiceID, testInvitedAddress)
	// The invited person accepted through some other path (#316, not yet
	// built) before the worker got to this row.
	setInvitationStatus(t, db, invitationID, "accepted")
	outboxID := seedOutboxRow(t, db, invitationID, "55555555-5555-5555-5555-555555555555", 0, time.Now().Add(-time.Minute))

	sender := &mail.FakeSender{}
	runWorker(t, db, newTestWorker(sender))

	status, _, _ := outboxRowState(t, db, outboxID)
	if status != testOutboxStatusSent {
		t.Fatalf("status = %q, want %s", status, testOutboxStatusSent)
	}
	if len(sender.Sent()) != 0 {
		t.Fatalf("expected no mail sent for an already-resolved invitation")
	}
}

func TestQueue_InsertsPendingRowThenRotationRefreshesToken(t *testing.T) {
	db := testdb.New(t)
	practiceID := seedPractice(t, db, "Queue Practice")
	invitationID := seedPracticeInvitation(t, db, practiceID, testInvitedAddress)

	tx, err := db.App.BeginTx(t.Context(), nil)
	if err != nil {
		t.Fatalf("begin: %v", err)
	}
	defer func() { _ = tx.Rollback() }()

	if err := staffinvite.Queue(t.Context(), tx, invitationID, "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa"); err != nil {
		t.Fatalf("Queue: %v", err)
	}
	if err := tx.Commit(); err != nil {
		t.Fatalf("commit: %v", err)
	}

	var id, token, status string
	var attemptCount int
	if err := db.Admin.QueryRowContext(t.Context(),
		`SELECT id, invite_token::text, status, attempt_count FROM staff_invite_outbox WHERE invitation_id = $1`, invitationID,
	).Scan(&id, &token, &status, &attemptCount); err != nil {
		t.Fatalf("query queued row: %v", err)
	}
	if token != "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa" || status != testOutboxStatusPending {
		t.Fatalf("token/status = %q/%q, want the queued token/pending", token, status)
	}

	// A re-invite: rotate the token again. This must refresh the same
	// row, not insert a second one, and never mail the stale first
	// token.
	if _, err := db.Admin.ExecContext(t.Context(),
		`UPDATE staff_invite_outbox SET attempt_count = 2 WHERE id = $1`, id,
	); err != nil {
		t.Fatalf("bump attempt_count: %v", err)
	}

	tx2, err := db.App.BeginTx(t.Context(), nil)
	if err != nil {
		t.Fatalf("begin: %v", err)
	}
	defer func() { _ = tx2.Rollback() }()
	if err := staffinvite.Queue(t.Context(), tx2, invitationID, "bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb"); err != nil {
		t.Fatalf("Queue (rotation): %v", err)
	}
	if err := tx2.Commit(); err != nil {
		t.Fatalf("commit: %v", err)
	}

	var rowCount int
	if err := db.Admin.QueryRowContext(t.Context(), `SELECT count(*) FROM staff_invite_outbox WHERE invitation_id = $1`, invitationID).Scan(&rowCount); err != nil {
		t.Fatalf("count rows: %v", err)
	}
	if rowCount != 1 {
		t.Fatalf("expected exactly 1 row for the invitation after a rotation, got %d", rowCount)
	}

	var rotatedToken string
	var rotatedAttempt int
	if err := db.Admin.QueryRowContext(t.Context(),
		`SELECT invite_token::text, attempt_count FROM staff_invite_outbox WHERE id = $1`, id,
	).Scan(&rotatedToken, &rotatedAttempt); err != nil {
		t.Fatalf("query rotated row: %v", err)
	}
	if rotatedToken != "bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb" {
		t.Fatalf("token after rotation = %q, want the new token", rotatedToken)
	}
	if rotatedAttempt != 0 {
		t.Fatalf("attempt_count after rotation = %d, want reset to 0", rotatedAttempt)
	}
}

// TestRefresh_ReplacesAPendingRowsTokenOnly covers the seam the Offer
// flow (#317) uses when it rotates an Invitation's token out from under
// a Staff invitation email that has not gone out yet: the pending row
// stays mailable, and a row that has already been sent is left alone --
// its token is gone by then, and re-arming it would hand a live
// credential back to a resolved row.
func TestRefresh_ReplacesAPendingRowsTokenOnly(t *testing.T) {
	db := testdb.New(t)
	practiceID := seedPractice(t, db, "Refresh Practice")
	pendingInvitation := seedPracticeInvitation(t, db, practiceID, testInvitedAddress)
	sentInvitation := seedPracticeInvitation(t, db, practiceID, "sent@example.com")

	tx, err := db.App.BeginTx(t.Context(), nil)
	if err != nil {
		t.Fatalf("begin: %v", err)
	}
	defer func() { _ = tx.Rollback() }()
	for _, invitationID := range []string{pendingInvitation, sentInvitation} {
		if err := staffinvite.Queue(t.Context(), tx, invitationID, "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa"); err != nil {
			t.Fatalf("Queue: %v", err)
		}
	}
	if _, err := tx.ExecContext(t.Context(),
		`UPDATE staff_invite_outbox SET status = 'sent', invite_token = NULL WHERE invitation_id = $1`, sentInvitation,
	); err != nil {
		t.Fatalf("mark row sent: %v", err)
	}

	const rotated = "bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb"
	for _, invitationID := range []string{pendingInvitation, sentInvitation} {
		if err := staffinvite.Refresh(t.Context(), tx, invitationID, rotated); err != nil {
			t.Fatalf("Refresh: %v", err)
		}
	}
	if err := tx.Commit(); err != nil {
		t.Fatalf("commit: %v", err)
	}

	var pendingToken, sentToken sql.NullString
	if err := db.Admin.QueryRowContext(t.Context(),
		`SELECT invite_token::text FROM staff_invite_outbox WHERE invitation_id = $1`, pendingInvitation,
	).Scan(&pendingToken); err != nil {
		t.Fatalf("read pending row: %v", err)
	}
	if pendingToken.String != rotated {
		t.Fatalf("pending token = %q, want the rotated one", pendingToken.String)
	}
	if err := db.Admin.QueryRowContext(t.Context(),
		`SELECT invite_token::text FROM staff_invite_outbox WHERE invitation_id = $1`, sentInvitation,
	).Scan(&sentToken); err != nil {
		t.Fatalf("read sent row: %v", err)
	}
	if sentToken.Valid {
		t.Fatalf("sent row token = %q, want it left cleared", sentToken.String)
	}
}
