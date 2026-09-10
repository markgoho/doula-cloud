package engagement_test

import (
	"testing"

	"doula-cloud/api/internal/testdb"
)

// These tests exercise the RLS policies from
// 00005_client_engagement.sql directly via db.App and set_config,
// bypassing the Go handlers -- proving the SQL policies themselves scope
// visibility correctly, not just that the handlers happen to agree with
// them.

func TestRLS_ClientsFailsClosedWithNoPracticeSet(t *testing.T) {
	db := testdb.New(t)
	practiceID, _ := testdb.SeedStaffAtNewPractice(t, db, "fail-closed-staff", []string{doulaRole}, "employee")
	testdb.SeedEngagementInStatus(t, db, practiceID, "Some Client", "some@example.com", "intake")

	var count int
	if err := db.App.QueryRowContext(t.Context(), `SELECT count(*) FROM clients`).Scan(&count); err != nil {
		t.Fatalf("query clients with no session vars set: %v", err)
	}
	if count != 0 {
		t.Fatalf("expected 0 rows with no session variables set, got %d", count)
	}
}

// TestRLS_ClientsSelectIsScopedToPracticeID proves the clients_select
// policy narrows to Clients whose own practice_id matches
// app.current_practice_id (00042's plain-column-comparison collapse of
// the old EXISTS-through-engagements shape), not to every Client row
// globally.
func TestRLS_ClientsSelectIsScopedToPracticeID(t *testing.T) {
	db := testdb.New(t)
	practiceA, _ := testdb.SeedStaffAtNewPractice(t, db, "staff-at-a", []string{doulaRole}, "employee")
	practiceB, _ := testdb.SeedStaffAtNewPractice(t, db, "staff-at-b", []string{doulaRole}, "employee")
	clientAtA, _ := testdb.SeedEngagementInStatus(t, db, practiceA, "Client A", "a@example.com", "intake")
	testdb.SeedEngagementInStatus(t, db, practiceB, "Client B", "b@example.com", "intake")

	tx, err := db.App.BeginTx(t.Context(), nil)
	if err != nil {
		t.Fatalf("begin: %v", err)
	}
	defer tx.Rollback()

	if _, err := tx.ExecContext(t.Context(), `SELECT set_config('app.current_practice_id', $1, true)`, practiceA); err != nil {
		t.Fatalf("set_config: %v", err)
	}

	var visibleIDs []string
	rows, err := tx.QueryContext(t.Context(), `SELECT id FROM clients`)
	if err != nil {
		t.Fatalf("query clients: %v", err)
	}
	defer rows.Close()
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			t.Fatalf("scan: %v", err)
		}
		visibleIDs = append(visibleIDs, id)
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("iterate rows: %v", err)
	}

	if len(visibleIDs) != 1 || visibleIDs[0] != clientAtA {
		t.Fatalf("visible clients = %v, want only %q", visibleIDs, clientAtA)
	}
}

// TestRLS_ClientsInsertRejectedWithNoPracticeSet proves clients_insert
// fails closed too: an INSERT attempted with no app.current_practice_id
// set is rejected by RLS, not silently allowed.
func TestRLS_ClientsInsertRejectedWithNoPracticeSet(t *testing.T) {
	db := testdb.New(t)
	// A real Practice to reference, so the only thing distinguishing this
	// from the allowed case below is the missing session var -- not a
	// missing practice_id or given_name NOT NULL violation instead.
	practiceID, _ := testdb.SeedStaffAtNewPractice(t, db, "fail-closed-inserting", []string{doulaRole}, "employee")

	tx, err := db.App.BeginTx(t.Context(), nil)
	if err != nil {
		t.Fatalf("begin: %v", err)
	}
	defer tx.Rollback()

	_, err = tx.ExecContext(t.Context(),
		`INSERT INTO clients (practice_id, given_name, email) VALUES ($1, 'No Context', 'no-context@example.com')`,
		practiceID,
	)
	if err == nil {
		t.Fatal("expected INSERT to be rejected by RLS with no app.current_practice_id set, got no error")
	}
}

// TestRLS_ClientsInsertAllowedWithPracticeSet proves clients_insert
// permits an INSERT once a Practice context is set and the caller's own
// Membership there is an employee's, even though no Engagement
// referencing the new Client exists yet -- the chicken-and-egg case
// CreateHandler relies on.
func TestRLS_ClientsInsertAllowedWithPracticeSet(t *testing.T) {
	db := testdb.New(t)
	practiceID, staffID := testdb.SeedStaffAtNewPractice(t, db, "staff-inserting", []string{doulaRole}, "employee")

	tx, err := db.App.BeginTx(t.Context(), nil)
	if err != nil {
		t.Fatalf("begin: %v", err)
	}
	defer tx.Rollback()

	if _, err := tx.ExecContext(t.Context(), `SELECT set_config('app.current_practice_id', $1, true)`, practiceID); err != nil {
		t.Fatalf("set_config: %v", err)
	}
	if _, err := tx.ExecContext(t.Context(), `SELECT set_config('app.current_staff_id', $1, true)`, staffID); err != nil {
		t.Fatalf("set_config: %v", err)
	}

	if _, err := tx.ExecContext(t.Context(),
		`INSERT INTO clients (practice_id, given_name, email) VALUES ($1, 'With Context', 'with-context@example.com')`,
		practiceID,
	); err != nil {
		t.Fatalf("expected INSERT to be allowed with a Practice context set, got error: %v", err)
	}
}

// TestRLS_ClientsInsertRejectedForContractorMembership proves the other
// half of clients_insert's new WITH CHECK: a contractor Doula's own
// Membership fails the same insert an employee's passes, even with both
// session vars set correctly. ADR-0017: "a contractor originates
// nothing" -- she cannot create a Client at a Practice she contracts for.
func TestRLS_ClientsInsertRejectedForContractorMembership(t *testing.T) {
	db := testdb.New(t)
	practiceID, _ := testdb.SeedStaffAtNewPractice(t, db, "staff-contractor-inserting", []string{doulaRole}, "employee")
	contractorID := testdb.SeedContractorAtPractice(t, db, practiceID, "contractor-inserting")

	tx, err := db.App.BeginTx(t.Context(), nil)
	if err != nil {
		t.Fatalf("begin: %v", err)
	}
	defer tx.Rollback()

	if _, err := tx.ExecContext(t.Context(), `SELECT set_config('app.current_practice_id', $1, true)`, practiceID); err != nil {
		t.Fatalf("set_config: %v", err)
	}
	if _, err := tx.ExecContext(t.Context(), `SELECT set_config('app.current_staff_id', $1, true)`, contractorID); err != nil {
		t.Fatalf("set_config: %v", err)
	}

	_, err = tx.ExecContext(t.Context(),
		`INSERT INTO clients (practice_id, given_name, email) VALUES ($1, 'Contractor Attempt', 'contractor-attempt@example.com')`,
		practiceID,
	)
	if err == nil {
		t.Fatal("expected INSERT to be rejected for a contractor Doula's Membership, got no error")
	}
}

// TestRLS_ClientsInsertAllowedForOwnerWithContractorEmploymentType proves
// clients_insert's refusal is role-gated, not bare employment_type: an
// Owner who also does the work under a contractor employment type
// (ADR-0017's "solo Practice") stays in the Owner column and may still
// create a Client, unlike a pure contractor Doula.
func TestRLS_ClientsInsertAllowedForOwnerWithContractorEmploymentType(t *testing.T) {
	db := testdb.New(t)
	var practiceID string
	if err := db.Admin.QueryRowContext(t.Context(),
		`INSERT INTO practices (name, timezone) VALUES ('Owner Contractor Practice', 'America/New_York') RETURNING id`,
	).Scan(&practiceID); err != nil {
		t.Fatalf("seed practice: %v", err)
	}
	ownerContractorID := testdb.SeedStaffAtPractice(t, db, practiceID, "owner-contractor-inserting", []string{ownerRole, doulaRole}, contractorType)

	tx, err := db.App.BeginTx(t.Context(), nil)
	if err != nil {
		t.Fatalf("begin: %v", err)
	}
	defer tx.Rollback()

	if _, err := tx.ExecContext(t.Context(), `SELECT set_config('app.current_practice_id', $1, true)`, practiceID); err != nil {
		t.Fatalf("set_config: %v", err)
	}
	if _, err := tx.ExecContext(t.Context(), `SELECT set_config('app.current_staff_id', $1, true)`, ownerContractorID); err != nil {
		t.Fatalf("set_config: %v", err)
	}

	if _, err := tx.ExecContext(t.Context(),
		`INSERT INTO clients (practice_id, given_name, email) VALUES ($1, 'Owner Contractor Client', 'owner-contractor-client@example.com')`,
		practiceID,
	); err != nil {
		t.Fatalf("expected INSERT to be allowed for an Owner with a contractor employment type, got error: %v", err)
	}
}

// TestRLS_EngagementsVisibilityIsScopedToCurrentPractice proves the
// engagements policy narrows to rows for app.current_practice_id, not
// every Engagement globally.
func TestRLS_EngagementsVisibilityIsScopedToCurrentPractice(t *testing.T) {
	db := testdb.New(t)
	practiceA, _ := testdb.SeedStaffAtNewPractice(t, db, "staff-eng-a", []string{doulaRole}, "employee")
	practiceB, _ := testdb.SeedStaffAtNewPractice(t, db, "staff-eng-b", []string{doulaRole}, "employee")
	testdb.SeedEngagementInStatus(t, db, practiceA, "Client A", "a@example.com", "intake")
	testdb.SeedEngagementInStatus(t, db, practiceB, "Client B", "b@example.com", "intake")

	tx, err := db.App.BeginTx(t.Context(), nil)
	if err != nil {
		t.Fatalf("begin: %v", err)
	}
	defer tx.Rollback()

	if _, err := tx.ExecContext(t.Context(), `SELECT set_config('app.current_practice_id', $1, true)`, practiceA); err != nil {
		t.Fatalf("set_config: %v", err)
	}

	var count int
	if err := tx.QueryRowContext(t.Context(), `SELECT count(*) FROM engagements`).Scan(&count); err != nil {
		t.Fatalf("query engagements: %v", err)
	}
	if count != 1 {
		t.Fatalf("expected only Practice A's engagement visible, got count = %d", count)
	}
}

// TestRLS_ClientsUpdateFollowsSelectScope proves clients_update (00042)
// follows clients_select's shape, per ADR-0017's "edit follows read": a
// Practice's own session can update its Client's row, and another
// Practice's session affects zero rows attempting the same update rather
// than erroring -- RLS filters rows, it doesn't reject the statement.
func TestRLS_ClientsUpdateFollowsSelectScope(t *testing.T) {
	db := testdb.New(t)
	practiceA, _ := testdb.SeedStaffAtNewPractice(t, db, "staff-updating-a", []string{doulaRole}, "employee")
	practiceB, _ := testdb.SeedStaffAtNewPractice(t, db, "staff-updating-b", []string{doulaRole}, "employee")
	clientID, _ := testdb.SeedEngagementInStatus(t, db, practiceA, "Original Name", "original@example.com", "intake")

	tx, err := db.App.BeginTx(t.Context(), nil)
	if err != nil {
		t.Fatalf("begin: %v", err)
	}
	defer tx.Rollback()

	if _, err := tx.ExecContext(t.Context(), `SELECT set_config('app.current_practice_id', $1, true)`, practiceB); err != nil {
		t.Fatalf("set_config: %v", err)
	}
	result, err := tx.ExecContext(t.Context(), `UPDATE clients SET given_name = 'Renamed' WHERE id = $1`, clientID)
	if err != nil {
		t.Fatalf("update from another practice's session: %v", err)
	}
	if n, _ := result.RowsAffected(); n != 0 {
		t.Fatalf("Practice B's session updated %d rows of Practice A's Client, want 0", n)
	}

	if _, err := tx.ExecContext(t.Context(), `SELECT set_config('app.current_practice_id', $1, true)`, practiceA); err != nil {
		t.Fatalf("set_config: %v", err)
	}
	result, err = tx.ExecContext(t.Context(), `UPDATE clients SET given_name = 'Renamed' WHERE id = $1`, clientID)
	if err != nil {
		t.Fatalf("update from own practice's session: %v", err)
	}
	if n, _ := result.RowsAffected(); n != 1 {
		t.Fatalf("Practice A's session updated %d rows of its own Client, want 1", n)
	}
}

// TestEngagementsCompletedIsExplained_CHECKDemandsBothFacts proves
// engagements_completed_is_explained (00094) is a real database CHECK,
// not only TransitionHandler's own guards: a direct UPDATE to
// 'completed' missing either of ADR-0015's two facts is rejected by
// Postgres itself, superuser connection and all -- a CHECK applies
// regardless of role or RLS. Both halves are asserted separately,
// because #253 landed only the ending_reason one and a CHECK that had
// silently dropped back to it would still pass a single-case test.
func TestEngagementsCompletedIsExplained_CHECKDemandsBothFacts(t *testing.T) {
	db := testdb.New(t)
	practiceID, _ := testdb.SeedStaffAtNewPractice(t, db, "check-staff", []string{doulaRole}, "employee")
	_, engagementID := testdb.SeedEngagementInStatus(t, db, practiceID, "Client", "check@example.com", "active")

	for _, missing := range []struct{ name, statement string }{
		{"neither fact",
			`UPDATE engagements SET status = 'completed' WHERE id = $1`},
		{"no ending_reason",
			`UPDATE engagements SET status = 'completed', birth_outcome = 'unknown' WHERE id = $1`},
		{"no birth_outcome",
			`UPDATE engagements SET status = 'completed', ending_reason = 'care_complete' WHERE id = $1`},
	} {
		if _, err := db.Admin.ExecContext(t.Context(), missing.statement, engagementID); err == nil {
			t.Fatalf("expected the CHECK to reject a completed row with %s, got no error", missing.name)
		}
	}

	if _, err := db.Admin.ExecContext(t.Context(),
		`UPDATE engagements SET status = 'completed', ending_reason = 'care_complete', birth_outcome = 'unknown'
		  WHERE id = $1`, engagementID,
	); err != nil {
		t.Fatalf("expected a completed row carrying both facts to be accepted, got: %v", err)
	}
}

// TestRLS_EngagementEventsScopedToPractice proves
// engagement_events_practice_visibility (00090) narrows to rows for
// app.current_practice_id, the same shape
// TestRLS_EngagementsVisibilityIsScopedToCurrentPractice proves for
// engagements itself.
func TestRLS_EngagementEventsScopedToPractice(t *testing.T) {
	db := testdb.New(t)
	practiceA, _ := testdb.SeedStaffAtNewPractice(t, db, "events-staff-a", []string{doulaRole}, "employee")
	practiceB, _ := testdb.SeedStaffAtNewPractice(t, db, "events-staff-b", []string{doulaRole}, "employee")
	_, engagementA := testdb.SeedEngagementInStatus(t, db, practiceA, "Client A", "events-a@example.com", "intake")
	_, engagementB := testdb.SeedEngagementInStatus(t, db, practiceB, "Client B", "events-b@example.com", "intake")

	for _, row := range []struct{ practiceID, engagementID string }{
		{practiceA, engagementA}, {practiceB, engagementB},
	} {
		if _, err := db.Admin.ExecContext(t.Context(),
			`INSERT INTO engagement_events (practice_id, engagement_id, event_type, previous_status, status)
			 VALUES ($1, $2, 'status_changed', 'intake', 'active')`,
			row.practiceID, row.engagementID,
		); err != nil {
			t.Fatalf("seed engagement_events: %v", err)
		}
	}

	tx, err := db.App.BeginTx(t.Context(), nil)
	if err != nil {
		t.Fatalf("begin: %v", err)
	}
	defer tx.Rollback()
	if _, err := tx.ExecContext(t.Context(), `SELECT set_config('app.current_practice_id', $1, true)`, practiceA); err != nil {
		t.Fatalf("set_config: %v", err)
	}

	var count int
	if err := tx.QueryRowContext(t.Context(), `SELECT count(*) FROM engagement_events`).Scan(&count); err != nil {
		t.Fatalf("query engagement_events: %v", err)
	}
	if count != 1 {
		t.Fatalf("expected only Practice A's engagement_events row visible, got count = %d", count)
	}
}

// TestRLS_EngagementEventsInvisibleToPortalSession proves engagement_events
// carries no client-tier read policy at all -- ADR-0015's "readable by
// staff at the Practice and never by a portal session" -- by setting
// app.current_client_id the way a Client-portal request does (00006) and
// proving RLS still narrows to zero rows even though a real row exists.
func TestRLS_EngagementEventsInvisibleToPortalSession(t *testing.T) {
	db := testdb.New(t)
	practiceID, _ := testdb.SeedStaffAtNewPractice(t, db, "events-portal-staff", []string{doulaRole}, "employee")
	clientID, engagementID := testdb.SeedEngagementInStatus(t, db, practiceID, "Client", "events-portal@example.com", "intake")
	if _, err := db.Admin.ExecContext(t.Context(),
		`INSERT INTO engagement_events (practice_id, engagement_id, event_type, previous_status, status)
		 VALUES ($1, $2, 'status_changed', 'intake', 'active')`,
		practiceID, engagementID,
	); err != nil {
		t.Fatalf("seed engagement_events: %v", err)
	}

	tx, err := db.App.BeginTx(t.Context(), nil)
	if err != nil {
		t.Fatalf("begin: %v", err)
	}
	defer tx.Rollback()
	if _, err := tx.ExecContext(t.Context(), `SELECT set_config('app.current_client_id', $1, true)`, clientID); err != nil {
		t.Fatalf("set_config: %v", err)
	}

	var count int
	if err := tx.QueryRowContext(t.Context(), `SELECT count(*) FROM engagement_events`).Scan(&count); err != nil {
		t.Fatalf("query engagement_events: %v", err)
	}
	if count != 0 {
		t.Fatalf("expected zero engagement_events rows visible to a Client-portal session, got %d", count)
	}
}
