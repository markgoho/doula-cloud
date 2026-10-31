package offer_test

import (
	"net/http"
	"testing"

	"doula-cloud/api/internal/offer"
	"doula-cloud/api/internal/testdb"
)

// redactClientFields leaves an Offer as client.Erase does (#2385): the
// three fields copied from the Client are NULL.
func redactClientFields(t *testing.T, db *testdb.DB, offerID string) {
	t.Helper()
	if _, err := db.Admin.ExecContext(t.Context(),
		`UPDATE engagement_offers SET client_first_initial = NULL, client_area = NULL, due_date = NULL WHERE id = $1`,
		offerID,
	); err != nil {
		t.Fatalf("redact offer client fields: %v", err)
	}
}

// TestInboxHandler_ReadsAnErasedClientsOffers is #2385: once her Client is
// erased, an open Offer reads without the three fields and an accepted one
// reads with no due month, on both sides.
func TestInboxHandler_ReadsAnErasedClientsOffers(t *testing.T) {
	f := newFixture(t)
	accepted := f.makeOffer(t, offerBody(f.doulaID, 45000))
	expectStatus(t, do(t, http.MethodPost, f.offerURL(accepted, "accept"), f.doulaSession, nil), http.StatusOK)
	_, otherEngagementID := testdb.SeedEngagement(t, f.db, f.practiceID)
	var created offer.CreateResponse
	decode(t, do(t, http.MethodPost,
		f.srv+"/api/practices/"+f.practiceID+"/engagements/"+otherEngagementID+"/offers",
		f.ownerSession, offerBody(f.doulaID, 45000)), http.StatusCreated, &created)
	redactClientFields(t, f.db, accepted)
	redactClientFields(t, f.db, created.OfferID)

	var inbox offer.ListResponse
	decode(t, do(t, http.MethodGet, f.srv+"/api/practices/"+f.practiceID+"/offers", f.doulaSession, nil),
		http.StatusOK, &inbox)
	if len(inbox.Items) != 2 {
		t.Fatalf("inbox = %d offers, want 2", len(inbox.Items))
	}
	for _, got := range inbox.Items {
		if got.ClientFirstInitial != "" || got.ClientArea != "" || got.DueDate != "" || got.DueMonth != "" {
			t.Fatalf("erased client's offer still serves her fields: %+v", got)
		}
	}

	var practiceSide offer.ListResponse
	decode(t, do(t, http.MethodGet, f.offersURL(), f.ownerSession, nil), http.StatusOK, &practiceSide)
	if got := practiceSide.Items[0]; got.ClientFirstInitial != "" || got.ClientArea != "" || got.DueDate != "" {
		t.Fatalf("practice-side offer still serves her fields: %+v", got)
	}
}

// TestReadHandler_ReadsAnOpenOfferWhoseClientIsErased is #2385 on the
// pre-account read: the Offer still opens, still decidable, without them.
func TestReadHandler_ReadsAnOpenOfferWhoseClientIsErased(t *testing.T) {
	f := newFixture(t)
	offerID, token, code := seedEmailOffer(t, f)
	redactClientFields(t, f.db, offerID)

	var got offer.PreAccountOffer
	decode(t, do(t, http.MethodGet, f.readURL(offerID, token, code), "", nil), http.StatusOK, &got)
	if got.State != stateOffered {
		t.Fatalf("state = %q, want offered", got.State)
	}
	if got.ClientFirstInitial != "" || got.ClientArea != "" || got.DueDate != "" {
		t.Fatalf("erased client's offer still serves her fields: %+v", got)
	}
	if got.AmountCents == nil || *got.AmountCents != 52000 {
		t.Fatalf("amountCents = %v, want her fee", got.AmountCents)
	}
}
