package practicetimezone_test

import (
	"encoding/json/v2"
	"net/http"
	"testing"
	"time"

	"doula-cloud/api/internal/fixture"
	"doula-cloud/api/internal/practicetimezone"
	"doula-cloud/api/internal/reqtest"
	"doula-cloud/api/internal/testdb"
)

// seededZone is what testdb.SeedPractice states for a fixture Practice,
// so it is what every one of these tests reads before it writes.
const seededZone = "America/New_York"

// denverZone is the zone these tests move a Practice to: a real IANA name,
// two hours off the seeded one, so a wrong day is a visible day rather
// than an hour's difference.
const denverZone = "America/Denver"

// env is what a test in this package starts from: a Practice with the
// timezone routes mounted and one Staff member at the screen, an Owner
// unless a test says otherwise.
type env struct {
	*fixture.Scene
}

func newEnv(t *testing.T, staff ...fixture.StaffOption) *env {
	t.Helper()
	if len(staff) == 0 {
		staff = []fixture.StaffOption{fixture.Owner()}
	}
	return &env{fixture.NewScene(t, fixture.Routes(practicetimezone.Mount), fixture.SignedInAs(staff...))}
}

func (e *env) url() string { return e.PracticeURL("/timezone") }

func put(zone string) reqtest.Option {
	return reqtest.JSON(practicetimezone.PutRequest{Timezone: zone})
}

// storedZone is the zone a Practice holds, read around the handlers.
func (e *env) storedZone(practiceID string) string {
	e.T.Helper()
	return testdb.Scalar[string](e.T, e.DB.Admin, `SELECT timezone FROM practices WHERE id = $1`, practiceID)
}

// changedRows is the Activity query for the Practice's zone changes.
const changedRows = `FROM activity WHERE practice_id = $1 AND action = 'practice_timezone_changed'`

// TestGetHandler_ReadsThePracticesOwnZone proves an Owner reads the zone
// the Practice actually holds, rather than a zone the BFF assumes.
func TestGetHandler_ReadsThePracticesOwnZone(t *testing.T) {
	e := newEnv(t)

	out := reqtest.Expect[practicetimezone.Response](t, http.MethodGet, e.url(), http.StatusOK, e.SignedIn)

	if out.Timezone != seededZone {
		t.Fatalf("timezone = %q, want %q", out.Timezone, seededZone)
	}
}

// TestPutHandler_OwnerStatesAZone is the happy path: the row moves and
// the answer names the zone that is now stored.
func TestPutHandler_OwnerStatesAZone(t *testing.T) {
	e := newEnv(t)

	out := reqtest.Expect[practicetimezone.Response](t, http.MethodPut, e.url(), http.StatusOK, e.SignedIn, put(denverZone))

	if out.Timezone != denverZone {
		t.Fatalf("timezone = %q, want %q", out.Timezone, denverZone)
	}
	if got := e.storedZone(e.Practice.ID); got != denverZone {
		t.Fatalf("stored zone = %q, want %q", got, denverZone)
	}
}

// TestPutHandler_RecordsWhoChangedItAndFromWhat is the audit trail
// CLAUDE.md asks every feature to carry: the Activity row names the
// acting Staff member and both sides of the change, so "how did this
// Practice come to be in Denver" has an answer.
func TestPutHandler_RecordsWhoChangedItAndFromWhat(t *testing.T) {
	e := newEnv(t)

	reqtest.ExpectStatus(t, http.MethodPut, e.url(), http.StatusOK, e.SignedIn, put(denverZone))

	if actor := testdb.Scalar[string](t, e.DB.Admin, `SELECT actor_staff_id `+changedRows, e.Practice.ID); actor != e.Staff.ID {
		t.Fatalf("actor = %q, want the Staff member who pressed it (%q)", actor, e.Staff.ID)
	}
	var recorded struct {
		Before string `json:"timezoneBefore"`
		After  string `json:"timezoneAfter"`
	}
	if err := json.Unmarshal(testdb.Scalar[[]byte](t, e.DB.Admin, `SELECT diff `+changedRows, e.Practice.ID), &recorded); err != nil {
		t.Fatalf("decode diff: %v", err)
	}
	if recorded.Before != seededZone || recorded.After != denverZone {
		t.Fatalf("diff = %+v, want %q -> %q", recorded, seededZone, denverZone)
	}
}

// TestPutHandler_ResendingTheSameZoneRecordsNothingNew proves the write
// is idempotent the way practicerate.PutRateHandler is: a retry with the
// same body leaves the row where it is and adds no second Activity row,
// so a double-press does not read as two changes.
func TestPutHandler_ResendingTheSameZoneRecordsNothingNew(t *testing.T) {
	e := newEnv(t)

	for range 2 {
		reqtest.ExpectStatus(t, http.MethodPut, e.url(), http.StatusOK, e.SignedIn, put(denverZone))
	}

	if rows := testdb.Scalar[int](t, e.DB.Admin, `SELECT count(*) `+changedRows, e.Practice.ID); rows != 1 {
		t.Fatalf("activity rows = %d, want 1 -- the second press changed nothing", rows)
	}
}

// TestPutHandler_RefusesAZoneTheDatabaseDoesNotName is the boundary the
// screen cannot enforce: the API, not the select, decides what a zone is.
// The three zones are the three ways a name can be wrong -- one the IANA
// database has never carried, the empty string that time.LoadLocation
// answers as UTC, and "Local", which is the server's own zone rather
// than the Practice's.
func TestPutHandler_RefusesAZoneTheDatabaseDoesNotName(t *testing.T) {
	for _, zone := range []string{"Nowhere/Atlantis", "", "Local"} {
		t.Run("zone "+zone, func(t *testing.T) {
			e := newEnv(t)

			body := reqtest.Expect[struct {
				Details map[string]string `json:"details"`
			}](t, http.MethodPut, e.url(), http.StatusBadRequest, e.SignedIn, put(zone))

			if body.Details["timezone"] == "" {
				t.Fatalf("details = %v, want an entry keyed by the DTO's own timezone field", body.Details)
			}
			if got := e.storedZone(e.Practice.ID); got != seededZone {
				t.Fatalf("stored zone = %q, want the refusal to have written nothing", got)
			}
		})
	}
}

// TestPutHandler_RefusesABodyItCannotRead covers the shape a hand-built
// caller can send that is not a zone at all: the decode refuses before
// the zone is ever looked at, so nothing is written.
func TestPutHandler_RefusesABodyItCannotRead(t *testing.T) {
	e := newEnv(t)

	reqtest.ExpectStatus(t, http.MethodPut, e.url(), http.StatusBadRequest, e.SignedIn, reqtest.RawJSON([]byte("{not json")))

	if got := e.storedZone(e.Practice.ID); got != seededZone {
		t.Fatalf("stored zone = %q, want the refusal to have written nothing", got)
	}
}

// loadZone calls practicetimezone.Load for practiceID in a transaction of
// its own.
func loadZone(t *testing.T, db *testdb.DB, practiceID string) (*time.Location, error) {
	t.Helper()
	return practicetimezone.Load(t.Context(), db.BeginAs(t, testdb.Practice(practiceID)), practiceID) //nolint:wrapcheck // the error under test is Load's own
}

// TestLoad_IsTheSeamEveryCalendarDayComparisonReads covers the reader
// itself rather than the handlers: visit.DeriveType calls it today and
// the manual Payment future-date guard (#1167) is meant to, so it is
// tested as the shared thing it is.
func TestLoad_IsTheSeamEveryCalendarDayComparisonReads(t *testing.T) {
	db := testdb.New(t)
	practiceID := testdb.SeedPractice(t, db, "Zone Reading Practice")

	loc, err := loadZone(t, db, practiceID)
	if err != nil {
		t.Fatalf("Load = %v, want the Practice's zone", err)
	}
	if loc.String() != seededZone {
		t.Fatalf("zone = %q, want %q", loc.String(), seededZone)
	}
}

// TestLoad_RefusesRatherThanAnsweringInTheWrongZone is the rule ADR-0036
// states and the reason Load returns an error at all: a stored name that
// will not load takes the read down rather than quietly standing UTC in
// for it, which would put the derivation back on the wrong day and say
// nothing about it.
func TestLoad_RefusesRatherThanAnsweringInTheWrongZone(t *testing.T) {
	db := testdb.New(t)
	practiceID := testdb.SeedPractice(t, db, "Unloadable Zone Practice")
	testdb.SetPracticeTimezone(t, db, practiceID, "Nowhere/Atlantis")

	if _, err := loadZone(t, db, practiceID); err == nil {
		t.Fatal("Load = nil error, want a refusal for a zone that will not load")
	}
}

// TestRoles_ReadIsAnyStaffAndWriteIsOwnerOrAdmin is the read boundary
// #1280 widened to and the write boundary #1166 asks for, enforced at the
// mount rather than on the screen: a Doula reads the zone the same as an
// Owner or Admin, because InvoiceSection's own future-date ceiling reads
// it too, but may not state it.
func TestRoles_ReadIsAnyStaffAndWriteIsOwnerOrAdmin(t *testing.T) {
	cases := []struct {
		name     string
		staff    fixture.StaffOption
		putWants int
	}{
		{fixture.RoleOwner, fixture.Owner(), http.StatusOK},
		{fixture.RoleAdmin, fixture.Admin(), http.StatusOK},
		{fixture.RoleDoula, fixture.Doula(), http.StatusForbidden},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			e := newEnv(t, tc.staff)

			reqtest.ExpectStatus(t, http.MethodGet, e.url(), http.StatusOK, e.SignedIn)
			reqtest.ExpectStatus(t, http.MethodPut, e.url(), tc.putWants, e.SignedIn, put(denverZone))
		})
	}
}

// TestTimezone_AnotherPracticesZoneIsOutOfReach proves the practice id in
// the path is not what decides whose row is read or written: an Owner at
// one Practice reaching for another's meets the session's own scope, on
// both the read (#1280 widened to AnyStaff) and the write.
func TestTimezone_AnotherPracticesZoneIsOutOfReach(t *testing.T) {
	e := newEnv(t)
	other := fixture.Practice(t, e.DB, fixture.PracticeName("Someone Else's Practice"))
	otherURL := e.URL("/api/practices/" + other.ID + "/timezone")

	for _, tc := range []struct {
		method string
		opts   []reqtest.Option
	}{
		{http.MethodGet, []reqtest.Option{e.SignedIn}},
		{http.MethodPut, []reqtest.Option{e.SignedIn, put(denverZone)}},
	} {
		resp := reqtest.Do(t, tc.method, otherURL, tc.opts...)
		_ = resp.Body.Close()
		if resp.StatusCode == http.StatusOK {
			t.Fatalf("%s status = %d, want a refusal for another Practice's zone", tc.method, resp.StatusCode)
		}
	}
	if got := e.storedZone(other.ID); got != seededZone {
		t.Fatalf("other Practice's zone = %q, want it untouched", got)
	}
}
