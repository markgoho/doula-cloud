package staffinvitation_test

import (
	"os"
	"testing"
	"time"

	"doula-cloud/api/internal/activity"
	"doula-cloud/api/internal/staffinvitation"
	"doula-cloud/api/internal/testdb"
)

// TestMain terminates the shared Postgres container testdb.New starts for
// this test process -- see testdb.Main's doc comment.
func TestMain(m *testing.M) {
	os.Exit(testdb.Main(m))
}

// TestRecord_WritesTheMoveAgainstThePracticeAtTheGivenInstant proves an
// Invitation move's row answers "who, when, which Invitation": the
// Practice's own subject, the move's action, the Staff actor, the instant
// the move wrote, and a diff naming the Invitation and what it was sent
// with.
func TestRecord_WritesTheMoveAgainstThePracticeAtTheGivenInstant(t *testing.T) {
	db := testdb.New(t)
	practiceID, ownerID := testdb.SeedStaffAtNewPractice(t, db, "record-owner", []string{"owner"}, "employee")
	at := time.Date(2027, 1, 4, 9, 30, 0, 0, time.UTC)

	tx := db.BeginAs(t, testdb.Practice(practiceID))
	if err := staffinvitation.Record(t.Context(), tx, staffinvitation.TransitionInvite, practiceID, ownerID, at, staffinvitation.Diff{
		InvitationID:   "11111111-1111-1111-1111-111111111111",
		Roles:          []string{"doula"},
		EmploymentType: "contractor",
		ExpiresAt:      at.Add(7 * 24 * time.Hour),
	}); err != nil {
		t.Fatalf("Record: %v", err)
	}

	var subjectKind, subjectID, action, actor, diff string
	var createdAt time.Time
	if err := tx.QueryRowContext(t.Context(),
		`SELECT subject_kind, subject_id, action, actor_staff_id, diff::text, created_at FROM activity WHERE practice_id = $1`, practiceID,
	).Scan(&subjectKind, &subjectID, &action, &actor, &diff, &createdAt); err != nil {
		t.Fatalf("read activity row: %v", err)
	}
	if subjectKind != activity.SubjectPractice || subjectID != practiceID {
		t.Errorf("subject = %s/%s, want %s/%s", subjectKind, subjectID, activity.SubjectPractice, practiceID)
	}
	if action != string(activity.ActionStaffInvited) || actor != ownerID {
		t.Errorf("action/actor = %q/%q, want %q/%q", action, actor, activity.ActionStaffInvited, ownerID)
	}
	if !createdAt.Equal(at) {
		t.Errorf("created_at = %v, want the move's own instant %v", createdAt, at)
	}
	const wantDiff = `{"roles": ["doula"], "expiresAt": "2027-01-11T09:30:00Z", "invitationId": "11111111-1111-1111-1111-111111111111", "employmentType": "contractor"}`
	if diff != wantDiff {
		t.Errorf("diff = %s, want %s", diff, wantDiff)
	}
}

// TestSchema_RefusesAStatusWithoutItsWhoAndWhen proves 00170's
// practice_invitations_lifecycle CHECK: each stored status carries exactly
// its own actor and instant, 'expired' is never stored, and the
// accept-after-revoke shape (accepted, still carrying revoked_by) is
// refused by the table itself.
func TestSchema_RefusesAStatusWithoutItsWhoAndWhen(t *testing.T) {
	db := testdb.New(t)
	practiceID, ownerID := testdb.SeedStaffAtNewPractice(t, db, "schema-owner", []string{"owner"}, "employee")
	invitationID := testdb.Scalar[string](t, db.Admin,
		`INSERT INTO practice_invitations (practice_id, address, roles, employment_type, token_digest, invited_by, expires_at)
		 VALUES ($1, 'schema@example.com', '{doula}', 'employee', 'schema-digest', $2, now() + interval '1 day') RETURNING id`,
		practiceID, ownerID)

	refused := map[string]string{
		"expired stored":            `UPDATE practice_invitations SET status = 'expired' WHERE id = $1`,
		"revoked with no actor":     `UPDATE practice_invitations SET status = 'revoked' WHERE id = $1`,
		"accepted with no accepter": `UPDATE practice_invitations SET status = 'accepted', accepted_at = now() WHERE id = $1`,
		"pending carrying a revoke": `UPDATE practice_invitations SET revoked_by = $2, revoked_at = now() WHERE id = $1`,
		"accepted after a revoke": `UPDATE practice_invitations SET status = 'accepted', accepted_staff_id = $2, accepted_at = now(),
		                                                           revoked_by = $2, revoked_at = now() WHERE id = $1`,
	}
	for name, statement := range refused {
		t.Run(name, func(t *testing.T) {
			tx := db.BeginAdmin(t)
			if _, err := tx.ExecContext(t.Context(), statement, invitationID, ownerID); err == nil {
				t.Fatalf("%s was stored, want practice_invitations_lifecycle to refuse it", name)
			}
		})
	}

	tx := db.BeginAdmin(t)
	if _, err := tx.ExecContext(t.Context(),
		`UPDATE practice_invitations SET status = 'revoked', revoked_by = $2, revoked_at = now() WHERE id = $1`, invitationID, ownerID,
	); err != nil {
		t.Fatalf("a revoke with its actor and instant was refused: %v", err)
	}
}

// TestRecord_RefusesAMoveWithNoActionOfItsOwn proves accepting, which the
// Membership's 'joined' row records, cannot write a second, blank row.
func TestRecord_RefusesAMoveWithNoActionOfItsOwn(t *testing.T) {
	if err := staffinvitation.Record(t.Context(), nil, staffinvitation.TransitionAccept, "", "", time.Time{}, staffinvitation.Diff{}); err == nil {
		t.Fatal("Record(TransitionAccept) = nil, want a refusal")
	}
}
