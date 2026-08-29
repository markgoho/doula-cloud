// Proves two of #396's GRANT-level claims hold at the database itself,
// not just by application convention -- app_runtime's table privileges
// are checked before RLS ever runs, so these reject regardless of the
// caller's session context.
package rlsguardrail_test

import (
	"strings"
	"testing"

	"doula-cloud/api/internal/testdb"
)

// TestGrant_ActivityIsAppendOnly proves activity (00051_activity_log.sql,
// ADR-0022) holds SELECT and INSERT but no UPDATE or DELETE grant for
// app_runtime -- an audit trail that can't be rewritten or removed after
// the fact.
func TestGrant_ActivityIsAppendOnly(t *testing.T) {
	db := testdb.New(t)
	practiceID, clientID, _ := seedEngagementAt(t, db, "Grant Client Events Practice")
	staffID := seedGrantStaffAt(t, db, practiceID, "grant-client-events-staff")

	var eventID string
	if err := db.Admin.QueryRowContext(t.Context(),
		`INSERT INTO activity (practice_id, subject_kind, subject_id, action, diff, actor_kind, actor_staff_id)
		 VALUES ($1, 'client', $2, 'created', '{}'::jsonb, 'staff', $3) RETURNING id`,
		practiceID, clientID, staffID,
	).Scan(&eventID); err != nil {
		t.Fatalf("seed client event: %v", err)
	}

	if _, err := db.App.ExecContext(t.Context(),
		`UPDATE activity SET diff = '{"x":1}'::jsonb WHERE id = $1`, eventID,
	); err == nil {
		t.Fatal("expected UPDATE on activity to be rejected, got no error")
	} else if !strings.Contains(err.Error(), "permission denied") {
		t.Fatalf("expected a permission-denied error, got: %v", err)
	}

	if _, err := db.App.ExecContext(t.Context(), `DELETE FROM activity WHERE id = $1`, eventID); err == nil {
		t.Fatal("expected DELETE on activity to be rejected, got no error")
	} else if !strings.Contains(err.Error(), "permission denied") {
		t.Fatalf("expected a permission-denied error, got: %v", err)
	}
}

// TestGrant_EngagementRequestsHasNoDelete proves engagement_requests
// (00042_client_intake_schema.sql) holds SELECT, INSERT and UPDATE but no
// DELETE grant -- a Request is asked once and answered once (withdrawn,
// refused and approved are all UPDATEs to its state column), and the row
// itself is never removed.
func TestGrant_EngagementRequestsHasNoDelete(t *testing.T) {
	db := testdb.New(t)
	practiceID, clientID, _ := seedEngagementAt(t, db, "Grant Engagement Requests Practice")
	staffID := seedGrantStaffAt(t, db, practiceID, "grant-engagement-requests-staff")

	var requestID string
	if err := db.Admin.QueryRowContext(t.Context(),
		`INSERT INTO engagement_requests (practice_id, client_id, kind, requested_by)
		 VALUES ($1, $2, 'birth', $3) RETURNING id`,
		practiceID, clientID, staffID,
	).Scan(&requestID); err != nil {
		t.Fatalf("seed engagement request: %v", err)
	}

	if _, err := db.App.ExecContext(t.Context(), `DELETE FROM engagement_requests WHERE id = $1`, requestID); err == nil {
		t.Fatal("expected DELETE on engagement_requests to be rejected, got no error")
	} else if !strings.Contains(err.Error(), "permission denied") {
		t.Fatalf("expected a permission-denied error, got: %v", err)
	}
}

// seedGrantStaffAt inserts a Staff row and an owner practice_memberships
// row at practiceID -- the actor these tests' rows are attributed to.
func seedGrantStaffAt(t *testing.T, db *testdb.DB, practiceID, identityUID string) (staffID string) {
	t.Helper()

	if err := db.Admin.QueryRowContext(t.Context(),
		`INSERT INTO staff (identity_uid, name, email, work_state) VALUES ($1, 'Grant Test Staff', 'grant-test@example.com', 'NY') RETURNING id`,
		identityUID,
	).Scan(&staffID); err != nil {
		t.Fatalf("seed staff: %v", err)
	}
	if _, err := db.Admin.ExecContext(t.Context(),
		`INSERT INTO practice_memberships (practice_id, staff_id, roles, employment_type) VALUES ($1, $2, '{owner}', 'employee')`,
		practiceID, staffID,
	); err != nil {
		t.Fatalf("seed membership: %v", err)
	}
	return staffID
}
