package mfarecoverymail

import (
	"context"
	"database/sql"
	"errors"
	"testing"
	"time"

	"doula-cloud/api/internal/authntest"
	"doula-cloud/api/internal/outbox"
	"doula-cloud/api/internal/testdb"
)

// composeTx opens the transaction compose reads `staff` in, with the
// same app.notification_worker_trusted flag outbox.ProcessHandler sets
// for the real worker -- staff_notification_worker (00033) is what lets
// these plain `staff` SELECTs past RLS at all. compose is no longer a
// pure function of its row: #892's recheck reads the recipient's and the
// subject's live rows before anything else happens, so every case below
// needs a real database rather than a nil tx.
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

// seedLiveStaff inserts one live staff row and returns its id.
func seedLiveStaff(t *testing.T, db *testdb.DB, identityUID, name string) string {
	t.Helper()
	var id string
	if err := db.Admin.QueryRowContext(t.Context(),
		`INSERT INTO staff (identity_uid, name, email, work_state) VALUES ($1, $2, $1 || '@example.com', 'NY') RETURNING id`,
		identityUID, name,
	).Scan(&id); err != nil {
		t.Fatalf("seed staff: %v", err)
	}
	return id
}

func runCompose(ctx context.Context, tx *sql.Tx, accounts *authntest.FakeAccountManager, r pendingRow) (to, subject, text string, err error) {
	return compose(accounts)(ctx, tx, r, time.Now())
}

func TestComposeCode_UnknownRecipientDeadLetters(t *testing.T) {
	db := testdb.New(t)
	subjectID := seedLiveStaff(t, db, "subject-unknown-recipient", "Priya Raman")
	// A live staff row whose Identity Platform account is missing -- the
	// recheck passes and the Admin SDK is what refuses.
	seedLiveStaff(t, db, "owner-uid-gone", "Renata Alves")
	accounts := authntest.NewFakeAccountManager() // no account seeded
	r := pendingRow{recipientIdentityUID: "owner-uid-gone", subjectStaffID: subjectID, token: sql.NullString{String: "55556666", Valid: true}}

	_, _, _, err := runCompose(t.Context(), composeTx(t, db), accounts, r)
	var dl *outbox.DeadLetterError
	if !errors.As(err, &dl) {
		t.Fatalf("err = %v, want a *outbox.DeadLetterError", err)
	}
}

func TestComposeCode_AccountManagerErrorRetries(t *testing.T) {
	db := testdb.New(t)
	subjectID := seedLiveStaff(t, db, "subject-sdk-down", "Priya Raman")
	seedLiveStaff(t, db, "owner-uid-5", "Renata Alves")
	accounts := authntest.NewFakeAccountManager()
	accounts.Seed("owner-uid-5", "owner5@example.com", true)
	accounts.Err = errors.New("admin sdk unreachable")
	r := pendingRow{recipientIdentityUID: "owner-uid-5", subjectStaffID: subjectID, token: sql.NullString{String: "77778888", Valid: true}}

	_, _, _, err := runCompose(t.Context(), composeTx(t, db), accounts, r)
	var dl *outbox.DeadLetterError
	if errors.As(err, &dl) || err == nil {
		t.Fatalf("err = %v, want a plain retryable error, not a DeadLetterError or nil", err)
	}
}

// TestComposeCode_DeletedRecipientSkipsSend is #892's recheck on the
// vouching Owner: she approved a colleague's recovery request and then
// deleted her own login before the code went out. Her Identity Platform
// account is gone with it, so without the recheck this row would
// dead-letter on ErrAccountNotFound; with it, the row is simply done.
func TestComposeCode_DeletedRecipientSkipsSend(t *testing.T) {
	db := testdb.New(t)
	subjectID := seedLiveStaff(t, db, "subject-live", "Priya Raman")
	ownerID := seedLiveStaff(t, db, "owner-uid-deleted", "Renata Alves")
	accounts := authntest.NewFakeAccountManager()
	accounts.Seed("owner-uid-deleted", "owner@example.com", true)
	testdb.RedactDeletedLogin(t, db, ownerID)
	r := pendingRow{recipientIdentityUID: "owner-uid-deleted", subjectStaffID: subjectID, token: sql.NullString{String: "11112222", Valid: true}}

	_, _, _, err := runCompose(t.Context(), composeTx(t, db), accounts, r)
	if !errors.Is(err, outbox.ErrAlreadyDone) {
		t.Fatalf("err = %v, want outbox.ErrAlreadyDone", err)
	}
}

// TestComposeCode_DeletedSubjectSkipsSend is the same recheck on the
// other person the row names: the locked-out colleague deleted her login
// rather than waiting for the code. Mailing the Owner a code for her
// would name a person who cannot use it.
func TestComposeCode_DeletedSubjectSkipsSend(t *testing.T) {
	db := testdb.New(t)
	subjectID := seedLiveStaff(t, db, "subject-deleted", "Priya Raman")
	seedLiveStaff(t, db, "owner-uid-live", "Renata Alves")
	accounts := authntest.NewFakeAccountManager()
	accounts.Seed("owner-uid-live", "owner@example.com", true)
	testdb.RedactDeletedLogin(t, db, subjectID)
	r := pendingRow{recipientIdentityUID: "owner-uid-live", subjectStaffID: subjectID, token: sql.NullString{String: "33334444", Valid: true}}

	_, _, _, err := runCompose(t.Context(), composeTx(t, db), accounts, r)
	if !errors.Is(err, outbox.ErrAlreadyDone) {
		t.Fatalf("err = %v, want outbox.ErrAlreadyDone", err)
	}
}
