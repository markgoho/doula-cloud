package visit_test

import (
	"slices"
	"testing"

	"doula-cloud/api/internal/fixture"
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

// requireAttachedEntries fails unless the Engagement's doula_attached
// trail is exactly want.
func (e *env) requireAttachedEntries(want ...attachedEntry) {
	e.T.Helper()
	if got := doulaAttachedEntries(e.T, e.DB, e.Engagement.ID); !slices.Equal(got, want) {
		e.T.Fatalf("doula_attached = %+v, want %+v", got, want)
	}
}

// #2567: a Visit that puts a Doula on the Engagement records "Put on this
// Engagement as the Doula" by the person who saved it, the same entry the
// Doula row's Save records, and a Visit for a Doula already on it records
// none.
func TestVisitWrites_RecordThatADoulaWasPutOnTheEngagement(t *testing.T) {
	t.Run("create", func(t *testing.T) {
		e := newEnv(t, signedInAs(fixture.Owner()))
		doula := e.Colleague()

		e.create(visit.CreateRequest{StaffID: &doula.ID})

		e.requireAttachedEntries(attachedEntry{e.Staff.ID, doula.ID})
	})
	t.Run("reassign", func(t *testing.T) {
		e := newEnv(t, signedInAs(fixture.Admin()))
		from, to := e.Colleague(), e.Colleague()
		visitID := seedVisit(t, e.DB, e.Engagement.ID, from.ID)

		e.reassignOK(visitID, to.ID)

		e.requireAttachedEntries(attachedEntry{e.Staff.ID, to.ID})
	})
	t.Run("already on the Engagement", func(t *testing.T) {
		e := newEnv(t, signedInAs(fixture.Admin()))
		doula := e.attachedDoula()

		e.create(visit.CreateRequest{StaffID: &doula.ID})
		e.create(visit.CreateRequest{StaffID: &doula.ID})

		e.requireAttachedEntries()
	})
	t.Run("a second Visit for the same new Doula", func(t *testing.T) {
		e := newEnv(t, signedInAs(fixture.Admin()))
		doula := e.Colleague()

		e.create(visit.CreateRequest{StaffID: &doula.ID})
		e.create(visit.CreateRequest{StaffID: &doula.ID})

		e.requireAttachedEntries(attachedEntry{e.Staff.ID, doula.ID})
		want := []queuedNotice{{doula.ID, kindPutOn, e.Staff.ID}, {doula.ID, kindVisitTo, e.Staff.ID}}
		if got := queuedNotices(t, e.DB); !slices.Equal(got, want) {
			t.Fatalf("queued = %+v, want %+v", got, want)
		}
	})
	t.Run("a Visit handed to the Doula who holds it, with only accrued work on the Engagement", func(t *testing.T) {
		e := newEnv(t, signedInAs(fixture.Admin()))
		doula := e.Colleague()
		testdb.SeedAttachment(t, e.DB, e.Engagement.ID, doula.ID, "accrued", false)
		visitID := seedVisit(t, e.DB, e.Engagement.ID, doula.ID)

		out := e.reassignOK(visitID, doula.ID)

		if want := []string{doula.ID}; !slices.Equal(out.NotifiedStaffIDs, want) {
			t.Fatalf("notifiedStaffIds = %v, want %v", out.NotifiedStaffIDs, want)
		}
		e.requireAttachedEntries(attachedEntry{e.Staff.ID, doula.ID})
	})
	t.Run("self", func(t *testing.T) {
		e := newEnv(t, signedInAs(fixture.Owner(), fixture.Doula()))

		e.create(visit.CreateRequest{})

		e.requireAttachedEntries(attachedEntry{e.Staff.ID, e.Staff.ID})
		if got := queuedNotices(t, e.DB); len(got) != 0 {
			t.Fatalf("queued = %+v, want none", got)
		}
	})
}
