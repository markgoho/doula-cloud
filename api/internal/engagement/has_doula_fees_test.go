package engagement_test

import (
	"testing"

	"doula-cloud/api/internal/engagement"
	"doula-cloud/api/internal/testdb"
)

// #2492: the Engagement read says whether a Doula on it has a fee, so the
// hub stands its Doula fees section from its first frame. Only an Owner
// or Admin, the readers of those fees (#2271), is told.
func TestDetailHandler_HasDoulaFeesForTheFeesReadersOnly(t *testing.T) {
	db := testdb.New(t)
	const ownerUID = "has-fees-owner"
	const adminUID = "has-fees-admin"
	const employeeUID = "has-fees-employee"
	const lenaUID = "has-fees-lena"
	practiceID := testdb.SeedPractice(t, db, "Has Fees Practice")
	testdb.SeedNamedStaffAtPractice(t, db, practiceID, ownerUID, "Renata Owner", []string{ownerRole}, employeeType)
	testdb.SeedNamedStaffAtPractice(t, db, practiceID, adminUID, "Dee Admin", []string{adminRole}, employeeType)
	employeeID := testdb.SeedNamedStaffAtPractice(t, db, practiceID, employeeUID, "Priya Employee", []string{doulaRole}, employeeType)
	lenaID := testdb.SeedNamedStaffAtPractice(t, db, practiceID, lenaUID, "Lena Vasquez", []string{doulaRole}, contractorType)
	_, engagementID := testdb.SeedEngagementInStatus(t, db, practiceID, "Amara", "has-fees-client@example.com", engagement.StatusActive)
	seedFeeAttachment(t, db, engagementID, lenaID, lenaID, 45000, "2026-07-01T12:00:00Z")
	if _, err := db.Admin.ExecContext(t.Context(),
		`INSERT INTO engagement_attachments (engagement_id, staff_id, origin, attached_by) VALUES ($1, $2, 'granted', $2)`,
		engagementID, employeeID); err != nil {
		t.Fatalf("seed employee attachment: %v", err)
	}

	srv, _ := newServer(t, db, ownerUID)
	defer srv.Close()

	for _, tc := range []struct {
		uid  string
		want bool
	}{
		{ownerUID, true},
		{adminUID, true},
		{employeeUID, false},
		{lenaUID, false},
	} {
		if got := readDetail(t, db, srv, tc.uid, practiceID, engagementID).HasDoulaFees; got != tc.want {
			t.Errorf("hasDoulaFees for %s = %v, want %v", tc.uid, got, tc.want)
		}
	}
}

// An ended Attachment's fee still counts: the Practice usually pays after
// the birth, and feepayment.listFees lists it.
func TestDetailHandler_HasDoulaFeesAfterTheAttachmentEnded(t *testing.T) {
	db := testdb.New(t)
	const ownerUID = "has-fees-ended-owner"
	practiceID := testdb.SeedPractice(t, db, "Has Fees Ended Practice")
	testdb.SeedNamedStaffAtPractice(t, db, practiceID, ownerUID, "Renata Owner", []string{ownerRole}, employeeType)
	lenaID := testdb.SeedNamedStaffAtPractice(t, db, practiceID, "has-fees-ended-lena", "Lena Vasquez", []string{doulaRole}, contractorType)
	_, engagementID := testdb.SeedEngagementInStatus(t, db, practiceID, "Amara", "has-fees-ended@example.com", engagement.StatusActive)
	seedFeeAttachment(t, db, engagementID, lenaID, lenaID, 45000, "2026-07-01T12:00:00Z")
	if _, err := db.Admin.ExecContext(t.Context(),
		`UPDATE engagement_attachments SET ended_at = now(), ended_by = staff_id WHERE engagement_id = $1`, engagementID); err != nil {
		t.Fatalf("end attachment: %v", err)
	}

	srv, _ := newServer(t, db, ownerUID)
	defer srv.Close()

	if !readDetail(t, db, srv, ownerUID, practiceID, engagementID).HasDoulaFees {
		t.Error("hasDoulaFees = false, want true for a fee on an ended Attachment")
	}
}

// Without a fee on the Engagement, the Owner reads false.
func TestDetailHandler_HasNoDoulaFeesWithoutAFee(t *testing.T) {
	db := testdb.New(t)
	const ownerUID = "has-no-fees-owner"
	practiceID := testdb.SeedPractice(t, db, "Has No Fees Practice")
	testdb.SeedNamedStaffAtPractice(t, db, practiceID, ownerUID, "Maya Owner", []string{ownerRole, doulaRole}, employeeType)
	_, engagementID := testdb.SeedEngagementInStatus(t, db, practiceID, "Amara", "has-no-fees@example.com", engagement.StatusActive)

	srv, _ := newServer(t, db, ownerUID)
	defer srv.Close()

	if readDetail(t, db, srv, ownerUID, practiceID, engagementID).HasDoulaFees {
		t.Error("hasDoulaFees = true, want false with no fee on the Engagement")
	}
}
