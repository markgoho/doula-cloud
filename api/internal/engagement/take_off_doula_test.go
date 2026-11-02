package engagement_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"doula-cloud/api/internal/activity"
	"doula-cloud/api/internal/authntest"
	"doula-cloud/api/internal/engagement"
	"doula-cloud/api/internal/testdb"
)

// The tests in this file are #2596: an Owner or an Admin takes a Doula off
// an Engagement. The Attachment is ended, never deleted; the ledger says
// who and when; the Doula gets one mail, unless she is the one who
// pressed; her booked Visits stay hers.

func takeOffDoulaAs(t *testing.T, db *testdb.DB, srv *httptest.Server, uid, practiceID, engagementID, staffID string) (int, attachSelfBody) {
	t.Helper()
	req, err := http.NewRequestWithContext(t.Context(), http.MethodDelete, attachDoulaURL(srv.URL, practiceID, engagementID, staffID), http.NoBody)
	if err != nil {
		t.Fatalf("build request: %v", err)
	}
	authntest.AddSessionCookie(req, authntest.SeedSession(t, db.App, uid))
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()
	var out attachSelfBody
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	return resp.StatusCode, out
}

func countTakenOffNotices(t *testing.T, db *testdb.DB, engagementID, staffID string) int {
	t.Helper()
	var n int
	if err := db.Admin.QueryRowContext(t.Context(),
		`SELECT count(*) FROM staff_notice_outbox
		  WHERE kind = 'taken_off_engagement' AND subject_id = $1 AND recipient_staff_id = $2`,
		engagementID, staffID).Scan(&n); err != nil {
		t.Fatalf("count notices: %v", err)
	}
	return n
}

func seedVisitAt(t *testing.T, db *testdb.DB, engagementID, staffID, when string) {
	t.Helper()
	if _, err := db.Admin.ExecContext(t.Context(),
		`INSERT INTO visits (engagement_id, staff_id, scheduled_at) VALUES ($1, $2, $3::timestamptz)`,
		engagementID, staffID, when); err != nil {
		t.Fatalf("seed visit: %v", err)
	}
}

func TestTakeOffDoulaHandler_EndsTheAttachmentAndRecordsIt(t *testing.T) {
	db := testdb.New(t)
	const uid = "take-off-doula"
	srv, practiceID, callerID, doulaID, engagementID := newAttachDoulaServer(t, db, uid, []string{ownerRole}, []string{doulaRole}, employeeType, engagement.StatusActive)
	attachDoulaAs(t, db, srv, uid, practiceID, engagementID, doulaID)
	seedVisitAt(t, db, engagementID, doulaID, "2999-01-01T12:00:00Z")
	seedVisitAt(t, db, engagementID, doulaID, "2001-01-01T12:00:00Z")

	_, on := attachDoulaAs(t, db, srv, uid, practiceID, engagementID, doulaID)
	if len(on.Doulas) != 1 || on.Doulas[0].BookedVisits != 1 {
		t.Fatalf("doulas before = %+v, want one with 1 booked Visit", on.Doulas)
	}

	status, body := takeOffDoulaAs(t, db, srv, uid, practiceID, engagementID, doulaID)
	if status != http.StatusOK || len(body.Doulas) != 0 {
		t.Fatalf("status = %d doulas = %+v, want 200 and nobody on it", status, body.Doulas)
	}
	var endedBy string
	var rows int
	if err := db.Admin.QueryRowContext(t.Context(),
		`SELECT count(*), max(ended_by::text) FROM engagement_attachments
		  WHERE engagement_id = $1 AND staff_id = $2 AND ended_at IS NOT NULL`,
		engagementID, doulaID).Scan(&rows, &endedBy); err != nil {
		t.Fatalf("read ended attachment: %v", err)
	}
	if rows != 1 || endedBy != callerID {
		t.Fatalf("ended rows = %d by %q, want one ended by the caller", rows, endedBy)
	}
	assertDiff(t, readFactDiff(t, db, engagementID, "doula_taken_off"), map[string]any{activity.DiffKeyAttachedStaffID: doulaID})
	ledger := readActivity(t, db, practiceID, engagementID, uid)
	var found bool
	for _, entry := range ledger.Items {
		if entry.Action == "doula_taken_off" {
			found = true
			if entry.ActorName != attachingOwnerName || entry.Detail != "Taken off this Engagement as the Doula: "+attachedDoulaName || entry.CreatedAt.IsZero() {
				t.Fatalf("ledger entry = %+v, want who, whom and when", entry)
			}
		}
	}
	if !found {
		t.Fatalf("ledger = %+v, want a doula_taken_off entry", ledger.Items)
	}
	if n := countTakenOffNotices(t, db, engagementID, doulaID); n != 1 {
		t.Fatalf("notices = %d, want 1", n)
	}
	var visits int
	if err := db.Admin.QueryRowContext(t.Context(),
		`SELECT count(*) FROM visits WHERE engagement_id = $1 AND staff_id = $2`, engagementID, doulaID).Scan(&visits); err != nil || visits != 2 {
		t.Fatalf("visits = %d (%v), want both still hers", visits, err)
	}
}

func TestTakeOffDoulaHandler_SafeToRepeat(t *testing.T) {
	db := testdb.New(t)
	const uid = "take-off-doula-repeat"
	srv, practiceID, _, doulaID, engagementID := newAttachDoulaServer(t, db, uid, []string{ownerRole}, []string{doulaRole}, employeeType, engagement.StatusActive)
	attachDoulaAs(t, db, srv, uid, practiceID, engagementID, doulaID)

	first, _ := takeOffDoulaAs(t, db, srv, uid, practiceID, engagementID, doulaID)
	second, _ := takeOffDoulaAs(t, db, srv, uid, practiceID, engagementID, doulaID)
	if first != http.StatusOK || second != http.StatusOK {
		t.Fatalf("statuses = %d, %d, want 200 twice", first, second)
	}
	if n := countActivityActions(t, db, engagementID, "doula_taken_off"); n != 1 {
		t.Fatalf("doula_taken_off entries = %d, want 1", n)
	}
	if n := countTakenOffNotices(t, db, engagementID, doulaID); n != 1 {
		t.Fatalf("notices = %d, want 1", n)
	}
}

func TestTakeOffDoulaHandler_NobodyIsMailedAboutHerOwnTakingOff(t *testing.T) {
	db := testdb.New(t)
	const uid = "take-off-doula-self"
	srv, practiceID, callerID, _, engagementID := newAttachDoulaServer(t, db, uid, []string{ownerRole, doulaRole}, []string{doulaRole}, employeeType, engagement.StatusActive)
	attachDoulaAs(t, db, srv, uid, practiceID, engagementID, callerID)

	if status, _ := takeOffDoulaAs(t, db, srv, uid, practiceID, engagementID, callerID); status != http.StatusOK {
		t.Fatalf("status = %d, want 200", status)
	}
	if n := countActivityActions(t, db, engagementID, "doula_taken_off"); n != 1 {
		t.Fatalf("doula_taken_off entries = %d, want 1", n)
	}
	if n := countTakenOffNotices(t, db, engagementID, callerID); n != 0 {
		t.Fatalf("notices = %d, want none for the person who pressed", n)
	}
}

func TestTakeOffDoulaHandler_RefusalsAndRoles(t *testing.T) {
	db := testdb.New(t)
	const uid = "take-off-doula-refusals"
	srv, practiceID, _, doulaID, engagementID := newAttachDoulaServer(t, db, uid, []string{ownerRole}, []string{doulaRole}, employeeType, engagement.StatusActive)
	attachDoulaAs(t, db, srv, uid, practiceID, engagementID, doulaID)
	const missing = "11111111-1111-1111-1111-111111111111"

	cases := []struct {
		name         string
		engagementID string
		staffID      string
		wantStatus   int
	}{
		{"unknown engagement", missing, doulaID, http.StatusNotFound},
		{"malformed engagement", malformedUUID, doulaID, http.StatusBadRequest},
		{"malformed staff", engagementID, malformedUUID, http.StatusBadRequest},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if status, _ := takeOffDoulaAs(t, db, srv, uid, practiceID, tc.engagementID, tc.staffID); status != tc.wantStatus {
				t.Fatalf("status = %d, want %d", status, tc.wantStatus)
			}
		})
	}

	t.Run("a doula cannot take a colleague off", func(t *testing.T) {
		doulaUID := uid + "-doula"
		if status, _ := takeOffDoulaAs(t, db, srv, doulaUID, practiceID, engagementID, doulaID); status != http.StatusForbidden {
			t.Fatalf("status = %d, want 403", status)
		}
		if got := readOpenAttachments(t, db, engagementID, doulaID); len(got) != 1 {
			t.Fatalf("attachment = %+v, want it still open", got)
		}
	})
}
