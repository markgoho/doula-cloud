package mfarecoverymail

import (
	"database/sql"
	"errors"
	"testing"
	"time"

	"doula-cloud/api/internal/authntest"
	"doula-cloud/api/internal/outbox"
	"doula-cloud/api/internal/testdb"
)

// TestComposeCode_Refusals runs compose against a vouching Owner and the
// locked-out colleague a code is for. compose is not a pure function of its
// row: #892's recheck reads the recipient's and the subject's live rows
// before anything else happens, so every case needs a real database, and
// reads them in a transaction with the same trust outbox.ProcessHandler
// gives the real worker -- staff_notification_worker (00033) is what lets
// these plain `staff` SELECTs past RLS at all.
//
// A recipient with a live staff row but no Identity Platform account
// dead-letters, because the Admin SDK is what refuses; an Admin SDK that is
// down is a plain retryable error. #892's recheck on the vouching Owner:
// she approved a colleague's request and then deleted her own login, so her
// account is gone and without the recheck the row would dead-letter; with
// it the row is simply done. The same recheck holds on the other person the
// row names: mailing the Owner a code for a colleague who deleted her login
// would name a person who cannot use it.
func TestComposeCode_Refusals(t *testing.T) {
	deadLetters := func(t *testing.T, err error) {
		t.Helper()
		if _, ok := errors.AsType[*outbox.DeadLetterError](err); !ok {
			t.Fatalf("err = %v, want a *outbox.DeadLetterError", err)
		}
	}
	alreadyDone := func(t *testing.T, err error) {
		t.Helper()
		if !errors.Is(err, outbox.ErrAlreadyDone) {
			t.Fatalf("err = %v, want outbox.ErrAlreadyDone", err)
		}
	}
	retries := func(t *testing.T, err error) {
		t.Helper()
		var dl *outbox.DeadLetterError
		if errors.As(err, &dl) || err == nil {
			t.Fatalf("err = %v, want a plain retryable error, not a DeadLetterError or nil", err)
		}
	}

	tests := []struct {
		name             string
		noAccount        bool
		accountsDown     bool
		recipientDeleted bool
		subjectDeleted   bool
		check            func(*testing.T, error)
	}{
		{name: "an unknown recipient dead-letters", noAccount: true, check: deadLetters},
		{name: "an Admin SDK error retries", accountsDown: true, check: retries},
		{name: "a deleted recipient skips the send", recipientDeleted: true, check: alreadyDone},
		{name: "a deleted subject skips the send", subjectDeleted: true, check: alreadyDone},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db := testdb.New(t)
			subjectID := testdb.SeedStaff(t, db, "subject")
			ownerID := testdb.SeedStaff(t, db, "owner-uid")
			accounts := authntest.NewFakeAccountManager()
			if !tt.noAccount {
				accounts.Seed("owner-uid", "owner@example.com", true)
			}
			if tt.accountsDown {
				accounts.Err = errors.New("admin sdk unreachable")
			}
			if tt.recipientDeleted {
				testdb.RedactDeletedLogin(t, db, ownerID)
			}
			if tt.subjectDeleted {
				testdb.RedactDeletedLogin(t, db, subjectID)
			}
			r := pendingRow{recipientIdentityUID: "owner-uid", subjectStaffID: subjectID, token: sql.NullString{String: "55556666", Valid: true}}

			tx := db.BeginAs(t, testdb.NotificationWorker())
			_, _, _, err := compose(accounts)(t.Context(), tx, r, time.Now())
			tt.check(t, err)
		})
	}
}
