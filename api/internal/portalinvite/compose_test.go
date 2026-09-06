package portalinvite

import (
	"database/sql"
	"errors"
	"strings"
	"testing"
	"time"

	"doula-cloud/api/internal/outbox"
)

const (
	testComposeAppBaseURL = "https://app.example.test"
	testInvitedEmail      = "invited@example.com"
)

func runCompose(r pendingRow) (to, subject, text string, err error) {
	c := compose(outbox.Mailer{AppBaseURL: testComposeAppBaseURL})
	return c(nil, nil, r, time.Now())
}

func TestCompose_UnclaimedInviteWithEmailSendsWithLink(t *testing.T) {
	r := pendingRow{
		inviteToken: sql.NullString{String: "11111111-1111-1111-1111-111111111111", Valid: true},
		email:       sql.NullString{String: testInvitedEmail, Valid: true},
	}
	to, subject, text, err := runCompose(r)
	if err != nil {
		t.Fatalf("compose: %v", err)
	}
	if to != testInvitedEmail {
		t.Fatalf("to = %q, want the Client's email", to)
	}
	if subject != inviteSubject {
		t.Fatalf("subject = %q, want %q", subject, inviteSubject)
	}
	wantLink := testComposeAppBaseURL + "/portal/accept-invite?token=11111111-1111-1111-1111-111111111111"
	if !strings.Contains(text, wantLink) {
		t.Fatalf("text %q does not contain link %q", text, wantLink)
	}
}

// ADR-0017: a Client with no email on file is dead-lettered outright, not
// scheduled for retry -- nothing about waiting fixes a missing address.
func TestCompose_NoEmailOnFileDeadLetters(t *testing.T) {
	r := pendingRow{email: sql.NullString{Valid: false}}
	_, _, _, err := runCompose(r)
	var dl *outbox.DeadLetterError
	if !errors.As(err, &dl) {
		t.Fatalf("err = %v, want a *outbox.DeadLetterError", err)
	}
	if dl.Reason != "client has no email on file" {
		t.Fatalf("Reason = %q, want the no-email explanation", dl.Reason)
	}
}

// A Client who claimed the invite through some other path before the
// worker got to this row already has access -- nothing to deliver.
func TestCompose_AlreadyAcceptedSkipsSend(t *testing.T) {
	r := pendingRow{
		identityUID: sql.NullString{String: "already-claimed", Valid: true},
		email:       sql.NullString{String: testInvitedEmail, Valid: true},
	}
	_, _, _, err := runCompose(r)
	if !errors.Is(err, outbox.ErrAlreadyDone) {
		t.Fatalf("err = %v, want outbox.ErrAlreadyDone", err)
	}
}
