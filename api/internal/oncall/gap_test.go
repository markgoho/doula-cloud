package oncall_test

import (
	"net/http"
	"testing"
	"time"

	"doula-cloud/api/internal/oncall"
	"doula-cloud/api/internal/tasknudge"
	"doula-cloud/api/internal/testdb"
)

// Saturday evening inside the solo fixture's 2026-10-09..2026-11-13 window.
var (
	gapStart = time.Date(2026, 10, 17, 22, 0, 0, 0, time.UTC)
	gapEnd   = time.Date(2026, 10, 18, 10, 0, 0, 0, time.UTC)
)

func gapsURL(srvURL, practiceID, engagementID string) string {
	return srvURL + "/api/practices/" + practiceID + "/engagements/" + engagementID + "/coverage-gaps"
}

func gapBody(staffID string, covering *string) oncall.GapRequest {
	reason := "  Sister's wedding  "
	return oncall.GapRequest{StaffID: staffID, StartsAt: &gapStart, EndsAt: &gapEnd, Reason: &reason, CoveringStaffID: covering}
}

// activityActors lists who performed action on engagementID, oldest first.
func activityActors(t *testing.T, db *testdb.DB, engagementID, action string) []string {
	t.Helper()
	rows, err := db.Admin.QueryContext(t.Context(),
		`SELECT actor_staff_id FROM activity WHERE subject_id = $1 AND action = $2 ORDER BY created_at, id`,
		engagementID, action)
	if err != nil {
		t.Fatalf("read activity: %v", err)
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			t.Fatalf("scan activity: %v", err)
		}
		out = append(out, id)
	}
	return out
}

func countRows(t *testing.T, db *testdb.DB, query string, args ...any) int {
	t.Helper()
	var n int
	if err := db.Admin.QueryRowContext(t.Context(), query, args...).Scan(&n); err != nil {
		t.Fatalf("count %q: %v", query, err)
	}
	return n
}

func pendingNotices(t *testing.T, db *testdb.DB, gapID string) int {
	t.Helper()
	return countRows(t, db, `SELECT count(*) FROM coverage_gap_outbox WHERE gap_id = $1 AND status = 'pending'`, gapID)
}

func TestCreateGap_AnUncoveredGapIsRecordedAndQueuesOneNotice(t *testing.T) {
	db := testdb.New(t)
	f := newSoloFixture(t, db, "gap-create")
	ownerID := countOwner(t, db, f)
	srv, session, enq := newServerWithNudge(t, db, f.ownerUID)
	defer srv.Close()

	gap := doJSON[oncall.Gap](t, session, http.MethodPost, gapsURL(srv.URL, f.practiceID, f.engagementID), gapBody(f.doulaID, nil), http.StatusCreated)
	if gap.StaffID != f.doulaID || !gap.StartsAt.Equal(gapStart) || gap.CoveringStaffID != nil {
		t.Fatalf("gap = %+v", gap)
	}
	if gap.Reason == nil || *gap.Reason != "Sister's wedding" {
		t.Errorf("reason = %v, want it trimmed", gap.Reason)
	}
	if got := activityActors(t, db, f.engagementID, "coverage_gap_created"); len(got) != 1 || got[0] != ownerID {
		t.Errorf("created entries = %v, want one naming the Owner %s", got, ownerID)
	}
	if n := pendingNotices(t, db, gap.ID); n != 1 {
		t.Errorf("pending notices = %d, want 1", n)
	}
	if calls := enq.Calls(); len(calls) != 1 || calls[0] != tasknudge.CoverageGap {
		t.Errorf("nudges = %v, want one %s", calls, tasknudge.CoverageGap)
	}
}

// countOwner is the fixture Owner's staff id.
func countOwner(t *testing.T, db *testdb.DB, f soloFixture) string {
	t.Helper()
	var id string
	if err := db.Admin.QueryRowContext(t.Context(), `SELECT id FROM staff WHERE identity_uid = $1`, f.ownerUID).Scan(&id); err != nil {
		t.Fatalf("owner id: %v", err)
	}
	return id
}

func TestCreateGap_ACoveredGapQueuesNothing(t *testing.T) {
	db := testdb.New(t)
	f := newSoloFixture(t, db, "gap-covered")
	backupID := testdb.SeedNamedStaffAtPractice(t, db, f.practiceID, "gap-covered-backup", backupName, []string{doulaRole}, employeeType)
	testdb.SeedGrantedAttachment(t, db, f.engagementID, backupID)
	srv, session, enq := newServerWithNudge(t, db, f.ownerUID)
	defer srv.Close()

	gap := doJSON[oncall.Gap](t, session, http.MethodPost, gapsURL(srv.URL, f.practiceID, f.engagementID), gapBody(f.doulaID, &backupID), http.StatusCreated)
	if gap.CoveringStaffName == nil || *gap.CoveringStaffName != backupName {
		t.Fatalf("covering = %v, want Bo Backup", gap.CoveringStaffName)
	}
	if n := pendingNotices(t, db, gap.ID); n != 0 || len(enq.Calls()) != 0 {
		t.Fatalf("a covered gap queued %d notices and %d nudges, want none", n, len(enq.Calls()))
	}
}

func TestCreateGap_RefusesWhatItShould(t *testing.T) {
	db := testdb.New(t)
	f := newSoloFixture(t, db, "gap-refuse")
	strangerID := testdb.SeedNamedStaffAtPractice(t, db, f.practiceID, "gap-refuse-stranger", "Not On It", []string{doulaRole}, employeeType)
	srv, session := newServer(t, db, f.ownerUID)
	defer srv.Close()

	outside := time.Date(2026, 12, 1, 0, 0, 0, 0, time.UTC)
	early := gapStart.Add(-time.Hour)
	tests := []struct {
		name  string
		body  oncall.GapRequest
		field string
	}{
		{"the Doula is not on the birth", gapBody(strangerID, nil), "staffId"},
		{"the cover is not on the birth, so a gap cannot attach her", gapBody(f.doulaID, &strangerID), "coveringStaffId"},
		{"the cover is the same Doula", gapBody(f.doulaID, &f.doulaID), "coveringStaffId"},
		{"it ends before it starts", oncall.GapRequest{StaffID: f.doulaID, StartsAt: &gapStart, EndsAt: &early}, "endsAt"},
		{"it runs past the window", oncall.GapRequest{StaffID: f.doulaID, StartsAt: &gapStart, EndsAt: &outside}, "startsAt"},
		{"it has no start", oncall.GapRequest{StaffID: f.doulaID, EndsAt: &gapEnd}, "startsAt"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			body := doJSON[map[string]any](t, session, http.MethodPost, gapsURL(srv.URL, f.practiceID, f.engagementID), tt.body, http.StatusBadRequest)
			details, _ := body["details"].(map[string]any)
			if _, ok := details[tt.field]; !ok {
				t.Fatalf("details = %v, want a refusal under %q", details, tt.field)
			}
		})
	}
	if n := countRows(t, db, `SELECT count(*) FROM engagement_coverage_gaps`); n != 0 {
		t.Fatalf("%d gaps were written by refused requests", n)
	}
}

func TestCreateGap_NoWindowIsRefused(t *testing.T) {
	db := testdb.New(t)
	f := newSoloFixture(t, db, "gap-nowindow")
	exec(t, db, `UPDATE engagements SET due_date = NULL WHERE id = $1`, f.engagementID)
	srv, session := newServer(t, db, f.ownerUID)
	defer srv.Close()

	body := doJSON[map[string]any](t, session, http.MethodPost, gapsURL(srv.URL, f.practiceID, f.engagementID), gapBody(f.doulaID, nil), http.StatusConflict)
	if body["message"] != oncall.MsgNoWindow {
		t.Fatalf("message = %v, want %q", body["message"], oncall.MsgNoWindow)
	}
}

func TestCreateGap_ADoulaRecordsOnlyHerOwn(t *testing.T) {
	db := testdb.New(t)
	f := newSoloFixture(t, db, "gap-self")
	const colleagueUID = "gap-self-colleague"
	colleagueID := testdb.SeedNamedStaffAtPractice(t, db, f.practiceID, colleagueUID, backupName, []string{doulaRole}, employeeType)
	testdb.SeedGrantedAttachment(t, db, f.engagementID, colleagueID)
	srv, session := newServer(t, db, colleagueUID)
	defer srv.Close()

	url := gapsURL(srv.URL, f.practiceID, f.engagementID)
	doJSON[map[string]any](t, session, http.MethodPost, url, gapBody(f.doulaID, nil), http.StatusForbidden)
	doJSON[oncall.Gap](t, session, http.MethodPost, url+"?self", gapBody(colleagueID, nil), http.StatusCreated)
}

func TestCreateGap_AContractorReachesOnlyHerOwnBirth(t *testing.T) {
	db := testdb.New(t)
	f := newSoloFixture(t, db, "gap-contractor")
	const contractorUID = "gap-contractor-contractor"
	contractorID := testdb.SeedContractorAtPractice(t, db, f.practiceID, contractorUID)
	srv, session := newServer(t, db, contractorUID)
	defer srv.Close()

	url := gapsURL(srv.URL, f.practiceID, f.engagementID)
	doJSON[map[string]any](t, session, http.MethodPost, url, gapBody(contractorID, nil), http.StatusNotFound)

	testdb.SeedGrantedAttachment(t, db, f.engagementID, contractorID)
	doJSON[oncall.Gap](t, session, http.MethodPost, url+"?attached", gapBody(contractorID, nil), http.StatusCreated)
}
