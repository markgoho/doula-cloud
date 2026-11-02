package visit_test

import (
	"slices"
	"testing"

	"doula-cloud/api/internal/testdb"
	"doula-cloud/api/internal/visit"
)

type attachedEntry struct{ actorStaffID, attachedStaffID string }

func doulaAttachedEntries(t *testing.T, db *testdb.DB, engagementID string) []attachedEntry {
	t.Helper()
	rows, err := db.Admin.QueryContext(t.Context(),
		`SELECT actor_staff_id, diff->>'attachedStaffId' FROM activity
		  WHERE subject_id = $1 AND action = 'doula_attached' ORDER BY created_at`, engagementID)
	if err != nil {
		t.Fatalf("read activity: %v", err)
	}
	defer rows.Close()
	var out []attachedEntry
	for rows.Next() {
		var e attachedEntry
		if err := rows.Scan(&e.actorStaffID, &e.attachedStaffID); err != nil {
			t.Fatalf("scan: %v", err)
		}
		out = append(out, e)
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("read rows: %v", err)
	}
	return out
}

// #2567: a Visit that puts a Doula on the Engagement records "Put on this
// Engagement as the Doula" by the person who saved it, the same entry the
// Doula row's Save records, and a Visit for a Doula already on it records
// none.
func TestVisitWrites_RecordThatADoulaWasPutOnTheEngagement(t *testing.T) {
	t.Run("create", func(t *testing.T) {
		db := testdb.New(t)
		const ownerUID = "owner-creating"
		practiceID, ownerID := testdb.SeedStaffAtNewPractice(t, db, ownerUID, []string{ownerRole}, "employee")
		doulaID := testdb.SeedStaffAtPractice(t, db, practiceID, "doula-new", []string{doulaRole}, "employee")
		_, engagementID := testdb.SeedEngagement(t, db, practiceID)

		createFor(t, db, ownerUID, practiceID, engagementID, visit.CreateRequest{StaffID: &doulaID})

		if got, want := doulaAttachedEntries(t, db, engagementID), []attachedEntry{{ownerID, doulaID}}; !slices.Equal(got, want) {
			t.Fatalf("doula_attached = %+v, want %+v", got, want)
		}
	})
	t.Run("reassign", func(t *testing.T) {
		db := testdb.New(t)
		const adminUID = "admin-moving"
		practiceID, adminID := testdb.SeedStaffAtNewPractice(t, db, adminUID, []string{adminRole}, "employee")
		fromID := testdb.SeedStaffAtPractice(t, db, practiceID, "doula-from", []string{doulaRole}, "employee")
		toID := testdb.SeedStaffAtPractice(t, db, practiceID, "doula-new", []string{doulaRole}, "employee")
		_, engagementID := testdb.SeedEngagement(t, db, practiceID)
		visitID := seedVisit(t, db, engagementID, fromID)

		reassign(t, db, adminUID, practiceID, engagementID, visitID, toID)

		if got, want := doulaAttachedEntries(t, db, engagementID), []attachedEntry{{adminID, toID}}; !slices.Equal(got, want) {
			t.Fatalf("doula_attached = %+v, want %+v", got, want)
		}
	})
	t.Run("already on the Engagement", func(t *testing.T) {
		db := testdb.New(t)
		const adminUID = "admin-creating"
		practiceID, _ := testdb.SeedStaffAtNewPractice(t, db, adminUID, []string{adminRole}, "employee")
		doulaID := testdb.SeedStaffAtPractice(t, db, practiceID, "doula-on", []string{doulaRole}, "employee")
		_, engagementID := testdb.SeedEngagement(t, db, practiceID)
		testdb.SeedGrantedAttachment(t, db, engagementID, doulaID)

		createFor(t, db, adminUID, practiceID, engagementID, visit.CreateRequest{StaffID: &doulaID})
		createFor(t, db, adminUID, practiceID, engagementID, visit.CreateRequest{StaffID: &doulaID})

		if got := doulaAttachedEntries(t, db, engagementID); len(got) != 0 {
			t.Fatalf("doula_attached = %+v, want none", got)
		}
	})
	t.Run("a second Visit for the same new Doula", func(t *testing.T) {
		db := testdb.New(t)
		const adminUID = "admin-twice"
		practiceID, adminID := testdb.SeedStaffAtNewPractice(t, db, adminUID, []string{adminRole}, "employee")
		doulaID := testdb.SeedStaffAtPractice(t, db, practiceID, "doula-new", []string{doulaRole}, "employee")
		_, engagementID := testdb.SeedEngagement(t, db, practiceID)

		createFor(t, db, adminUID, practiceID, engagementID, visit.CreateRequest{StaffID: &doulaID})
		createFor(t, db, adminUID, practiceID, engagementID, visit.CreateRequest{StaffID: &doulaID})

		if got, want := doulaAttachedEntries(t, db, engagementID), []attachedEntry{{adminID, doulaID}}; !slices.Equal(got, want) {
			t.Fatalf("doula_attached = %+v, want %+v", got, want)
		}
		want := []queuedNotice{{doulaID, kindPutOn, adminID}, {doulaID, kindVisitTo, adminID}}
		if got := queuedNotices(t, db); !slices.Equal(got, want) {
			t.Fatalf("queued = %+v, want %+v", got, want)
		}
	})
	t.Run("a Visit handed to the Doula who holds it, with only accrued work on the Engagement", func(t *testing.T) {
		db := testdb.New(t)
		const adminUID = "admin-confirming"
		practiceID, adminID := testdb.SeedStaffAtNewPractice(t, db, adminUID, []string{adminRole}, "employee")
		doulaID := testdb.SeedStaffAtPractice(t, db, practiceID, "doula-accrued", []string{doulaRole}, "employee")
		_, engagementID := testdb.SeedEngagement(t, db, practiceID)
		testdb.SeedAttachment(t, db, engagementID, doulaID, "accrued", false)
		visitID := seedVisit(t, db, engagementID, doulaID)

		out := reassign(t, db, adminUID, practiceID, engagementID, visitID, doulaID)

		if want := []string{doulaID}; !slices.Equal(out.NotifiedStaffIDs, want) {
			t.Fatalf("notifiedStaffIds = %v, want %v", out.NotifiedStaffIDs, want)
		}
		if got, want := doulaAttachedEntries(t, db, engagementID), []attachedEntry{{adminID, doulaID}}; !slices.Equal(got, want) {
			t.Fatalf("doula_attached = %+v, want %+v", got, want)
		}
	})
	t.Run("self", func(t *testing.T) {
		db := testdb.New(t)
		const doulaUID = "doula-own"
		practiceID, doulaID := testdb.SeedStaffAtNewPractice(t, db, doulaUID, []string{ownerRole, doulaRole}, "employee")
		_, engagementID := testdb.SeedEngagement(t, db, practiceID)

		createFor(t, db, doulaUID, practiceID, engagementID, visit.CreateRequest{})

		if got, want := doulaAttachedEntries(t, db, engagementID), []attachedEntry{{doulaID, doulaID}}; !slices.Equal(got, want) {
			t.Fatalf("doula_attached = %+v, want %+v", got, want)
		}
		if got := queuedNotices(t, db); len(got) != 0 {
			t.Fatalf("queued = %+v, want none", got)
		}
	})
}
