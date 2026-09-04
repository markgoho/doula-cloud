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

// TestMoneyActions_ContainsExactlyTheADR0008MoneySet pins the money-tier
// action set #476's read filter builds its SQL exclusion clause from --
// a drift here silently widens or narrows what an employed Doula or a
// contractor can read on her Engagement ledger.
func TestMoneyActions_ContainsExactlyTheADR0008MoneySet(t *testing.T) {
	got := map[activity.EngagementAction]bool{}
	for _, a := range activity.MoneyActions() {
		got[a] = true
	}
	want := []activity.EngagementAction{
		activity.ActionContractCreated,
		activity.ActionContractSent,
		activity.ActionContractSigned,
		activity.ActionContractVoided,
		activity.ActionInvoiceRaised,
		activity.ActionInvoicePaid,
	}
	if len(got) != len(want) {
		t.Fatalf("MoneyActions() = %v, want exactly %v", activity.MoneyActions(), want)
	}
	for _, a := range want {
		if !got[a] {
			t.Errorf("MoneyActions() missing %q", a)
		}
	}
}

// TestMoneyActions_Sorted proves the deterministic ordering
// engagement.ListActivityHandler's SQL-literal clause relies on.
func TestMoneyActions_Sorted(t *testing.T) {
	got := activity.MoneyActions()
	for i := 1; i < len(got); i++ {
		if got[i-1] >= got[i] {
			t.Fatalf("MoneyActions() not sorted at index %d: %v", i, got)
		}
	}
}

// TestStaffingActions_ContainsExactlyTheRosterSet pins the set CONTEXT.md's
// Activity entry keeps off a Client's own portal ledger -- "never who
// inside the Practice did what": an Offer is which Doula was asked,
// accepted or bumped, and a Visit reassignment is which Doula covers it,
// both Practice-roster facts rather than facts about her. Money actions
// are deliberately absent from this set (CONTEXT.md: "her money" stays on
// her own ledger) -- a drift here would either leak roster facts to a
// Client or hide a fact she is owed.
func TestStaffingActions_ContainsExactlyTheRosterSet(t *testing.T) {
	got := map[activity.EngagementAction]bool{}
	for _, a := range activity.StaffingActions() {
		got[a] = true
	}
	want := []activity.EngagementAction{
		activity.ActionOfferSent,
		activity.ActionOfferAccepted,
		activity.ActionOfferDeclined,
		activity.ActionOfferSuperseded,
		activity.ActionOfferWithdrawn,
		activity.ActionVisitReassigned,
	}
	if len(got) != len(want) {
		t.Fatalf("StaffingActions() = %v, want exactly %v", activity.StaffingActions(), want)
	}
	for _, a := range want {
		if !got[a] {
			t.Errorf("StaffingActions() missing %q", a)
		}
	}
}

// TestStaffingActions_Sorted proves the deterministic ordering a caller
// building a SQL exclusion clause relies on, the same way MoneyActions'
// own ordering test does.
func TestStaffingActions_Sorted(t *testing.T) {
	got := activity.StaffingActions()
	for i := 1; i < len(got); i++ {
		if got[i-1] >= got[i] {
			t.Fatalf("StaffingActions() not sorted at index %d: %v", i, got)
		}
	}
}

// TestScopeToPractice_LetsAWriteOutsideStaffauthMiddlewarePassRLS proves
// the landmine ADR-0022 names for a write site with no per-request
// app.current_practice_id (a Client-portal or webhook path): without
// ScopeToPractice, activity's own RLS policy refuses the INSERT.
func TestScopeToPractice_LetsAWriteOutsideStaffauthMiddlewarePassRLS(t *testing.T) {
	db := testdb.New(t)
	practiceID := seedPractice(t, db)
	clientID := seedClient(t, db, practiceID)

	// Proved in its own transaction: a failed statement aborts the rest
	// of a Postgres transaction (SQLSTATE 25P02), so the RLS failure and
	// the ScopeToPractice success below cannot share one tx.
	func() {
		tx, err := db.App.BeginTx(t.Context(), nil)
		if err != nil {
			t.Fatalf("begin: %v", err)
		}
		defer func() { _ = tx.Rollback() }()

		if err := activity.Record(t.Context(), tx, activity.Entry{
			PracticeID:  practiceID,
			SubjectKind: activity.SubjectEngagement,
			SubjectID:   clientID,
			Action:      "contract_signed",
			Actor:       activity.ClientActor(clientID),
		}); err == nil {
			t.Fatal("expected Record to fail RLS with no app.current_practice_id set, got no error")
		}
	}()

	tx, err := db.App.BeginTx(t.Context(), nil)
	if err != nil {
		t.Fatalf("begin: %v", err)
	}
	defer func() { _ = tx.Rollback() }()

	if err := activity.ScopeToPractice(t.Context(), tx, practiceID); err != nil {
		t.Fatalf("ScopeToPractice: %v", err)
	}
	if err := activity.Record(t.Context(), tx, activity.Entry{
		PracticeID:  practiceID,
		SubjectKind: activity.SubjectEngagement,
		SubjectID:   clientID,
		Action:      "contract_signed",
		Actor:       activity.ClientActor(clientID),
	}); err != nil {
		t.Fatalf("Record after ScopeToPractice: %v", err)
	}
}
