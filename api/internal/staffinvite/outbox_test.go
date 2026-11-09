package staffinvite_test

import (
	"database/sql"
	"strings"
	"testing"

	"doula-cloud/api/internal/mail"
	"doula-cloud/api/internal/staffinvite"
	"doula-cloud/api/internal/testdb"
)

func TestWorker_ProcessPending_SendsDueRowAndMarksSent(t *testing.T) {
	const token = "11111111-1111-1111-1111-111111111111"
	db, outboxID := queuedInvite(t, "Staff Invite Test Practice", token)

	sender := &mail.FakeSender{}
	runWorker(t, db, newTestWorker(sender))

	status, inviteToken := outboxRowState(t, db, outboxID)
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

func TestQueue_InsertsPendingRowThenRotationRefreshesToken(t *testing.T) {
	db := testdb.New(t)
	practiceID := testdb.SeedPractice(t, db, "Queue Practice")
	invitationID := seedPracticeInvitation(t, db, practiceID, testInvitedAddress)

	queue(t, db, invitationID, "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa")

	var id, token, status string
	if err := db.Admin.QueryRowContext(t.Context(),
		`SELECT id, invite_token::text, status FROM staff_invite_outbox WHERE invitation_id = $1`, invitationID,
	).Scan(&id, &token, &status); err != nil {
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
	queue(t, db, invitationID, "bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb")

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
	practiceID := testdb.SeedPractice(t, db, "Refresh Practice")
	pendingInvitation := seedPracticeInvitation(t, db, practiceID, testInvitedAddress)
	sentInvitation := seedPracticeInvitation(t, db, practiceID, "sent@example.com")

	tx := db.BeginAs(t)
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

	tokenOf := func(invitationID string) (token sql.NullString) {
		t.Helper()
		if err := db.Admin.QueryRowContext(t.Context(),
			`SELECT invite_token::text FROM staff_invite_outbox WHERE invitation_id = $1`, invitationID,
		).Scan(&token); err != nil {
			t.Fatalf("read row: %v", err)
		}
		return token
	}
	if pendingToken := tokenOf(pendingInvitation); pendingToken.String != rotated {
		t.Fatalf("pending token = %q, want the rotated one", pendingToken.String)
	}
	if sentToken := tokenOf(sentInvitation); sentToken.Valid {
		t.Fatalf("sent row token = %q, want it left cleared", sentToken.String)
	}
}
