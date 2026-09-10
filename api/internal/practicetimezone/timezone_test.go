package practicetimezone_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"doula-cloud/api/internal/authntest"
	"doula-cloud/api/internal/idempotency"
	"doula-cloud/api/internal/practicetimezone"
	"doula-cloud/api/internal/staffauth"
	"doula-cloud/api/internal/testdb"
)

// The roles and employment types this package's tests seed a Membership
// with, named once so golangci-lint's goconst check doesn't see the same
// literal repeated across the table-driven role cases.
const (
	ownerRole    = "owner"
	adminRole    = "admin"
	doulaRole    = "doula"
	employeeType = "employee"
)

// seededZone is what testdb.SeedPractice states for a fixture Practice,
// so it is what every one of these tests reads before it writes.
const seededZone = "America/New_York"

// denverZone is the zone these tests move a Practice to: a real IANA name,
// two hours off the seeded one, so a wrong day is a visible day rather
// than an hour's difference.
const denverZone = "America/Denver"

func newServer(t *testing.T, db *testdb.DB, uid string) (srv *httptest.Server, session string) {
	t.Helper()
	mux := http.NewServeMux()
	g := staffauth.NewGatedRouter(mux, db.App)
	ir := idempotency.NewRouter(g, db.App)
	practicetimezone.Mount(g, ir)
	return httptest.NewServer(mux), authntest.SeedSession(t, db.App, uid)
}

func getTimezone(t *testing.T, srv *httptest.Server, session, practiceID string) *http.Response {
	t.Helper()
	req, err := http.NewRequestWithContext(t.Context(), http.MethodGet, srv.URL+"/api/practices/"+practiceID+"/timezone", nil)
	if err != nil {
		t.Fatalf("build request: %v", err)
	}
	authntest.AddSessionCookie(req, session)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	return resp
}

func putTimezone(t *testing.T, srv *httptest.Server, session, practiceID, zone string) *http.Response {
	t.Helper()
	payload, err := json.Marshal(practicetimezone.PutRequest{Timezone: zone})
	if err != nil {
		t.Fatalf("marshal body: %v", err)
	}
	req, err := http.NewRequestWithContext(t.Context(), http.MethodPut, srv.URL+"/api/practices/"+practiceID+"/timezone", bytes.NewReader(payload))
	if err != nil {
		t.Fatalf("build request: %v", err)
	}
	authntest.AddSessionCookie(req, session)
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	return resp
}

func storedZone(t *testing.T, db *testdb.DB, practiceID string) string {
	t.Helper()
	var zone string
	if err := db.Admin.QueryRowContext(t.Context(),
		`SELECT timezone FROM practices WHERE id = $1`, practiceID,
	).Scan(&zone); err != nil {
		t.Fatalf("read stored zone: %v", err)
	}
	return zone
}

func decodeDetails(t *testing.T, resp *http.Response) map[string]string {
	t.Helper()
	var out struct {
		Details map[string]string `json:"details"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		t.Fatalf("decode refusal: %v", err)
	}
	return out.Details
}

// TestGetHandler_ReadsThePracticesOwnZone proves an Owner reads the zone
// the Practice actually holds, rather than a zone the BFF assumes.
func TestGetHandler_ReadsThePracticesOwnZone(t *testing.T) {
	db := testdb.New(t)
	const uid = "timezone-owner-read"
	practiceID, _ := testdb.SeedStaffAtNewPractice(t, db, uid, []string{ownerRole}, employeeType)

	srv, session := newServer(t, db, uid)
	defer srv.Close()

	resp := getTimezone(t, srv, session, practiceID)
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}
	var out practicetimezone.Response
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if out.Timezone != seededZone {
		t.Fatalf("timezone = %q, want %q", out.Timezone, seededZone)
	}
}

// TestPutHandler_OwnerStatesAZone is the happy path: the row moves and
// the answer names the zone that is now stored.
func TestPutHandler_OwnerStatesAZone(t *testing.T) {
	db := testdb.New(t)
	const uid = "timezone-owner-write"
	practiceID, _ := testdb.SeedStaffAtNewPractice(t, db, uid, []string{ownerRole}, employeeType)

	srv, session := newServer(t, db, uid)
	defer srv.Close()

	resp := putTimezone(t, srv, session, practiceID, denverZone)
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}
	var out practicetimezone.Response
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if out.Timezone != denverZone {
		t.Fatalf("timezone = %q, want %q", out.Timezone, denverZone)
	}
	if got := storedZone(t, db, practiceID); got != denverZone {
		t.Fatalf("stored zone = %q, want %q", got, denverZone)
	}
}

// TestPutHandler_RecordsWhoChangedItAndFromWhat is the audit trail
// CLAUDE.md asks every feature to carry: the Activity row names the
// acting Staff member and both sides of the change, so "how did this
// Practice come to be in Denver" has an answer.
func TestPutHandler_RecordsWhoChangedItAndFromWhat(t *testing.T) {
	db := testdb.New(t)
	const uid = "timezone-activity"
	practiceID, staffID := testdb.SeedStaffAtNewPractice(t, db, uid, []string{ownerRole}, employeeType)

	srv, session := newServer(t, db, uid)
	defer srv.Close()

	resp := putTimezone(t, srv, session, practiceID, denverZone)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}

	var actorStaffID string
	var diff []byte
	if err := db.Admin.QueryRowContext(t.Context(),
		`SELECT actor_staff_id, diff FROM activity
		 WHERE practice_id = $1 AND action = 'practice_timezone_changed'`, practiceID,
	).Scan(&actorStaffID, &diff); err != nil {
		t.Fatalf("read activity row: %v", err)
	}
	if actorStaffID != staffID {
		t.Fatalf("actor = %q, want the Staff member who pressed it (%q)", actorStaffID, staffID)
	}
	var recorded struct {
		Before string `json:"timezoneBefore"`
		After  string `json:"timezoneAfter"`
	}
	if err := json.Unmarshal(diff, &recorded); err != nil {
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
	db := testdb.New(t)
	const uid = "timezone-idempotent"
	practiceID, _ := testdb.SeedStaffAtNewPractice(t, db, uid, []string{ownerRole}, employeeType)

	srv, session := newServer(t, db, uid)
	defer srv.Close()

	for range 2 {
		resp := putTimezone(t, srv, session, practiceID, denverZone)
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusOK)
		}
		resp.Body.Close()
	}

	var rows int
	if err := db.Admin.QueryRowContext(t.Context(),
		`SELECT count(*) FROM activity WHERE practice_id = $1 AND action = 'practice_timezone_changed'`,
		practiceID,
	).Scan(&rows); err != nil {
		t.Fatalf("count activity rows: %v", err)
	}
	if rows != 1 {
		t.Fatalf("activity rows = %d, want 1 -- the second press changed nothing", rows)
	}
}

// TestPutHandler_RefusesAZoneTheDatabaseDoesNotName is the boundary the
// screen cannot enforce: the API, not the select, decides what a zone is.
// The three cases are the three ways a name can be wrong -- one the IANA
// database has never carried, the empty string that time.LoadLocation
// answers as UTC, and "Local", which is the server's own zone rather
// than the Practice's.
func TestPutHandler_RefusesAZoneTheDatabaseDoesNotName(t *testing.T) {
	for _, zone := range []string{"Nowhere/Atlantis", "", "Local"} {
		t.Run("zone "+zone, func(t *testing.T) {
			db := testdb.New(t)
			uid := "timezone-refused-" + zone
			practiceID, _ := testdb.SeedStaffAtNewPractice(t, db, uid, []string{ownerRole}, employeeType)

			srv, session := newServer(t, db, uid)
			defer srv.Close()

			resp := putTimezone(t, srv, session, practiceID, zone)
			defer resp.Body.Close()

			if resp.StatusCode != http.StatusBadRequest {
				t.Fatalf("status = %d, want %d for zone %q", resp.StatusCode, http.StatusBadRequest, zone)
			}
			if details := decodeDetails(t, resp); details["timezone"] == "" {
				t.Fatalf("details = %v, want an entry keyed by the DTO's own timezone field", details)
			}
			if got := storedZone(t, db, practiceID); got != seededZone {
				t.Fatalf("stored zone = %q, want the refusal to have written nothing", got)
			}
		})
	}
}

// TestTimezone_OnlyAnOwnerOrAdminReachesIt is the role boundary #1166
// asks for, enforced at the mount rather than on the screen: a Doula is
// refused both the read and the write.
func TestTimezone_OnlyAnOwnerOrAdminReachesIt(t *testing.T) {
	for _, tc := range []struct {
		role       string
		wantStatus int
	}{
		{ownerRole, http.StatusOK},
		{adminRole, http.StatusOK},
		{doulaRole, http.StatusForbidden},
	} {
		t.Run(tc.role, func(t *testing.T) {
			db := testdb.New(t)
			uid := "timezone-role-" + tc.role
			practiceID, _ := testdb.SeedStaffAtNewPractice(t, db, uid, []string{tc.role}, employeeType)

			srv, session := newServer(t, db, uid)
			defer srv.Close()

			read := getTimezone(t, srv, session, practiceID)
			read.Body.Close()
			if read.StatusCode != tc.wantStatus {
				t.Fatalf("GET status for %s = %d, want %d", tc.role, read.StatusCode, tc.wantStatus)
			}

			write := putTimezone(t, srv, session, practiceID, denverZone)
			write.Body.Close()
			if write.StatusCode != tc.wantStatus {
				t.Fatalf("PUT status for %s = %d, want %d", tc.role, write.StatusCode, tc.wantStatus)
			}
		})
	}
}

// TestTimezone_AnotherPracticesZoneIsOutOfReach proves the practice id in
// the path is not what decides whose row is read or written: an Owner at
// one Practice reaching for another's meets the session's own scope.
func TestTimezone_AnotherPracticesZoneIsOutOfReach(t *testing.T) {
	db := testdb.New(t)
	const uid = "timezone-cross-practice"
	_, _ = testdb.SeedStaffAtNewPractice(t, db, uid, []string{ownerRole}, employeeType)
	otherPracticeID := testdb.SeedPractice(t, db, "Someone Else's Practice")

	srv, session := newServer(t, db, uid)
	defer srv.Close()

	resp := putTimezone(t, srv, session, otherPracticeID, denverZone)
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusOK {
		t.Fatalf("status = %d, want a refusal for another Practice's zone", resp.StatusCode)
	}
	if got := storedZone(t, db, otherPracticeID); got != seededZone {
		t.Fatalf("other Practice's zone = %q, want it untouched", got)
	}
}
