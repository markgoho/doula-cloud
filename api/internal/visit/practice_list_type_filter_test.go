package visit_test

import (
	"net/http"
	"net/url"
	"slices"
	"strings"
	"testing"
	"time"

	"doula-cloud/api/internal/fixture"
	"doula-cloud/api/internal/reqtest"
	"doula-cloud/api/internal/testdb"
	"doula-cloud/api/internal/visit"
)

const typeParameter = "type"

type typeCase struct {
	name   string
	at     time.Time
	endsOn string
	want   string
}

// typeCases is the one table the Go rule and the SQL are both held to. The
// Practice's zone is behind UTC, so the instants sit either side of its
// midnight, not UTC's.
func typeCases() []typeCase {
	const ended = "2026-03-15"
	return []typeCase{
		{"no end date", time.Date(2026, 3, 20, 14, 0, 0, 0, time.UTC), "", visit.TypePrenatal},
		{"a day before", time.Date(2026, 3, 14, 14, 0, 0, 0, time.UTC), ended, visit.TypePrenatal},
		{"last minute of the day before", time.Date(2026, 3, 15, 3, 59, 0, 0, time.UTC), ended, visit.TypePrenatal},
		{"first minute of the birth day", time.Date(2026, 3, 15, 4, 0, 0, 0, time.UTC), ended, visit.TypeBirth},
		{"evening of the birth day, already the next day in UTC", time.Date(2026, 3, 16, 2, 0, 0, 0, time.UTC), ended, visit.TypeBirth},
		{"first minute of the next day", time.Date(2026, 3, 16, 4, 0, 0, 0, time.UTC), ended, visit.TypePostpartum},
		{"a week after", time.Date(2026, 3, 22, 14, 0, 0, 0, time.UTC), ended, visit.TypePostpartum},
	}
}

func visitTypes() []string {
	return []string{visit.TypePrenatal, visit.TypeBirth, visit.TypePostpartum}
}

func (f *scheduleEnv) readType(visitType string) visit.PracticeScheduleResponse {
	f.T.Helper()
	params := typeWindow()
	params.Set(typeParameter, visitType)
	return f.readSchedule(params)
}

// TestPracticeSchedule_TypeSelectsWhatDeriveTypeSays holds the SQL CASE in
// scheduleFromWhere and DeriveType to the same cases, in the Practice's zone.
func TestPracticeSchedule_TypeSelectsWhatDeriveTypeSays(t *testing.T) {
	f := newScheduleEnv(t, fixture.Owner())
	zone := seededPracticeZone(t)
	if _, offset := time.Date(2026, 3, 15, 12, 0, 0, 0, zone).Zone(); offset >= 0 {
		t.Fatalf("the seeded zone is not behind UTC (offset %d)", offset)
	}
	setPregnancyEnded(t, f.DB, f.engagementA, "live_birth", "2026-03-15")

	byType := map[string][]string{}
	for _, c := range typeCases() {
		engagement := f.engagementA
		var endsOn *string
		if c.endsOn == "" {
			engagement = f.engagementB
		} else {
			endsOn = &c.endsOn
		}
		if got := visit.DeriveType(c.at, endsOn, zone); got != c.want {
			t.Fatalf("%s: DeriveType = %q, want %q", c.name, got, c.want)
		}
		id := seedScheduledVisit(t, f.DB, engagement, f.employeeDoula.ID, c.at)
		byType[c.want] = append(byType[c.want], id)
	}

	for _, visitType := range visitTypes() {
		t.Run(visitType, func(t *testing.T) {
			page := f.readType(visitType)
			got := visitIDs(page)
			want := byType[visitType]
			slices.Sort(got)
			slices.Sort(want)
			if !slices.Equal(got, want) {
				t.Fatalf("type=%s rows = %v, want %v", visitType, got, want)
			}
			wantVisitCount(t, page, len(want))
			for _, item := range page.Items {
				if item.Type != visitType {
					t.Errorf("visit %s: type = %q under type=%s", item.VisitID, item.Type, visitType)
				}
			}
		})
	}
}

func TestPracticeSchedule_TypeNarrowsTheListBody(t *testing.T) {
	f := newScheduleEnv(t, fixture.Owner())
	_, birth, _ := seedTypedVisits(t, f)
	from, to := rfc3339Window(time.Date(2026, 2, 1, 0, 0, 0, 0, time.UTC), time.Date(2026, 4, 1, 0, 0, 0, 0, time.UTC))

	page := f.searchSchedule(visit.ScheduleListRequest{From: from, To: to, Type: visit.TypeBirth})
	if got := visitIDs(page); !slices.Equal(got, []string{birth}) {
		t.Fatalf("type birth = %v, want [%s]", got, birth)
	}
	wantVisitCount(t, page, 1)
}

func TestPracticeSchedule_AContractorWithATypeStillSeesOnlyHerAttachments(t *testing.T) {
	f := newScheduleEnv(t, fixture.Doula(), fixture.Contractor())
	testdb.SeedGrantedAttachment(t, f.DB, f.engagementA, f.Staff.ID)
	_, birth, _ := seedTypedVisits(t, f)
	setPregnancyEnded(t, f.DB, f.engagementB, "live_birth", "2026-03-15")
	seedScheduledVisit(t, f.DB, f.engagementB, f.otherDoula.ID, time.Date(2026, 3, 16, 2, 0, 0, 0, time.UTC))

	page := f.readType(visit.TypeBirth)
	if got := visitIDs(page); !slices.Equal(got, []string{birth}) {
		t.Fatalf("contractor type=birth = %v, want only her own [%s]", got, birth)
	}
	wantVisitCount(t, page, 1)
}

func TestPracticeSchedule_TypeCountAgreesWithEveryRowAcrossPagesAndOrders(t *testing.T) {
	f := newScheduleEnv(t, fixture.Owner())
	setPregnancyEnded(t, f.DB, f.engagementA, "live_birth", "2020-01-01")
	f.seedOverOnePage()

	for _, order := range []string{"", latestFirst().Get(orderParameter)} {
		params := url.Values{typeParameter: {visit.TypePostpartum}, "order": {order}}
		first := f.readSchedule(params)
		if !first.HasMore {
			t.Fatalf("order %q: first page hasMore = false, want a second page", order)
		}
		wantVisitCount(t, first, 32)
		if rows := f.pagedRows(params); rows != 32 {
			t.Fatalf("order %q: rows paged = %d, want the count 32", order, rows)
		}
	}
	wantVisitCount(t, f.readSchedule(url.Values{typeParameter: {visit.TypePrenatal}}), 1)
}

func TestPracticeSchedule_AnUnknownTypeIsA400(t *testing.T) {
	f := newScheduleEnv(t, fixture.Owner())

	for _, value := range []string{"Birth", "antenatal", "any", "1"} {
		t.Run(value, func(t *testing.T) {
			body := reqtest.ExpectBody(t, http.MethodGet, f.scheduleURL(url.Values{typeParameter: {value}}), http.StatusBadRequest, f.SignedIn)
			if !strings.Contains(string(body), visit.MsgInvalidType) {
				t.Fatalf("body = %s, want %q", body, visit.MsgInvalidType)
			}
		})
	}
	if body := f.expectSearchRefused(visit.ScheduleListRequest{Type: "antenatal"}, http.StatusBadRequest); !strings.Contains(body, visit.MsgInvalidType) {
		t.Fatalf("body = %s, want %q", body, visit.MsgInvalidType)
	}
}

func TestPracticeSchedule_AnEmptyTypeIsNoNarrowing(t *testing.T) {
	f := newScheduleEnv(t, fixture.Owner())

	wantVisitCount(t, f.readSchedule(url.Values{typeParameter: {""}}), 2)
}
