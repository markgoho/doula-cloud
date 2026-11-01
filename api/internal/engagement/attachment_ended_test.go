package engagement_test

import (
	"encoding/json"
	"net/http"
	"testing"

	"doula-cloud/api/internal/apierr"
	"doula-cloud/api/internal/authntest"
	"doula-cloud/api/internal/engagement"
	"doula-cloud/api/internal/testdb"
)

// The tests in this file are #2270: a contractor whose Attachment on an
// Engagement has ended reads that the work ended and when, at the
// address she had, and nothing else about the Engagement.

func detailRefusal(t *testing.T, db *testdb.DB, srvURL, uid, practiceID, engagementID string) (int, apierr.APIError) {
	t.Helper()
	resp := authedGet(t, authntest.SeedSession(t, db.App, uid), srvURL+"/api/practices/"+practiceID+"/engagements/"+engagementID)
	defer func() { _ = resp.Body.Close() }()
	var body apierr.APIError
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatalf("decode refusal: %v", err)
	}
	return resp.StatusCode, body
}

func endAttachments(t *testing.T, db *testdb.DB, engagementID, endedAt string) {
	t.Helper()
	if _, err := db.Admin.ExecContext(t.Context(),
		`UPDATE engagement_attachments SET ended_at = $2::timestamptz, ended_by = staff_id WHERE engagement_id = $1`,
		engagementID, endedAt); err != nil {
		t.Fatalf("end attachments: %v", err)
	}
}

func seedAcceptedOffer(t *testing.T, db *testdb.DB, engagementID, staffID, dueDate string) {
	t.Helper()
	if _, err := db.Admin.ExecContext(t.Context(),
		`INSERT INTO engagement_offers
		     (engagement_id, staff_id, employment_type, amount_cents, terms, client_first_initial, client_area,
		      due_date, state, offered_by, expires_at, decided_at, decided_by)
		 VALUES ($1, $2, 'contractor', 55000, 'Primary doula', 'A', 'Rochester', $3::date, 'accepted', $2,
		         now() + interval '7 days', now(), $2)`,
		engagementID, staffID, dueDate); err != nil {
		t.Fatalf("seed accepted offer: %v", err)
	}
}

func TestDetailHandler_EndedAttachmentSaysTheWorkEnded(t *testing.T) {
	db := testdb.New(t)
	const uid = "ended-lena"
	srv, practiceID, staffID, engagementID := newAttachSelfServer(t, db, uid, doulaName, []string{doulaRole}, contractorType, engagement.StatusActive)
	if _, err := db.Admin.ExecContext(t.Context(),
		`UPDATE practices SET timezone = 'America/New_York' WHERE id = $1`, practiceID); err != nil {
		t.Fatalf("set timezone: %v", err)
	}
	seedFeeAttachment(t, db, engagementID, staffID, staffID, 55000, "2026-10-01T12:00:00Z")
	seedAcceptedOffer(t, db, engagementID, staffID, "2027-02-14")
	// 02:00 UTC on Nov 1 is still Oct 31 in the Practice's zone.
	endAttachments(t, db, engagementID, "2026-11-01T02:00:00Z")

	status, body := detailRefusal(t, db, srv.URL, uid, practiceID, engagementID)
	if status != http.StatusGone {
		t.Fatalf("status = %d, want 410", status)
	}
	if body.Code != apierr.CodeAttachmentEnded {
		t.Fatalf("code = %q, want %q", body.Code, apierr.CodeAttachmentEnded)
	}
	if want := "Your work on the birth due Feb 2027 ended on Oct 31, 2026."; body.Message != want {
		t.Fatalf("message = %q, want %q", body.Message, want)
	}
	if body.Details != nil {
		t.Fatalf("details = %v, want none: the refusal carries her own record only", body.Details)
	}
}

func TestDetailHandler_EndedAttachmentWithoutADueMonth(t *testing.T) {
	db := testdb.New(t)
	const uid = "ended-no-offer"
	srv, practiceID, staffID, engagementID := newAttachSelfServer(t, db, uid, doulaName, []string{doulaRole}, contractorType, engagement.StatusActive)
	seedFeeAttachment(t, db, engagementID, staffID, staffID, 55000, "2026-10-01T12:00:00Z")
	endAttachments(t, db, engagementID, "2026-10-31T15:00:00Z")

	_, body := detailRefusal(t, db, srv.URL, uid, practiceID, engagementID)
	if want := "Your work on this birth ended on Oct 31, 2026."; body.Message != want {
		t.Fatalf("message = %q, want %q", body.Message, want)
	}

	if _, err := db.Admin.ExecContext(t.Context(),
		`UPDATE engagements SET kind = 'postpartum', due_date = NULL WHERE id = $1`, engagementID); err != nil {
		t.Fatalf("make postpartum: %v", err)
	}
	_, body = detailRefusal(t, db, srv.URL, uid, practiceID, engagementID)
	if want := "Your postpartum work with this Client ended on Oct 31, 2026."; body.Message != want {
		t.Fatalf("message = %q, want %q", body.Message, want)
	}
}

func TestDetailHandler_ContractorNeverOnItStillGetsNotFound(t *testing.T) {
	db := testdb.New(t)
	const uid = "never-on-it"
	srv, practiceID, _, engagementID := newAttachSelfServer(t, db, uid, doulaName, []string{doulaRole}, contractorType, engagement.StatusActive)

	status, body := detailRefusal(t, db, srv.URL, uid, practiceID, engagementID)
	if status != http.StatusNotFound || body.Code != apierr.CodeNotFound {
		t.Fatalf("status, code = %d, %q, want 404 NOT_FOUND", status, body.Code)
	}
}
