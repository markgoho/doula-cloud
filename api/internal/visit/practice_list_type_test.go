package visit_test

import (
	"net/http"
	"net/url"
	"testing"
	"time"

	"doula-cloud/api/internal/fixture"
	"doula-cloud/api/internal/reqtest"
	"doula-cloud/api/internal/testdb"
	"doula-cloud/api/internal/visit"
)

// typeWindow holds every instant the type tests seed, whatever today is.
func typeWindow() url.Values {
	return window(time.Date(2026, 2, 1, 0, 0, 0, 0, time.UTC), time.Date(2026, 4, 1, 0, 0, 0, 0, time.UTC))
}

// seedTypedVisits gives engagementA a pregnancy that ended on 2026-03-15 and
// one Visit on each side of it, and one late on the evening of the 15th in
// the Practice's zone, which is already the 16th in UTC.
func seedTypedVisits(t *testing.T, f *scheduleEnv) (prenatal, birth, postpartum string) {
	t.Helper()
	setPregnancyEnded(t, f.DB, f.engagementA, "live_birth", "2026-03-15")
	prenatal = seedScheduledVisit(t, f.DB, f.engagementA, f.employeeDoula.ID, time.Date(2026, 3, 1, 14, 0, 0, 0, time.UTC))
	birth = seedScheduledVisit(t, f.DB, f.engagementA, f.employeeDoula.ID, time.Date(2026, 3, 16, 2, 0, 0, 0, time.UTC))
	postpartum = seedScheduledVisit(t, f.DB, f.engagementA, f.employeeDoula.ID, time.Date(2026, 3, 20, 14, 0, 0, 0, time.UTC))
	return prenatal, birth, postpartum
}

func typesByID(page visit.PracticeScheduleResponse) map[string]string {
	types := map[string]string{}
	for _, item := range page.Items {
		types[item.VisitID] = item.Type
	}
	return types
}

// TestPracticeSchedule_RowsCarryTheVisitType is #2870: the Practice-wide row
// says what kind of Visit it is, from the one derivation the Engagement's
// list calls, in the Practice's zone (ADR-0036).
func TestPracticeSchedule_RowsCarryTheVisitType(t *testing.T) {
	f := newScheduleEnv(t, fixture.Owner())
	prenatal, birth, postpartum := seedTypedVisits(t, f)
	elsewhere := seedScheduledVisit(t, f.DB, f.engagementB, f.otherDoula.ID, time.Date(2026, 3, 20, 14, 0, 0, 0, time.UTC))

	got := typesByID(f.readSchedule(typeWindow()))
	want := map[string]string{
		prenatal:   visit.TypePrenatal,
		birth:      visit.TypeBirth,
		postpartum: visit.TypePostpartum,
		elsewhere:  visit.TypePrenatal,
	}
	for id, wantType := range want {
		if got[id] != wantType {
			t.Errorf("visit %s: type = %q, want %q", id, got[id], wantType)
		}
	}
}

// TestPracticeSchedule_AContractorsRowsCarryTheVisitType: the narrowing is a
// row filter, and the rows she keeps are typed like anyone's.
func TestPracticeSchedule_AContractorsRowsCarryTheVisitType(t *testing.T) {
	f := newScheduleEnv(t, fixture.Doula(), fixture.Contractor())
	testdb.SeedGrantedAttachment(t, f.DB, f.engagementA, f.Staff.ID)
	prenatal, birth, postpartum := seedTypedVisits(t, f)

	got := typesByID(f.readSchedule(typeWindow()))
	want := map[string]string{
		prenatal:   visit.TypePrenatal,
		birth:      visit.TypeBirth,
		postpartum: visit.TypePostpartum,
	}
	for id, wantType := range want {
		if got[id] != wantType {
			t.Errorf("visit %s: type = %q, want %q", id, got[id], wantType)
		}
	}
}

// TestPracticeSchedule_AZoneThatWillNotLoadIsA500 is ADR-0036's rule: never
// UTC in its place.
func TestPracticeSchedule_AZoneThatWillNotLoadIsA500(t *testing.T) {
	f := newScheduleEnv(t, fixture.Owner())
	if _, err := f.DB.Admin.ExecContext(t.Context(), `UPDATE practices SET timezone = 'Not/AZone' WHERE id = $1`, f.Practice.ID); err != nil {
		t.Fatalf("break the zone: %v", err)
	}

	reqtest.ExpectStatus(t, http.MethodGet, f.scheduleURL(nil), http.StatusInternalServerError, f.SignedIn)
}
