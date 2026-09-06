package mfarecoverymail

import (
	"database/sql"
	"errors"
	"testing"
	"time"

	"doula-cloud/api/internal/authntest"
	"doula-cloud/api/internal/outbox"
)

func TestComposeCode_UnknownRecipientDeadLetters(t *testing.T) {
	accounts := authntest.NewFakeAccountManager() // no account seeded
	r := pendingRow{recipientIdentityUID: "owner-uid-gone", subjectStaffID: "subject-1", token: sql.NullString{String: "55556666", Valid: true}}

	_, _, _, err := compose(accounts)(nil, nil, r, time.Now())
	var dl *outbox.DeadLetterError
	if !errors.As(err, &dl) {
		t.Fatalf("err = %v, want a *outbox.DeadLetterError", err)
	}
}

func TestComposeCode_AccountManagerErrorRetries(t *testing.T) {
	accounts := authntest.NewFakeAccountManager()
	accounts.Seed("owner-uid-5", "owner5@example.com", true)
	accounts.Err = errors.New("admin sdk unreachable")
	r := pendingRow{recipientIdentityUID: "owner-uid-5", subjectStaffID: "subject-1", token: sql.NullString{String: "77778888", Valid: true}}

	_, _, _, err := compose(accounts)(nil, nil, r, time.Now())
	var dl *outbox.DeadLetterError
	if errors.As(err, &dl) || err == nil {
		t.Fatalf("err = %v, want a plain retryable error, not a DeadLetterError or nil", err)
	}
}
