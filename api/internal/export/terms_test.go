package export_test

import (
	"testing"
)

// TestHandler_CarriesThePracticesAgreement holds #1547's export criterion:
// the terms_accepted row signup writes (staffauth.ActionTermsAccepted) is
// in the Practice's archive with its versions and address as written, so
// a Practice that leaves takes the record of what it agreed to with it.
func TestHandler_CarriesThePracticesAgreement(t *testing.T) {
	e := newEnv(t)
	const diff = `{"ipAddress": "203.0.113.7", "termsVersion": "2026-10-03", "privacyVersion": "2026-10-22"}`
	e.Exec(`INSERT INTO activity (practice_id, subject_kind, subject_id, action, diff, actor_kind, actor_staff_id)
		 VALUES ($1, 'practice', $1, 'terms_accepted', $2, 'staff', $3)`,
		e.Practice.ID, diff, e.Staff.ID)

	rows := e.csvFile("activity.csv")
	actionCol := indexOf(t, rows[0], "action")
	diffCol := indexOf(t, rows[0], "diff")
	actorCol := indexOf(t, rows[0], "actor_staff_id")
	for _, row := range rows[1:] {
		if row[actionCol] != "terms_accepted" {
			continue
		}
		if row[diffCol] != diff || row[actorCol] != e.Staff.ID {
			t.Fatalf("terms_accepted row = %v, want diff %s by %s", row, diff, e.Staff.ID)
		}
		return
	}
	t.Fatalf("activity.csv has no terms_accepted row: %v", rows)
}
