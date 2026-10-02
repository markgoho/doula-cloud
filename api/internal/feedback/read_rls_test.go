package feedback_test

import (
	"database/sql"
	"testing"
	"time"

	"doula-cloud/api/internal/feedback"
	"doula-cloud/api/internal/testdb"
)

// readerTx opens a transaction on the low-privilege connection as
// identityUID, the way authn.Begin and staffauth's self-resolution leave
// one, and sets app.feedback_reader to reader when it is not empty.
func readerTx(t *testing.T, db *testdb.DB, identityUID, reader string) *sql.Tx {
	t.Helper()
	tx, err := db.App.BeginTx(t.Context(), nil)
	if err != nil {
		t.Fatalf("begin: %v", err)
	}
	t.Cleanup(func() { _ = tx.Rollback() })
	if _, err := tx.ExecContext(t.Context(), `SELECT set_config('app.current_identity_uid', $1, true)`, identityUID); err != nil {
		t.Fatalf("set identity: %v", err)
	}
	if reader != "" {
		if err := feedback.OpenReader(t.Context(), tx, reader); err != nil {
			t.Fatalf("open reader: %v", err)
		}
	}
	return tx
}

func visiblePieces(t *testing.T, tx *sql.Tx) int {
	t.Helper()
	var n int
	if err := tx.QueryRowContext(t.Context(), `SELECT count(*) FROM feedback`).Scan(&n); err != nil {
		t.Fatalf("count feedback: %v", err)
	}
	return n
}

func senderRows(t *testing.T, tx *sql.Tx, feedbackID string) int {
	t.Helper()
	var n int
	if err := tx.QueryRowContext(t.Context(), `SELECT count(*) FROM feedback_sender($1)`, feedbackID).Scan(&n); err != nil {
		t.Fatalf("call feedback_sender: %v", err)
	}
	return n
}

// TestRLS_FeedbackIsReadOnlyThroughTheReaderDoor is 00122's door, from
// Postgres' side: a Staff transaction reads no piece and no sender until
// app.feedback_reader names the Staff member that transaction resolved.
// Naming somebody else opens nothing -- a transaction cannot open the
// door in another person's name.
func TestRLS_FeedbackIsReadOnlyThroughTheReaderDoor(t *testing.T) {
	db := testdb.New(t)
	const readerUID, otherUID = "feedback-reader", "feedback-not-reader"
	readerID := testdb.SeedStaff(t, db, readerUID)
	otherID := testdb.SeedStaff(t, db, otherUID)
	feedbackID := seedStaffFeedbackRow(t, db, otherID, feedback.KindNotWorking, "/account", "free text")

	for name, tc := range map[string]struct {
		identityUID, reader string
		want                int
	}{
		"no door":                           {identityUID: readerUID, reader: "", want: 0},
		"the door, in her own name":         {identityUID: readerUID, reader: readerID, want: 1},
		"the door, in somebody else's name": {identityUID: otherUID, reader: readerID, want: 0},
	} {
		t.Run(name, func(t *testing.T) {
			tx := readerTx(t, db, tc.identityUID, tc.reader)
			if got := visiblePieces(t, tx); got != tc.want {
				t.Errorf("visible pieces = %d, want %d", got, tc.want)
			}
			if got := senderRows(t, tx, feedbackID); got != tc.want {
				t.Errorf("feedback_sender rows = %d, want %d", got, tc.want)
			}
		})
	}
}

// TestRLS_AReadRowNamesTheReaderHerself: feedback_reads takes a row only
// behind the door and only in the reader's own name, and app_runtime
// cannot read the table back at all.
func TestRLS_AReadRowNamesTheReaderHerself(t *testing.T) {
	db := testdb.New(t)
	const readerUID = "feedback-read-row-reader"
	readerID := testdb.SeedStaff(t, db, readerUID)
	otherID := testdb.SeedStaff(t, db, "feedback-read-row-other")
	feedbackID := seedStaffFeedbackRow(t, db, otherID, feedback.KindNotWorking, "/account", "")
	at := time.Date(2026, 11, 3, 14, 30, 0, 0, time.UTC)

	noDoor := readerTx(t, db, readerUID, "")
	if err := feedback.RecordRead(t.Context(), noDoor, feedbackID, readerID, at); err == nil {
		t.Error("a read row was written with no door open, want it refused")
	}

	inAnothersName := readerTx(t, db, readerUID, readerID)
	if err := feedback.RecordRead(t.Context(), inAnothersName, feedbackID, otherID, at); err == nil {
		t.Error("a read row naming another Staff member was written, want it refused")
	}

	open := readerTx(t, db, readerUID, readerID)
	if err := feedback.RecordRead(t.Context(), open, feedbackID, readerID, at); err != nil {
		t.Fatalf("record read behind the door: %v", err)
	}
	var n int
	if err := open.QueryRowContext(t.Context(), `SELECT count(*) FROM feedback_reads`).Scan(&n); err == nil {
		t.Errorf("app_runtime read feedback_reads (%d rows), want no SELECT grant", n)
	}
}
