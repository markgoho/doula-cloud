package staffinvite_test

import (
	"strings"
	"testing"
	"time"

	"doula-cloud/api/internal/mail"
	"doula-cloud/api/internal/mailsuppress"
	"doula-cloud/api/internal/testdb"
)

// The Staff invitation is the non-portal-invite outbox #733 proves the
// consequence against: it is Platform voice, it sends pre-account, and
// nothing in this package knows the suppression list exists -- the guard
// arrives entirely through the mail.Sender main wraps once (ADR-0029).
// If it works here it works for the other nine kinds, which reach
// Mailgun through that same wrapped Sender.
func TestWorker_ProcessPending_SuppressedAddressDeadLettersWithoutSending(t *testing.T) {
	db := testdb.New(t)
	practiceID := seedPractice(t, db, "Suppressed Invite Practice")
	invitationID := seedPracticeInvitation(t, db, practiceID, testInvitedAddress)
	const token = "22222222-2222-2222-2222-222222222222"
	outboxID := seedOutboxRow(t, db, invitationID, token, 0, time.Now().Add(-time.Minute))

	if err := mailsuppress.Record(t.Context(), db.App, testInvitedAddress, mailsuppress.CauseComplaint, "evt-1"); err != nil {
		t.Fatalf("record suppression: %v", err)
	}

	mailgun := &mail.FakeSender{}
	runWorker(t, db, newTestWorker(mailsuppress.Sender{Inner: mailgun, DB: db.App}))

	if len(mailgun.Sent()) != 0 {
		t.Fatalf("handed %d messages to Mailgun, want 0", len(mailgun.Sent()))
	}
	status, _, inviteToken := outboxRowState(t, db, outboxID)
	if status != "dead_lettered" {
		t.Fatalf("status = %q, want dead_lettered (no retry for a suppressed address)", status)
	}
	if inviteToken.Valid {
		t.Fatalf("invite_token = %v, want NULL once the row is terminal", inviteToken.String)
	}
}

// The same row sends normally once nothing suppresses the address, so
// the dead-letter above is the suppression's doing and not the fixture's.
func TestWorker_ProcessPending_ClearedSuppressionSendsNormally(t *testing.T) {
	db := testdb.New(t)
	practiceID := seedPractice(t, db, "Cleared Suppression Practice")
	invitationID := seedPracticeInvitation(t, db, practiceID, testInvitedAddress)
	const token = "33333333-3333-3333-3333-333333333333"
	outboxID := seedOutboxRow(t, db, invitationID, token, 0, time.Now().Add(-time.Minute))

	if err := mailsuppress.Record(t.Context(), db.App, testInvitedAddress, mailsuppress.CauseBounce, "evt-1"); err != nil {
		t.Fatalf("record suppression: %v", err)
	}
	if _, err := db.Admin.ExecContext(t.Context(),
		`UPDATE email_suppressions SET cleared_at = now() WHERE address = $1`, strings.ToLower(testInvitedAddress),
	); err != nil {
		t.Fatalf("clear suppression: %v", err)
	}

	mailgun := &mail.FakeSender{}
	runWorker(t, db, newTestWorker(mailsuppress.Sender{Inner: mailgun, DB: db.App}))

	if len(mailgun.Sent()) != 1 {
		t.Fatalf("delivered %d messages, want 1", len(mailgun.Sent()))
	}
	status, _, _ := outboxRowState(t, db, outboxID)
	if status != testOutboxStatusSent {
		t.Fatalf("status = %q, want %s", status, testOutboxStatusSent)
	}
}
