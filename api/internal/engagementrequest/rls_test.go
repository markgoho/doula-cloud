package engagementrequest_test

import (
	"testing"

	"doula-cloud/api/internal/fixture"
	"doula-cloud/api/internal/testdb"
)

// TestRLS_InsertPolicyGatesOnRole proves engagement_requests_insert (00047)
// refuses a contractor Doula's INSERT independent of RequestHandler's own
// Go-level check, and admits an employee Doula, so the refusal is the
// policy's role gate working and not a blanket refusal. A contractor
// employment type does not refuse an Owner or Admin membership: ADR-0017's
// solo-Practice carve-out.
func TestRLS_InsertPolicyGatesOnRole(t *testing.T) {
	for _, tc := range []struct {
		name    string
		staff   []fixture.StaffOption
		refused bool
	}{
		{"a contractor Doula is refused", []fixture.StaffOption{fixture.Contractor()}, true},
		{"an employee Doula is admitted", nil, false},
		{"a contractor Owner is admitted", []fixture.StaffOption{fixture.Owner(), fixture.Contractor()}, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			e := newEnv(t, signedInAs(tc.staff...))
			tx := e.DB.BeginAs(t, testdb.Practice(e.Practice.ID), testdb.Staff(e.Staff.ID))

			_, err := tx.ExecContext(t.Context(),
				`INSERT INTO engagement_requests (practice_id, client_id, kind, due_date, requested_by)
				 VALUES ($1, $2, 'birth', $3, $4)`,
				e.Practice.ID, e.Client.ID, testDueDate, e.Staff.ID,
			)
			if tc.refused && err == nil {
				t.Fatal("expected engagement_requests_insert to refuse the INSERT, got no error")
			}
			if !tc.refused && err != nil {
				t.Fatalf("expected the INSERT to be admitted, got: %v", err)
			}
		})
	}
}
