package engagement_test

import (
	"net/http"
	"testing"

	"doula-cloud/api/internal/apierr"
	"doula-cloud/api/internal/fixture"
	"doula-cloud/api/internal/reqtest"
)

// The tests in this file are #2270: a contractor whose Attachment on an
// Engagement has ended reads that the work ended and when, at the
// address she had, and nothing else about the Engagement.

// detailRefusal reads the Engagement as the contractor at the screen and
// requires want.
func (e *env) detailRefusal(want int) apierr.APIError {
	e.T.Helper()
	return reqtest.Expect[apierr.APIError](e.T, http.MethodGet, e.engagementURL(""), want, e.SignedIn)
}

func (e *env) endAttachments(endedAt string) {
	e.T.Helper()
	e.Exec(`UPDATE engagement_attachments SET ended_at = $2::timestamptz, ended_by = staff_id WHERE engagement_id = $1`, e.Engagement.ID, endedAt)
}

func (e *env) seedAcceptedOffer(dueDate string) {
	e.T.Helper()
	e.Exec(`INSERT INTO engagement_offers
		     (engagement_id, staff_id, employment_type, amount_cents, terms, client_first_initial, client_area,
		      due_date, state, offered_by, expires_at, decided_at, decided_by)
		 VALUES ($1, $2, 'contractor', 55000, 'Primary doula', 'A', 'Rochester', $3::date, 'accepted', $2,
		         now() + interval '7 days', now(), $2)`,
		e.Engagement.ID, e.Staff.ID, dueDate)
}

// contractorWhoWasPaid is the contractor at the screen, with a fee
// Attachment on an active Engagement.
func contractorWhoWasPaid(t *testing.T) *env {
	t.Helper()
	e := newEnv(t, signedInAs(fixture.Contractor()), withEngagement(fixture.Status(fixture.StatusActive)))
	seedFeeAttachment(t, e.DB, e.Engagement.ID, e.Staff.ID, e.Staff.ID, 55000, "2026-10-01T12:00:00Z")
	return e
}

func TestDetailHandler_EndedAttachmentSaysTheWorkEnded(t *testing.T) {
	e := contractorWhoWasPaid(t)
	e.seedAcceptedOffer("2027-02-14")
	// 02:00 UTC on Nov 1 is still Oct 31 in the Practice's zone.
	e.endAttachments("2026-11-01T02:00:00Z")

	body := e.detailRefusal(http.StatusGone)

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
	e := contractorWhoWasPaid(t)
	e.endAttachments("2026-10-31T15:00:00Z")

	if body := e.detailRefusal(http.StatusGone); body.Message != "Your work on this birth ended on Oct 31, 2026." {
		t.Fatalf("message = %q, want the birth sentence", body.Message)
	}

	e.Exec(`UPDATE engagements SET kind = 'postpartum', due_date = NULL WHERE id = $1`, e.Engagement.ID)

	if body := e.detailRefusal(http.StatusGone); body.Message != "Your postpartum work with this Client ended on Oct 31, 2026." {
		t.Fatalf("message = %q, want the postpartum sentence", body.Message)
	}
}

func TestDetailHandler_ContractorNeverOnItStillGetsNotFound(t *testing.T) {
	e := newEnv(t, signedInAs(fixture.Contractor()), withEngagement(fixture.Status(fixture.StatusActive)))

	if body := e.detailRefusal(http.StatusNotFound); body.Code != apierr.CodeNotFound {
		t.Fatalf("code = %q, want NOT_FOUND", body.Code)
	}
}
