package visit_test

import (
	"slices"
	"testing"
	"time"

	"doula-cloud/api/internal/fixture"
	"doula-cloud/api/internal/testdb"
	"doula-cloud/api/internal/visit"
)

// attachedDoula is a Doula already on the Engagement.
func (e *env) attachedDoula() fixture.StaffRecord {
	e.T.Helper()
	doula := e.Colleague()
	testdb.SeedGrantedAttachment(e.T, e.DB, e.Engagement.ID, doula.ID)
	return doula
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
			e := newEnv(t, signedInAs(fixture.Admin()))
			doula := e.attachedDoula()

			out := e.create(visit.CreateRequest{StaffID: &doula.ID, ScheduledAt: scheduledAt})

			if want := []string{doula.ID}; !slices.Equal(out.NotifiedStaffIDs, want) {
				t.Fatalf("notifiedStaffIds = %v, want %v", out.NotifiedStaffIDs, want)
			}
			if got, want := queuedNotices(t, e.DB), []queuedNotice{{doula.ID, kindVisitTo, e.Staff.ID}}; !slices.Equal(got, want) {
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
		e := newEnv(t, signedInAs(fixture.Owner()))
		doula := e.Colleague()

		out := e.create(visit.CreateRequest{StaffID: &doula.ID})

		if want := []string{doula.ID}; !slices.Equal(out.NotifiedStaffIDs, want) {
			t.Fatalf("notifiedStaffIds = %v, want %v", out.NotifiedStaffIDs, want)
		}
		if got, want := queuedNotices(t, e.DB), []queuedNotice{{doula.ID, kindPutOn, e.Staff.ID}}; !slices.Equal(got, want) {
			t.Fatalf("queued = %+v, want %+v", got, want)
		}
	})
	t.Run("reassign", func(t *testing.T) {
		e := newEnv(t, signedInAs(fixture.Admin()))
		from, to := e.Colleague(), e.Colleague()
		visitID := seedVisit(t, e.DB, e.Engagement.ID, from.ID)

		out := e.reassignOK(visitID, to.ID)

		if want := []string{to.ID, from.ID}; !slices.Equal(out.NotifiedStaffIDs, want) {
			t.Fatalf("notifiedStaffIds = %v, want %v", out.NotifiedStaffIDs, want)
		}
		want := []queuedNotice{
			{to.ID, kindPutOn, e.Staff.ID},
			{from.ID, "visit_reassigned_from", e.Staff.ID},
		}
		if got := queuedNotices(t, e.DB); !slices.Equal(got, want) {
			t.Fatalf("queued = %+v, want %+v", got, want)
		}
	})
}

// #2540: a Doula who creates her own Visit is never mailed about it.
func TestCreateHandler_EmailsNobodyAboutTheirOwnVisit(t *testing.T) {
	e := newEnv(t, signedInAs(fixture.Owner(), fixture.Doula()))

	out := e.create(visit.CreateRequest{})

	if len(out.NotifiedStaffIDs) != 0 {
		t.Fatalf("notifiedStaffIds = %v, want none", out.NotifiedStaffIDs)
	}
	if got := queuedNotices(t, e.DB); len(got) != 0 {
		t.Fatalf("queued = %+v, want none", got)
	}
}
