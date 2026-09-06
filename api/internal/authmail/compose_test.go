package authmail

import (
	"database/sql"
	"errors"
	"strings"
	"testing"
	"time"

	"doula-cloud/api/internal/authntest"
	"doula-cloud/api/internal/outbox"
)

const (
	testComposeAppBaseURL = "https://app.example.test"
	testVerifyToken       = "verify-token" //nolint:gosec // test fixture token, not a credential
)

func runComposeTokenMail(accounts *authntest.FakeAccountManager, r tokenMailRow) (to, subject, text string, err error) {
	c := composeTokenMail(outbox.Mailer{AppBaseURL: testComposeAppBaseURL}, accounts)
	return c(nil, nil, r, time.Now())
}

func TestComposeTokenMail_UnverifiedSendsVerificationLink(t *testing.T) {
	accounts := authntest.NewFakeAccountManager()
	accounts.Seed("uid-1", "person@example.com", false)
	r := tokenMailRow{identityUID: "uid-1", kind: KindEmailVerification, token: sql.NullString{String: testVerifyToken, Valid: true}}

	to, subject, text, err := runComposeTokenMail(accounts, r)
	if err != nil {
		t.Fatalf("compose: %v", err)
	}
	if to != "person@example.com" {
		t.Fatalf("to = %q, want the account's current email", to)
	}
	wantLink := testComposeAppBaseURL + "/verify-email?token=verify-token"
	if !strings.Contains(text, wantLink) || subject == "" {
		t.Fatalf("subject/text = %q/%q, want the verification link %q", subject, text, wantLink)
	}
}

func TestComposeTokenMail_PasswordResetSendsResetLink(t *testing.T) {
	accounts := authntest.NewFakeAccountManager()
	accounts.Seed("uid-2", "person@example.com", true)
	r := tokenMailRow{identityUID: "uid-2", kind: KindPasswordReset, token: sql.NullString{String: "reset-token", Valid: true}}

	_, _, text, err := runComposeTokenMail(accounts, r)
	if err != nil {
		t.Fatalf("compose: %v", err)
	}
	wantLink := testComposeAppBaseURL + "/reset-password?token=reset-token"
	if !strings.Contains(text, wantLink) {
		t.Fatalf("text %q does not contain link %q", text, wantLink)
	}
}

// Verified through some other path -- a fresher re-request, or a
// provider that reports addresses pre-verified -- before this row got
// sent: nothing to deliver.
func TestComposeTokenMail_AlreadyVerifiedSkipsSend(t *testing.T) {
	accounts := authntest.NewFakeAccountManager()
	accounts.Seed("uid-3", "person@example.com", true)
	r := tokenMailRow{identityUID: "uid-3", kind: KindEmailVerification, token: sql.NullString{String: testVerifyToken, Valid: true}}

	_, _, _, err := runComposeTokenMail(accounts, r)
	if !errors.Is(err, outbox.ErrAlreadyDone) {
		t.Fatalf("err = %v, want outbox.ErrAlreadyDone", err)
	}
}

func TestComposeTokenMail_UnknownAccountDeadLetters(t *testing.T) {
	accounts := authntest.NewFakeAccountManager() // no account seeded
	r := tokenMailRow{identityUID: "uid-ghost", kind: KindEmailVerification, token: sql.NullString{String: testVerifyToken, Valid: true}}

	_, _, _, err := runComposeTokenMail(accounts, r)
	var dl *outbox.DeadLetterError
	if !errors.As(err, &dl) {
		t.Fatalf("err = %v, want a *outbox.DeadLetterError", err)
	}
}

func TestComposeTokenMail_AccountManagerErrorRetries(t *testing.T) {
	accounts := authntest.NewFakeAccountManager()
	accounts.Seed("uid-4", "person@example.com", false)
	accounts.Err = errors.New("admin sdk unreachable")
	r := tokenMailRow{identityUID: "uid-4", kind: KindEmailVerification, token: sql.NullString{String: testVerifyToken, Valid: true}}

	_, _, _, err := runComposeTokenMail(accounts, r)
	var dl *outbox.DeadLetterError
	if errors.As(err, &dl) || err == nil {
		t.Fatalf("err = %v, want a plain retryable error, not a DeadLetterError or nil", err)
	}
}
