package activity_test

import (
	"encoding/json"
	"testing"

	"doula-cloud/api/internal/activity"
	"doula-cloud/api/internal/testdb"
)

func seedPractice(t *testing.T, db *testdb.DB) (practiceID string) {
	t.Helper()
	if err := db.Admin.QueryRowContext(t.Context(),
		`INSERT INTO practices (name) VALUES ('Test Practice') RETURNING id`,
	).Scan(&practiceID); err != nil {
		t.Fatalf("seed practice: %v", err)
	}
	return practiceID
}

func seedStaff(t *testing.T, db *testdb.DB, identityUID string) (staffID string) {
	t.Helper()
	if err := db.Admin.QueryRowContext(t.Context(),
		`INSERT INTO staff (identity_uid, name, email, work_state) VALUES ($1, 'Test Staff', $1 || '@example.com', 'NY') RETURNING id`,
		identityUID,
	).Scan(&staffID); err != nil {
		t.Fatalf("seed staff: %v", err)
	}
	return staffID
}

func seedClient(t *testing.T, db *testdb.DB, practiceID string) (clientID string) {
	t.Helper()
	if err := db.Admin.QueryRowContext(t.Context(),
		`INSERT INTO clients (practice_id, given_name) VALUES ($1, 'Test Client') RETURNING id`,
		practiceID,
	).Scan(&clientID); err != nil {
		t.Fatalf("seed client: %v", err)
	}
	return clientID
}

type row struct {
	SubjectKind   string
	SubjectID     string
	Action        string
	ActorKind     string
	ActorStaffID  *string
	ActorClientID *string
}

func readRow(t *testing.T, db *testdb.DB, practiceID string) row {
	t.Helper()
	var r row
	if err := db.Admin.QueryRowContext(t.Context(),
		`SELECT subject_kind, subject_id, action, actor_kind::text, actor_staff_id, actor_client_id
		 FROM activity WHERE practice_id = $1`, practiceID,
	).Scan(&r.SubjectKind, &r.SubjectID, &r.Action, &r.ActorKind, &r.ActorStaffID, &r.ActorClientID); err != nil {
		t.Fatalf("read activity row: %v", err)
	}
	return r
}

func TestRecord_StaffActor(t *testing.T) {
	db := testdb.New(t)
	practiceID := seedPractice(t, db)
	staffID := seedStaff(t, db, "record-staff-actor")

	tx, err := db.Admin.BeginTx(t.Context(), nil)
	if err != nil {
		t.Fatalf("begin: %v", err)
	}
	defer func() { _ = tx.Rollback() }()

	diff, _ := json.Marshal(map[string]string{"note": "hello"})
	if err := activity.Record(t.Context(), tx, activity.Entry{
		PracticeID:  practiceID,
		SubjectKind: "membership",
		SubjectID:   staffID,
		Action:      "joined",
		Diff:        diff,
		Actor:       activity.StaffActor(staffID),
	}); err != nil {
		t.Fatalf("record: %v", err)
	}
	if err := tx.Commit(); err != nil {
		t.Fatalf("commit: %v", err)
	}

	r := readRow(t, db, practiceID)
	if r.ActorKind != "staff" || r.ActorStaffID == nil || *r.ActorStaffID != staffID || r.ActorClientID != nil {
		t.Fatalf("row = %+v, want a staff actor naming %q", r, staffID)
	}
	if r.SubjectKind != "membership" || r.SubjectID != staffID || r.Action != "joined" {
		t.Fatalf("row = %+v, want subject membership/%q joined", r, staffID)
	}
}

func TestRecord_ClientActor(t *testing.T) {
	db := testdb.New(t)
	practiceID := seedPractice(t, db)
	clientID := seedClient(t, db, practiceID)

	tx, err := db.Admin.BeginTx(t.Context(), nil)
	if err != nil {
		t.Fatalf("begin: %v", err)
	}
	defer func() { _ = tx.Rollback() }()

	if err := activity.Record(t.Context(), tx, activity.Entry{
		PracticeID:  practiceID,
		SubjectKind: "contract",
		SubjectID:   clientID,
		Action:      "signed",
		Diff:        json.RawMessage(`{}`),
		Actor:       activity.ClientActor(clientID),
	}); err != nil {
		t.Fatalf("record: %v", err)
	}
	if err := tx.Commit(); err != nil {
		t.Fatalf("commit: %v", err)
	}

	r := readRow(t, db, practiceID)
	if r.ActorKind != "client" || r.ActorClientID == nil || *r.ActorClientID != clientID || r.ActorStaffID != nil {
		t.Fatalf("row = %+v, want a client actor naming %q", r, clientID)
	}
}

func TestRecord_SystemActor(t *testing.T) {
	db := testdb.New(t)
	practiceID := seedPractice(t, db)

	tx, err := db.Admin.BeginTx(t.Context(), nil)
	if err != nil {
		t.Fatalf("begin: %v", err)
	}
	defer func() { _ = tx.Rollback() }()

	staffID := seedStaff(t, db, "record-system-actor")
	if err := activity.Record(t.Context(), tx, activity.Entry{
		PracticeID:  practiceID,
		SubjectKind: "offer",
		SubjectID:   staffID,
		Action:      "superseded",
		Diff:        nil,
		Actor:       activity.SystemActor(),
	}); err != nil {
		t.Fatalf("record: %v", err)
	}
	if err := tx.Commit(); err != nil {
		t.Fatalf("commit: %v", err)
	}

	r := readRow(t, db, practiceID)
	if r.ActorKind != "system" || r.ActorStaffID != nil || r.ActorClientID != nil {
		t.Fatalf("row = %+v, want a system actor naming nobody", r)
	}

	var diffJSON string
	if err := db.Admin.QueryRowContext(t.Context(), `SELECT diff::text FROM activity WHERE practice_id = $1`, practiceID).Scan(&diffJSON); err != nil {
		t.Fatalf("read diff: %v", err)
	}
	if diffJSON != "{}" {
		t.Fatalf("diff = %q, want a nil Diff to fall back to an empty object", diffJSON)
	}
}
