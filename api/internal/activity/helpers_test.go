package activity_test

import (
	"cmp"
	"database/sql"
	"slices"
	"testing"
	"time"

	"doula-cloud/api/internal/activity"
	"doula-cloud/api/internal/testdb"
)

// row is one activity row as a test reads it back.
type row struct {
	SubjectKind        string
	SubjectID          string
	Action             string
	ActorKind          string
	ActorStaffID       *string
	ActorClientID      *string
	ActorOperatorActID *string
	Diff               string
	CreatedAt          time.Time
}

// readRow reads the one activity row practiceID has, around Row-Level
// Security.
func readRow(t *testing.T, db *testdb.DB, practiceID string) row {
	t.Helper()
	var r row
	if err := db.Admin.QueryRowContext(t.Context(),
		`SELECT subject_kind, subject_id, action, actor_kind::text, actor_staff_id, actor_client_id, actor_operator_act_id, diff::text, created_at
		 FROM activity WHERE practice_id = $1`, practiceID,
	).Scan(&r.SubjectKind, &r.SubjectID, &r.Action, &r.ActorKind, &r.ActorStaffID, &r.ActorClientID, &r.ActorOperatorActID, &r.Diff, &r.CreatedAt); err != nil {
		t.Fatalf("read activity row: %v", err)
	}
	return r
}

// inTx runs write in a transaction on conn and commits it, or rolls it
// back and returns the error write gave.
func inTx(t *testing.T, conn *sql.DB, write func(*sql.Tx) error) error {
	t.Helper()
	tx, err := conn.BeginTx(t.Context(), nil)
	if err != nil {
		t.Fatalf("begin: %v", err)
	}
	defer func() { _ = tx.Rollback() }()
	if err := write(tx); err != nil {
		return err
	}
	if err := tx.Commit(); err != nil {
		t.Fatalf("commit: %v", err)
	}
	return nil
}

// record writes e through conn and returns the error Record gave.
func record(t *testing.T, conn *sql.DB, e activity.Entry, opts ...activity.RecordOption) error {
	t.Helper()
	return inTx(t, conn, func(tx *sql.Tx) error {
		return activity.Record(t.Context(), tx, e, opts...)
	})
}

// mustRecord is record for a write that has to succeed.
func mustRecord(t *testing.T, conn *sql.DB, e activity.Entry, opts ...activity.RecordOption) {
	t.Helper()
	if err := record(t, conn, e, opts...); err != nil {
		t.Fatalf("record: %v", err)
	}
}

// requireExactly fails unless got holds every action of want and no other.
func requireExactly(t *testing.T, name string, got, want []activity.EngagementAction) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("%s = %v, want exactly %v", name, got, want)
	}
	for _, a := range want {
		if !slices.Contains(got, a) {
			t.Errorf("%s missing %q", name, a)
		}
	}
}

// requireStrictlySorted fails unless got is in order with no repeats, the
// deterministic order a caller building a SQL exclusion clause relies on.
func requireStrictlySorted[T cmp.Ordered](t *testing.T, name string, got []T) {
	t.Helper()
	for i := 1; i < len(got); i++ {
		if got[i-1] >= got[i] {
			t.Fatalf("%s not sorted at index %d: %v", name, i, got)
		}
	}
}
