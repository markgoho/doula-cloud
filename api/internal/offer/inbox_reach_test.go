package offer_test

import (
	"net/http"
	"testing"

	"doula-cloud/api/internal/offer"
	"doula-cloud/api/internal/testdb"
)

// The tests in this file are #2269 and #2270 on her own inbox: an
// accepted Offer names the Engagement it put her on while she reaches
// it, and says the day her Attachment ended once it has.

func inbox(t *testing.T, f fixture, session string) offer.Summary {
	t.Helper()
	var listed offer.ListResponse
	decode(t, do(t, http.MethodGet, f.srv+"/api/practices/"+f.practiceID+"/offers", session, nil), http.StatusOK, &listed)
	if len(listed.Items) != 1 {
		t.Fatalf("offers = %+v, want one", listed.Items)
	}
	return listed.Items[0]
}

func completeEngagement(t *testing.T, f fixture) {
	t.Helper()
	testdb.SeedBirthOutcome(t, f.db, f.engagementID)
	expectStatus(t, do(t, http.MethodPatch,
		f.srv+"/api/practices/"+f.practiceID+"/engagements/"+f.engagementID+"/status", f.ownerSession,
		completeStatusBody),
		http.StatusOK)
}

func TestInboxHandler_ContractorLinksToTheEngagementUntilHerAttachmentEnds(t *testing.T) {
	f := newFixture(t)
	offerID := f.makeOffer(t, offerBody(f.doulaID, 45000))

	if got := inbox(t, f, f.doulaSession); got.EngagementID != "" {
		t.Fatalf("open offer engagementId = %q, want none", got.EngagementID)
	}

	expectStatus(t, do(t, http.MethodPost, f.offerURL(offerID, "accept"), f.doulaSession, nil), http.StatusOK)
	got := inbox(t, f, f.doulaSession)
	if got.EngagementID != f.engagementID || got.EndedOn != "" {
		t.Fatalf("accepted offer = %+v, want the Engagement and no end", got)
	}

	completeEngagement(t, f)
	got = inbox(t, f, f.doulaSession)
	if got.EngagementID != "" {
		t.Fatalf("engagementId = %q, want none once her reach is gone", got.EngagementID)
	}
	var today string
	if err := f.db.Admin.QueryRowContext(t.Context(),
		`SELECT (now() AT TIME ZONE p.timezone)::date::text FROM practices p WHERE p.id = $1`, f.practiceID,
	).Scan(&today); err != nil {
		t.Fatalf("read practice day: %v", err)
	}
	if got.EndedOn != today {
		t.Fatalf("endedOn = %q, want %q (the Practice's day)", got.EndedOn, today)
	}

	var practiceSide offer.ListResponse
	decode(t, do(t, http.MethodGet, f.offersURL(), f.ownerSession, nil), http.StatusOK, &practiceSide)
	if p := practiceSide.Items[0]; p.EngagementID != "" || p.EndedOn != "" {
		t.Fatalf("practice-side offer = %+v, want neither field", p)
	}
}

func TestInboxHandler_EmployeeKeepsTheLinkAfterCompletion(t *testing.T) {
	f := newFixture(t)
	employeeID := testdb.SeedStaffAtPractice(t, f.db, f.practiceID, "uid-employee-reach", []string{doulaRole}, employeeType)
	employeeSession := seedSessionFor(t, f.db, "uid-employee-reach")
	body := offerBody(employeeID, 0)
	body.AmountCents = nil
	body.Terms = ""
	offerID := f.makeOffer(t, body)
	expectStatus(t, do(t, http.MethodPost, f.offerURL(offerID, "accept"), employeeSession, nil), http.StatusOK)

	completeEngagement(t, f)
	got := inbox(t, f, employeeSession)
	if got.EngagementID != f.engagementID {
		t.Fatalf("engagementId = %q, want %q: an employee still reaches every Engagement", got.EngagementID, f.engagementID)
	}
	if got.EndedOn == "" {
		t.Fatal("endedOn is absent, want the day her Attachment ended")
	}
}
