package clientauth_test

import (
	"testing"

	"doula-cloud/api/internal/testdb"
)

// These tests exercise the RLS policies from
// 00006_client_portal_users.sql directly via db.App and set_config,
// bypassing the Go middleware -- proving the SQL policies themselves
// scope visibility correctly, not just that the middleware happens to
// agree with them.

// TestRLS_ClientPortalUsersFailsClosedWithNoSessionVarsSet proves
// client_portal_users denies all rows when no session variable is set,
// even though a matching row genuinely exists.
func TestRLS_ClientPortalUsersFailsClosedWithNoSessionVarsSet(t *testing.T) {
	db := testdb.New(t)
	seedClientWithEngagement(t, db, "fail-closed-portal-uid")

	var count int
	if err := db.App.QueryRowContext(t.Context(), `SELECT count(*) FROM client_portal_users`).Scan(&count); err != nil {
		t.Fatalf("query client_portal_users with no session vars set: %v", err)
	}
	if count != 0 {
		t.Fatalf("expected 0 rows with no session variables set, got %d", count)
	}
}

// TestRLS_ClientPortalUsersSelfVisibilityOnlyAppliesBeforeClientIsChosen
// proves the self-visibility policy stops applying once
// app.current_client_id has been set, mirroring
// TestRLS_StaffSelfVisibilityOnlyAppliesBeforePracticeIsChosen in
// staffauth/rls_test.go for the Client-portal population.
func TestRLS_ClientPortalUsersSelfVisibilityOnlyAppliesBeforeClientIsChosen(t *testing.T) {
	db := testdb.New(t)
	const identityUID = "self-visibility-portal-uid"
	clientID, _ := seedClientWithEngagement(t, db, identityUID)

	tx, err := db.App.BeginTx(t.Context(), nil)
	if err != nil {
		t.Fatalf("begin: %v", err)
	}
	defer tx.Rollback()

	if _, err := tx.ExecContext(t.Context(), `SELECT set_config('app.current_identity_uid', $1, true)`, identityUID); err != nil {
		t.Fatalf("set_config identity: %v", err)
	}

	var count int
	if err := tx.QueryRowContext(t.Context(), `SELECT count(*) FROM client_portal_users WHERE identity_uid = $1`, identityUID).Scan(&count); err != nil {
		t.Fatalf("query client_portal_users before client id set: %v", err)
	}
	if count != 1 {
		t.Fatalf("expected caller's own row visible before a Client is chosen, got count = %d", count)
	}

	// Once a Client context is set (to some other Client -- as would
	// happen if a bug ever queried this table mid-request), the
	// self-visibility policy must stop applying.
	if _, err := tx.ExecContext(t.Context(), `SELECT set_config('app.current_client_id', $1, true)`, clientID); err != nil {
		t.Fatalf("set_config client: %v", err)
	}
	if err := tx.QueryRowContext(t.Context(), `SELECT count(*) FROM client_portal_users WHERE identity_uid = $1`, identityUID).Scan(&count); err != nil {
		t.Fatalf("query client_portal_users after client id set: %v", err)
	}
	if count != 0 {
		t.Fatalf("expected self-visibility to stop applying once a Client is chosen, got count = %d", count)
	}
}

// TestRLS_EngagementsFailsClosedForClientTierWithNoSessionVarsSet proves
// engagements_client_visibility, like every other RLS policy in this
// schema, denies all rows with no session variable set.
func TestRLS_EngagementsFailsClosedForClientTierWithNoSessionVarsSet(t *testing.T) {
	db := testdb.New(t)
	seedClientWithEngagement(t, db, "fail-closed-engagement-uid")

	var count int
	if err := db.App.QueryRowContext(t.Context(), `SELECT count(*) FROM engagements`).Scan(&count); err != nil {
		t.Fatalf("query engagements with no session vars set: %v", err)
	}
	if count != 0 {
		t.Fatalf("expected 0 rows with no session variables set, got %d", count)
	}
}

// TestRLS_EngagementsClientVisibilityIsScopedToCurrentClient proves
// engagements_client_visibility narrows to the Engagement belonging to
// app.current_client_id, not every Engagement globally -- and not even
// every Engagement belonging to that Client's own other Engagements at a
// different Practice, when queried from the wrong Client's context.
func TestRLS_EngagementsClientVisibilityIsScopedToCurrentClient(t *testing.T) {
	db := testdb.New(t)
	clientA, engagementA := seedClientWithEngagement(t, db, "client-a-uid")
	_, engagementB := seedClientWithEngagement(t, db, "client-b-uid")

	tx, err := db.App.BeginTx(t.Context(), nil)
	if err != nil {
		t.Fatalf("begin: %v", err)
	}
	defer tx.Rollback()

	if _, err := tx.ExecContext(t.Context(), `SELECT set_config('app.current_client_id', $1, true)`, clientA); err != nil {
		t.Fatalf("set_config: %v", err)
	}

	var visibleIDs []string
	rows, err := tx.QueryContext(t.Context(), `SELECT id FROM engagements`)
	if err != nil {
		t.Fatalf("query engagements: %v", err)
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

	if len(visibleIDs) != 1 || visibleIDs[0] != engagementA {
		t.Fatalf("visible engagements = %v, want only %q", visibleIDs, engagementA)
	}
	if visibleIDs[0] == engagementB {
		t.Fatalf("Client B's engagement leaked into Client A's context")
	}
}

// TestRLS_StaffSelfVisibilityHiddenDuringClientPortalContext proves the
// 00006 widening of staff_self_visibility: a person whose identity_uid
// happens to match both a staff row and a client_portal_users row must
// not have their staff row leak into a Client-portal-scoped transaction,
// even though app.current_practice_id is (correctly) never set there.
func TestRLS_StaffSelfVisibilityHiddenDuringClientPortalContext(t *testing.T) {
	db := testdb.New(t)
	const sharedUID = "shared-identity-uid"

	clientID, _ := seedClientWithEngagement(t, db, sharedUID)
	if _, err := db.Admin.ExecContext(t.Context(),
		`INSERT INTO staff (identity_uid, name, email) VALUES ($1, 'Shared Person', 'shared@example.com')`,
		sharedUID,
	); err != nil {
		t.Fatalf("seed staff row with shared identity_uid: %v", err)
	}

	tx, err := db.App.BeginTx(t.Context(), nil)
	if err != nil {
		t.Fatalf("begin: %v", err)
	}
	defer tx.Rollback()

	if _, err := tx.ExecContext(t.Context(), `SELECT set_config('app.current_identity_uid', $1, true)`, sharedUID); err != nil {
		t.Fatalf("set_config identity: %v", err)
	}
	if _, err := tx.ExecContext(t.Context(), `SELECT set_config('app.current_client_id', $1, true)`, clientID); err != nil {
		t.Fatalf("set_config client: %v", err)
	}

	var count int
	if err := tx.QueryRowContext(t.Context(), `SELECT count(*) FROM staff WHERE identity_uid = $1`, sharedUID).Scan(&count); err != nil {
		t.Fatalf("query staff during client-portal context: %v", err)
	}
	if count != 0 {
		t.Fatalf("expected the shared identity's staff row to stay hidden during a Client-portal context, got count = %d", count)
	}
}
