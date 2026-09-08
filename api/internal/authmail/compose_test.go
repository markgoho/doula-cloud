package authmail

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"testing"
	"time"

	"doula-cloud/api/internal/authntest"
	"doula-cloud/api/internal/outbox"
	"doula-cloud/api/internal/testdb"
)

const (
	testComposeAppBaseURL = "https://app.example.test"
	testVerifyToken       = "verify-token" //nolint:gosec // test fixture token, not a credential
)

// composeTx opens the transaction composeTokenMail reads `staff` in, with
// the same app.notification_worker_trusted flag outbox.ProcessHandler
// sets for the real worker -- staff_notification_worker (00033) is what
// lets that SELECT past RLS at all. A verification row's Compose is no
// longer a pure function of its row: #892's recheck reads her live staff
// row before the Admin SDK is reached.
func composeTx(t *testing.T, db *testdb.DB) *sql.Tx {
	t.Helper()
	tx, err := db.App.BeginTx(t.Context(), nil)
	if err != nil {
		t.Fatalf("begin: %v", err)
	}
	t.Cleanup(func() { _ = tx.Rollback() })
	if _, err := tx.ExecContext(t.Context(), `SELECT set_config('app.notification_worker_trusted', 'true', true)`); err != nil {
		t.Fatalf("set trusted flag: %v", err)
	}
	return tx
}

func runComposeTokenMail(ctx context.Context, tx *sql.Tx, accounts *authntest.FakeAccountManager, r tokenMailRow) (to, subject, text string, err error) {
	c := composeTokenMail(outbox.Mailer{AppBaseURL: testComposeAppBaseURL}, accounts)
	return c(ctx, tx, r, time.Now())
}

func TestComposeTokenMail_UnverifiedSendsVerificationLink(t *testing.T) {
	db := testdb.New(t)
	testdb.SeedStaff(t, db, "uid-1")
	accounts := authntest.NewFakeAccountManager()
	accounts.Seed("uid-1", "person@example.com", false)
	r := tokenMailRow{identityUID: "uid-1", kind: KindEmailVerification, token: sql.NullString{String: testVerifyToken, Valid: true}}

	to, subject, text, err := runComposeTokenMail(t.Context(), composeTx(t, db), accounts, r)
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

// A reset row reads no `staff` row at all -- RequestResetHandler resolves
// its identity through Identity Platform, which also holds Client Portal
// accounts, so this kind cannot read absence as deletion. Hence the nil
// tx here: reaching for one would be a lie about what this path touches.
func TestComposeTokenMail_PasswordResetSendsResetLink(t *testing.T) {
	accounts := authntest.NewFakeAccountManager()
	accounts.Seed("uid-2", "person@example.com", true)
	r := tokenMailRow{identityUID: "uid-2", kind: KindPasswordReset, token: sql.NullString{String: "reset-token", Valid: true}}

	_, _, text, err := runComposeTokenMail(t.Context(), nil, accounts, r)
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
	db := testdb.New(t)
	testdb.SeedStaff(t, db, "uid-3")
	accounts := authntest.NewFakeAccountManager()
	accounts.Seed("uid-3", "person@example.com", true)
	r := tokenMailRow{identityUID: "uid-3", kind: KindEmailVerification, token: sql.NullString{String: testVerifyToken, Valid: true}}

	_, _, _, err := runComposeTokenMail(t.Context(), composeTx(t, db), accounts, r)
	if !errors.Is(err, outbox.ErrAlreadyDone) {
		t.Fatalf("err = %v, want outbox.ErrAlreadyDone", err)
	}
}

// A reset row for an identity Identity Platform no longer knows: this is
// the dead-letter this file's whole asymmetry preserves. A verification
// row in the same position is skipped instead (see
// TestComposeTokenMail_DeletedLoginSkipsVerification), because that kind
// can tell the two cases apart and this one cannot.
func TestComposeTokenMail_UnknownAccountDeadLetters(t *testing.T) {
	accounts := authntest.NewFakeAccountManager() // no account seeded
	r := tokenMailRow{identityUID: "uid-ghost", kind: KindPasswordReset, token: sql.NullString{String: "reset-token", Valid: true}}

	_, _, _, err := runComposeTokenMail(t.Context(), nil, accounts, r)
	var dl *outbox.DeadLetterError
	if !errors.As(err, &dl) {
		t.Fatalf("err = %v, want a *outbox.DeadLetterError", err)
	}
}

func TestComposeTokenMail_AccountManagerErrorRetries(t *testing.T) {
	db := testdb.New(t)
	testdb.SeedStaff(t, db, "uid-4")
	accounts := authntest.NewFakeAccountManager()
	accounts.Seed("uid-4", "person@example.com", false)
	accounts.Err = errors.New("admin sdk unreachable")
	r := tokenMailRow{identityUID: "uid-4", kind: KindEmailVerification, token: sql.NullString{String: testVerifyToken, Valid: true}}

	_, _, _, err := runComposeTokenMail(t.Context(), composeTx(t, db), accounts, r)
	var dl *outbox.DeadLetterError
	if errors.As(err, &dl) || err == nil {
		t.Fatalf("err = %v, want a plain retryable error, not a DeadLetterError or nil", err)
	}
}

// TestComposeTokenMail_DeletedLoginSkipsVerification is #892's
// skip-at-send recheck: she asked for a fresh verification link and
// deleted her login before the row was claimed. Her Identity Platform
// account went with it, so without the recheck this row would
// dead-letter on ErrAccountNotFound; with it, there is simply nothing
// left to verify.
func TestComposeTokenMail_DeletedLoginSkipsVerification(t *testing.T) {
	db := testdb.New(t)
	staffID := testdb.SeedStaff(t, db, "uid-deleted-login")
	accounts := authntest.NewFakeAccountManager()
	accounts.Seed("uid-deleted-login", "person@example.com", false)
	testdb.RedactDeletedLogin(t, db, staffID)
	r := tokenMailRow{identityUID: "uid-deleted-login", kind: KindEmailVerification, token: sql.NullString{String: testVerifyToken, Valid: true}}

	_, _, _, err := runComposeTokenMail(t.Context(), composeTx(t, db), accounts, r)
	if !errors.Is(err, outbox.ErrAlreadyDone) {
		t.Fatalf("err = %v, want outbox.ErrAlreadyDone", err)
	}
}
