package staffinvite

import (
	"database/sql"
	"errors"
	"strings"
	"testing"
	"time"

	"doula-cloud/api/internal/outbox"
)

const testAppBaseURL = "https://app.example.test"

// runCompose exercises compose's Compose closure with the fields a real
// call site provides (context and tx neither branch below ever touches).
func runCompose(r pendingRow, now time.Time) (to, subject, text string, err error) {
	c := compose(outbox.Mailer{AppBaseURL: testAppBaseURL})
	return c(nil, nil, r, now) //nolint:staticcheck // compose reads only r and now; ctx/tx are never dereferenced
}

func TestCompose_PendingUnexpiredInvitationSendsWithLink(t *testing.T) {
	now := time.Now()
	r := pendingRow{
		inviteToken: sql.NullString{String: "11111111-1111-1111-1111-111111111111", Valid: true},
		address:     "invited@example.com",
		status:      invitationStatusPending,
		expiresAt:   now.Add(time.Hour),
	}
	to, subject, text, err := runCompose(r, now)
	if err != nil {
		t.Fatalf("compose: %v", err)
	}
	if to != "invited@example.com" {
		t.Fatalf("to = %q, want the Invitation's address", to)
	}
	if subject != staffInviteSubject {
		t.Fatalf("subject = %q, want %q", subject, staffInviteSubject)
	}
	wantLink := testAppBaseURL + "/accept-invite?token=11111111-1111-1111-1111-111111111111"
	if !strings.Contains(text, wantLink) {
		t.Fatalf("text %q does not contain link %q", text, wantLink)
	}
}

func TestCompose_ExpiredInvitationSkipsSend(t *testing.T) {
	now := time.Now()
	r := pendingRow{status: invitationStatusPending, expiresAt: now.Add(-time.Minute)}
	_, _, _, err := runCompose(r, now)
	if !errors.Is(err, outbox.ErrAlreadyDone) {
		t.Fatalf("err = %v, want outbox.ErrAlreadyDone", err)
	}
}

func TestCompose_NonPendingInvitationSkipsSend(t *testing.T) {
	now := time.Now()
	r := pendingRow{status: "accepted", expiresAt: now.Add(time.Hour)}
	_, _, _, err := runCompose(r, now)
	if !errors.Is(err, outbox.ErrAlreadyDone) {
		t.Fatalf("err = %v, want outbox.ErrAlreadyDone", err)
	}
}
