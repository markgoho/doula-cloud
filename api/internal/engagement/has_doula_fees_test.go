package engagement_test

import (
	"testing"

	"doula-cloud/api/internal/fixture"
	"doula-cloud/api/internal/testdb"
)

// #2492: the Engagement read says whether a Doula on it has a fee, so the
// hub stands its Doula fees section from its first frame. Only an Owner
// or Admin, the readers of those fees (#2271), is told.
func TestDetailHandler_HasDoulaFeesForTheFeesReadersOnly(t *testing.T) {
	e := newEnv(t, withEngagement(fixture.Status(fixture.StatusActive)))
	owner := e.staff(fixture.Owner())
	admin := e.staff(fixture.Admin())
	employee := e.staff()
	lena := e.staff(fixture.Contractor())
	seedFeeAttachment(t, e.db, e.Engagement.ID, lena.ID, lena.ID, 45000, "2026-07-01T12:00:00Z")
	testdb.SeedAttachment(t, e.db, e.Engagement.ID, employee.ID, "granted", false)

	for _, tc := range []struct {
		name   string
		reader fixture.StaffRecord
		want   bool
	}{
		{"an Owner is told", owner, true},
		{"an Admin is told", admin, true},
		{"an employee Doula on it is not", employee, false},
		{"the contractor who has the fee is not", lena, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := e.detailAs(tc.reader).HasDoulaFees; got != tc.want {
				t.Errorf("hasDoulaFees = %v, want %v", got, tc.want)
			}
		})
	}
}

// An ended Attachment's fee still counts: the Practice usually pays after
// the birth, and feepayment.listFees lists it.
func TestDetailHandler_HasDoulaFeesAfterTheAttachmentEnded(t *testing.T) {
	e := newEnv(t, signedInAs(fixture.Owner()), withEngagement(fixture.Status(fixture.StatusActive)))
	lena := e.staff(fixture.Contractor())
	seedFeeAttachment(t, e.db, e.Engagement.ID, lena.ID, lena.ID, 45000, "2026-07-01T12:00:00Z")
	e.exec(`UPDATE engagement_attachments SET ended_at = now(), ended_by = staff_id WHERE engagement_id = $1`, e.Engagement.ID)

	if !e.detail().HasDoulaFees {
		t.Error("hasDoulaFees = false, want true for a fee on an ended Attachment")
	}
}

// Without a fee on the Engagement, the Owner reads false.
func TestDetailHandler_HasNoDoulaFeesWithoutAFee(t *testing.T) {
	e := newEnv(t, signedInAs(fixture.Owner()), withEngagement(fixture.Status(fixture.StatusActive)))

	if e.detail().HasDoulaFees {
		t.Error("hasDoulaFees = true, want false with no fee on the Engagement")
	}
}
