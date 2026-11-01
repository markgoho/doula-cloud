package visit_test

import (
	"slices"
	"testing"
	"time"

	"doula-cloud/api/internal/testdb"
	"doula-cloud/api/internal/visit"
)

func createFor(t *testing.T, db *testdb.DB, uid, practiceID, engagementID string, body visit.CreateRequest) visit.CreateResponse {
	t.Helper()
	srv, session := newServer(t, db, uid)
	defer srv.Close()
	resp := createVisit(t, session, visitsURL(srv.URL, practiceID, engagementID), "", body)
	defer resp.Body.Close()
	return decodeCreate(t, resp)
}

// #2540: an Admin creating a Visit that names a Doula already on the
// Engagement queues her the same "put on a visit" email a move does,
// naming the Admin, and the response says she was emailed. With no date
// set it is mailed too: the product cannot tell a meeting that happened
// from one not yet scheduled.
func TestCreateHandler_EmailsTheDoulaItNames(t *testing.T) {
	scheduled := time.Date(2027, 3, 4, 15, 0, 0, 0, time.UTC).Format(time.RFC3339)
	for name, scheduledAt := range map[string]*string{"scheduled": &scheduled, "no date": nil} {
		t.Run(name, func(t *testing.T) {
			db := testdb.New(t)
			const adminUID = "admin-creating"
			practiceID, adminID := testdb.SeedStaffAtNewPractice(t, db, adminUID, []string{adminRole}, "employee")
			doulaID := testdb.SeedStaffAtPractice(t, db, practiceID, "doula-named", []string{doulaRole}, "employee")
			_, engagementID := testdb.SeedEngagement(t, db, practiceID)
			testdb.SeedGrantedAttachment(t, db, engagementID, doulaID)

			out := createFor(t, db, adminUID, practiceID, engagementID, visit.CreateRequest{StaffID: &doulaID, ScheduledAt: scheduledAt})

			if want := []string{doulaID}; !slices.Equal(out.NotifiedStaffIDs, want) {
				t.Fatalf("notifiedStaffIds = %v, want %v", out.NotifiedStaffIDs, want)
			}
			if got, want := queuedNotices(t, db), []queuedNotice{{doulaID, kindVisitTo, adminID}}; !slices.Equal(got, want) {
				t.Fatalf("queued = %+v, want %+v", got, want)
			}
		})
	}
}

// #2540: naming a Doula who was not on the Engagement puts her on it, and
// she gets one email for the act, "put on a client", not that and "put
// on a visit" as well. Create and reassign decide it the same way.
func TestVisitWrites_ADoulaPutOnTheClientGetsOneEmail(t *testing.T) {
	t.Run("create", func(t *testing.T) {
		db := testdb.New(t)
		const ownerUID = "owner-creating"
		practiceID, ownerID := testdb.SeedStaffAtNewPractice(t, db, ownerUID, []string{ownerRole}, "employee")
		doulaID := testdb.SeedStaffAtPractice(t, db, practiceID, "doula-new", []string{doulaRole}, "employee")
		_, engagementID := testdb.SeedEngagement(t, db, practiceID)

		out := createFor(t, db, ownerUID, practiceID, engagementID, visit.CreateRequest{StaffID: &doulaID})

		if want := []string{doulaID}; !slices.Equal(out.NotifiedStaffIDs, want) {
			t.Fatalf("notifiedStaffIds = %v, want %v", out.NotifiedStaffIDs, want)
		}
		if got, want := queuedNotices(t, db), []queuedNotice{{doulaID, "put_on_engagement", ownerID}}; !slices.Equal(got, want) {
			t.Fatalf("queued = %+v, want %+v", got, want)
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

		out := reassign(t, db, adminUID, practiceID, engagementID, visitID, toID)

		if want := []string{toID, fromID}; !slices.Equal(out.NotifiedStaffIDs, want) {
			t.Fatalf("notifiedStaffIds = %v, want %v", out.NotifiedStaffIDs, want)
		}
		want := []queuedNotice{
			{toID, "put_on_engagement", adminID},
			{fromID, "visit_reassigned_from", adminID},
		}
		if got := queuedNotices(t, db); !slices.Equal(got, want) {
			t.Fatalf("queued = %+v, want %+v", got, want)
		}
	})
}

// #2540: a Doula who creates her own Visit is never mailed about it.
func TestCreateHandler_EmailsNobodyAboutTheirOwnVisit(t *testing.T) {
	db := testdb.New(t)
	const doulaUID = "doula-own"
	practiceID, _ := testdb.SeedStaffAtNewPractice(t, db, doulaUID, []string{ownerRole, doulaRole}, "employee")
	_, engagementID := testdb.SeedEngagement(t, db, practiceID)

	out := createFor(t, db, doulaUID, practiceID, engagementID, visit.CreateRequest{})

	if len(out.NotifiedStaffIDs) != 0 {
		t.Fatalf("notifiedStaffIds = %v, want none", out.NotifiedStaffIDs)
	}
	if got := queuedNotices(t, db); len(got) != 0 {
		t.Fatalf("queued = %+v, want none", got)
	}
}
