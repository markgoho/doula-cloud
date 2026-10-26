package engagement_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"doula-cloud/api/internal/apierr"
	"doula-cloud/api/internal/authntest"
	"doula-cloud/api/internal/engagement"
	"doula-cloud/api/internal/idempotency"
	"doula-cloud/api/internal/oncall"
	"doula-cloud/api/internal/portal"
	"doula-cloud/api/internal/staffauth"
	"doula-cloud/api/internal/tasknudge"
	"doula-cloud/api/internal/testdb"
)

// The fixture calendar, named once. A birth due on dueBefore has a
// 37-week, 14-day-grace window of 2026-10-09..2026-11-13; moved to
// dueAfter, the same rule draws 2026-10-30..2026-12-04 (the same
// arithmetic oncall's own TestRoster_WindowIsDerivedAndMovesWithTheDueDate
// pins).
const (
	dueBefore = "2026-10-30"
	dueAfter  = "2026-11-20"
)

// Named once for goconst: the kind and outcome tests carry the same
// literals in their own tables.
const (
	noRoleCase          = "no role"
	notAUUIDCase        = "not a uuid"
	unknownEngagementID = "00000000-0000-0000-0000-000000000000"
)

// dueDateResponseBody decodes both shapes this endpoint answers with: the
// success DTO, and apierr's own envelope's Code and Details.
type dueDateResponseBody struct {
	EngagementID string            `json:"engagementId"`
	DueDate      string            `json:"dueDate"`
	Code         apierr.Code       `json:"code"`
	Details      map[string]string `json:"details"`
}

// putDueDate PUTs raw as the body of a due-date change, as uid.
func putDueDate(t *testing.T, db *testdb.DB, srvURL, uid, practiceID, engagementID string, raw []byte) (int, dueDateResponseBody) {
	t.Helper()
	req, err := http.NewRequestWithContext(t.Context(), http.MethodPut,
		srvURL+"/api/practices/"+practiceID+"/engagements/"+engagementID+"/due-date", bytes.NewReader(raw))
	if err != nil {
		t.Fatalf("build request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	authntest.AddSessionCookie(req, authntest.SeedSession(t, db.App, uid))
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()
	var out dueDateResponseBody
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	return resp.StatusCode, out
}

// changeDueDateAs is putDueDate with a well-formed body.
func changeDueDateAs(t *testing.T, db *testdb.DB, srvURL, uid, practiceID, engagementID, dueDate string) (int, dueDateResponseBody) {
	t.Helper()
	encoded, err := json.Marshal(map[string]any{"dueDate": dueDate})
	if err != nil {
		t.Fatalf("marshal body: %v", err)
	}
	return putDueDate(t, db, srvURL, uid, practiceID, engagementID, encoded)
}

// readEngagementDueDate is the raw column, bypassing the API; "" for null.
func readEngagementDueDate(t *testing.T, db *testdb.DB, engagementID string) string {
	t.Helper()
	var due *string
	if err := db.Admin.QueryRowContext(t.Context(),
		`SELECT due_date::text FROM engagements WHERE id = $1`, engagementID,
	).Scan(&due); err != nil {
		t.Fatalf("read engagement due date: %v", err)
	}
	if due == nil {
		return ""
	}
	return *due
}

// setDueDate writes the column directly, "" for null -- the fixture's
// starting point, not the act under test.
func setDueDate(t *testing.T, db *testdb.DB, engagementID, dueDate string) {
	t.Helper()
	if _, err := db.Admin.ExecContext(t.Context(),
		`UPDATE engagements SET due_date = nullif($2, '')::date WHERE id = $1`, engagementID, dueDate,
	); err != nil {
		t.Fatalf("set due date: %v", err)
	}
}

// newDueDateServer mounts this package's whole surface and seeds a birth
// Engagement due on dueBefore at a new Practice, with uid holding roles.
func newDueDateServer(t *testing.T, db *testdb.DB, uid string, roles []string, employmentType string) (srv *httptest.Server, practiceID, engagementID string) {
	t.Helper()
	srv, practiceID, engagementID = newKindServer(t, db, uid, roles, employmentType)
	setDueDate(t, db, engagementID, dueBefore)
	return srv, practiceID, engagementID
}

// countDueDateEntries counts this Engagement's due_date_changed rows.
func countDueDateEntries(t *testing.T, db *testdb.DB, engagementID string) int {
	t.Helper()
	return countActivityActions(t, db, engagementID, "due_date_changed")
}

// TestChangeDueDateHandler_RolesMayChange is ADR-0015's role table for
// the Engagement's mutable facts, read the same way kind's own test
// reads it (refuseFactWrite): an Owner, an Admin and an employee Doula
// may change the due date; a contractor Doula and a member with no role
// are refused at the endpoint, and nothing is written for them. Every
// allowed change leaves one Activity row naming who changed it, from
// what and to what (readFactDiff fails on a row with no actor; the row's
// created_at is the when).
func TestChangeDueDateHandler_RolesMayChange(t *testing.T) {
	cases := []struct {
		role           string
		roles          []string
		employmentType string
		wantOK         bool
	}{
		{ownerRole, []string{ownerRole}, employeeType, true},
		{adminRole, []string{adminRole}, employeeType, true},
		{employeeDoulaKind, []string{doulaRole}, employeeType, true},
		{contractorDoulaKind, []string{doulaRole}, contractorType, false},
		{noRoleCase, []string{}, employeeType, false},
	}
	for _, tc := range cases {
		t.Run(tc.role, func(t *testing.T) {
			db := testdb.New(t)
			uid := "change-due-date-" + tc.role
			srv, practiceID, engagementID := newDueDateServer(t, db, uid, tc.roles, tc.employmentType)

			status, body := changeDueDateAs(t, db, srv.URL, uid, practiceID, engagementID, dueAfter)

			if !tc.wantOK {
				if status != http.StatusForbidden {
					t.Fatalf("status = %d, want 403", status)
				}
				if got := readEngagementDueDate(t, db, engagementID); got != dueBefore {
					t.Fatalf("due date = %q, want left unchanged at %s", got, dueBefore)
				}
				if n := countDueDateEntries(t, db, engagementID); n != 0 {
					t.Fatalf("due_date_changed entries = %d, want 0", n)
				}
				return
			}
			if status != http.StatusOK {
				t.Fatalf("status = %d, want 200", status)
			}
			if body.DueDate != dueAfter || body.EngagementID != engagementID {
				t.Fatalf("body = %+v, want %s on %s", body, dueAfter, engagementID)
			}
			if got := readEngagementDueDate(t, db, engagementID); got != dueAfter {
				t.Fatalf("due date = %q, want %s", got, dueAfter)
			}
			if n := countDueDateEntries(t, db, engagementID); n != 1 {
				t.Fatalf("due_date_changed entries = %d, want 1", n)
			}
			assertDiff(t, readFactDiff(t, db, engagementID, "due_date_changed"), map[string]any{
				"dueDateBefore": dueBefore,
				"dueDateAfter":  dueAfter,
			})
		})
	}
}

// TestChangeDueDateHandler_FirstDueDate covers the nullable column: an
// Engagement with no due date yet is given one, and the Activity row
// records the before side as JSON null rather than leaving it out.
func TestChangeDueDateHandler_FirstDueDate(t *testing.T) {
	db := testdb.New(t)
	const uid = "first-due-date-owner"
	srv, practiceID, engagementID := newKindServer(t, db, uid, []string{ownerRole}, employeeType)

	status, body := changeDueDateAs(t, db, srv.URL, uid, practiceID, engagementID, dueAfter)
	if status != http.StatusOK || body.DueDate != dueAfter {
		t.Fatalf("status = %d, body = %+v, want 200 with %s", status, body, dueAfter)
	}
	assertDiff(t, readFactDiff(t, db, engagementID, "due_date_changed"), map[string]any{
		"dueDateBefore": nil,
		"dueDateAfter":  dueAfter,
	})
}

// TestChangeDueDateHandler_NoOp proves the PUT is naturally idempotent:
// re-sending the date the Engagement already holds writes nothing and
// raises no second Activity row (docs/api-design.md rule 4).
func TestChangeDueDateHandler_NoOp(t *testing.T) {
	db := testdb.New(t)
	const uid = "due-date-no-op-owner"
	srv, practiceID, engagementID := newDueDateServer(t, db, uid, []string{ownerRole}, employeeType)

	for range 2 {
		if status, body := changeDueDateAs(t, db, srv.URL, uid, practiceID, engagementID, dueAfter); status != http.StatusOK || body.DueDate != dueAfter {
			t.Fatalf("status = %d, body = %+v", status, body)
		}
	}
	if n := countDueDateEntries(t, db, engagementID); n != 1 {
		t.Fatalf("due_date_changed entries = %d, want 1 -- the resend writes nothing", n)
	}

	if status, _ := changeDueDateAs(t, db, srv.URL, uid, practiceID, engagementID, dueBefore); status != http.StatusOK {
		t.Fatalf("change back: status = %d, want 200", status)
	}
	if n := countDueDateEntries(t, db, engagementID); n != 2 {
		t.Fatalf("due_date_changed entries = %d, want 2", n)
	}
}

// TestChangeDueDateHandler_Validation proves a due date is a calendar
// day, YYYY-MM-DD, and nothing else: an empty value (this endpoint sets a
// date, it never clears one), an impossible day and free text are each
// refused with a 400 naming the dueDate field, and none writes a thing.
func TestChangeDueDateHandler_Validation(t *testing.T) {
	cases := []struct{ name, dueDate string }{
		{"empty", "   "},
		{"impossible day", "2027-02-30"},
		{"free text", "next week"},
		{"an instant", "2027-03-01T00:00:00Z"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			db := testdb.New(t)
			uid := "due-date-validation-" + tc.name
			srv, practiceID, engagementID := newDueDateServer(t, db, uid, []string{ownerRole}, employeeType)

			status, body := changeDueDateAs(t, db, srv.URL, uid, practiceID, engagementID, tc.dueDate)
			if status != http.StatusBadRequest {
				t.Fatalf("status = %d, want 400", status)
			}
			if body.Code != apierr.CodeInvalidArgument || body.Details["dueDate"] != engagement.MsgDueDateMalformed {
				t.Fatalf("body = %+v, want %s naming dueDate", body, apierr.CodeInvalidArgument)
			}
			if got := readEngagementDueDate(t, db, engagementID); got != dueBefore {
				t.Fatalf("due date = %q, want left unchanged", got)
			}
			if n := countDueDateEntries(t, db, engagementID); n != 0 {
				t.Fatalf("due_date_changed entries = %d, want 0", n)
			}
		})
	}
}

// TestChangeDueDateHandler_UnknownEngagement covers both doors an
// engagement id can fail at, and the decode refusal.
func TestChangeDueDateHandler_UnknownEngagement(t *testing.T) {
	cases := []struct {
		name         string
		engagementID string
		body         []byte
		want         int
	}{
		{notAUUIDCase, "due-date-not-an-id", []byte(`{"dueDate":"2026-11-20"}`), http.StatusBadRequest},
		{"nobody's engagement", unknownEngagementID, []byte(`{"dueDate":"2026-11-20"}`), http.StatusNotFound},
		{"malformed body", "", []byte("{"), http.StatusBadRequest},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			db := testdb.New(t)
			uid := "missing-due-date-engagement-" + tc.name
			srv, practiceID, engagementID := newDueDateServer(t, db, uid, []string{ownerRole}, employeeType)
			if tc.engagementID != "" {
				engagementID = tc.engagementID
			}

			status, _ := putDueDate(t, db, srv.URL, uid, practiceID, engagementID, tc.body)
			if status != tc.want {
				t.Fatalf("status = %d, want %d", status, tc.want)
			}
		})
	}
}

// newCrossReadServer mounts this package beside the two other readers of
// the due date -- oncall (the On-call window) and portal (the Client's
// own read) -- on one mux, the way main.go mounts them together.
func newCrossReadServer(t *testing.T, db *testdb.DB) *httptest.Server {
	t.Helper()
	mux := http.NewServeMux()
	g := staffauth.NewGatedRouter(mux, db.App)
	ir := idempotency.NewRouter(g, db.App)
	engagement.Mount(g, ir)
	oncall.Mount(g, ir, &tasknudge.FakeEnqueuer{})
	portal.Mount(g, db.App)
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return srv
}

// getAs reads url as session into T, failing on any status but 200.
func getAs[T any](t *testing.T, session, url string) T {
	t.Helper()
	req, err := http.NewRequestWithContext(t.Context(), http.MethodGet, url, nil)
	if err != nil {
		t.Fatalf("build request: %v", err)
	}
	authntest.AddSessionCookie(req, session)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("GET %s: status = %d, want 200", url, resp.StatusCode)
	}
	var out T
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		t.Fatalf("decode: %v", err)
	}
	return out
}

// TestChangeDueDateHandler_EveryReaderFollows is #2013's third AC: what
// reads the due date reads the new one on its next read, through its own
// endpoint -- the Staff Engagement read behind the summary card, the
// On-call window (derived, never stored, so it moves with no write of
// its own), and the Client's portal read of her care.
func TestChangeDueDateHandler_EveryReaderFollows(t *testing.T) {
	db := testdb.New(t)
	const uid = "due-date-readers-owner"
	practiceID, _ := testdb.SeedStaffAtNewPractice(t, db, uid, []string{ownerRole}, employeeType)
	testdb.SetPracticeTimezone(t, db, practiceID, "America/New_York")
	doulaID := testdb.SeedNamedStaffAtPractice(t, db, practiceID, "due-date-readers-doula", "Maya Primary", []string{doulaRole}, employeeType)
	clientID, engagementID := testdb.SeedEngagementWithKind(t, db, practiceID, "Ada Whitfield", "ada@example.com", string(engagement.KindBirth))
	setDueDate(t, db, engagementID, dueBefore)
	testdb.SeedGrantedAttachment(t, db, engagementID, doulaID)

	const portalUID = "due-date-readers-client"
	clientUID := testdb.PortalUID(portalUID)
	testdb.SeedPortalAccount(t, db, clientUID, clientUID+"@example.com")
	if _, err := db.Admin.ExecContext(t.Context(),
		`INSERT INTO client_portal_users (identity_uid, client_id) VALUES ($1, $2)`, clientUID, clientID,
	); err != nil {
		t.Fatalf("seed client_portal_users: %v", err)
	}

	srv := newCrossReadServer(t, db)
	staffSession := authntest.SeedSession(t, db.App, uid)
	clientSession := authntest.SeedSession(t, db.App, clientUID)
	engagementURL := srv.URL + "/api/practices/" + practiceID + "/engagements/" + engagementID

	before := getAs[oncall.EngagementOnCall](t, staffSession, engagementURL+"/on-call")
	if before.Window == nil || *before.Window != (oncall.Window{Start: "2026-10-09", End: "2026-11-13"}) {
		t.Fatalf("window before = %+v, want 2026-10-09..2026-11-13", before.Window)
	}

	if status, _ := changeDueDateAs(t, db, srv.URL, uid, practiceID, engagementID, dueAfter); status != http.StatusOK {
		t.Fatalf("change: status = %d, want 200", status)
	}

	if d := getAs[engagement.Detail](t, staffSession, engagementURL); d.DueDate == nil || *d.DueDate != dueAfter {
		t.Fatalf("staff detail dueDate = %v, want %s", d.DueDate, dueAfter)
	}
	after := getAs[oncall.EngagementOnCall](t, staffSession, engagementURL+"/on-call")
	if after.Window == nil || *after.Window != (oncall.Window{Start: "2026-10-30", End: "2026-12-04"}) {
		t.Fatalf("window after = %+v, want 2026-10-30..2026-12-04", after.Window)
	}
	if p := getAs[portal.Detail](t, clientSession, srv.URL+"/api/portal/engagements/"+engagementID); p.DueDate == nil || *p.DueDate != dueAfter {
		t.Fatalf("portal dueDate = %v, want %s", p.DueDate, dueAfter)
	}
}
