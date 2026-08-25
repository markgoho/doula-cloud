package offer_test

import (
	"net/http"
	"net/url"
	"testing"

	"doula-cloud/api/internal/offer"
)

// seedEmailOffer makes an Offer to a bare email address and returns its
// id together with the two credentials the worker would mail.
func seedEmailOffer(t *testing.T, f fixture) (offerID, token, code string) {
	t.Helper()
	fee := int64(52000)
	offerID = f.makeOffer(t, emailOfferBody(testAddress, contractorType, &fee))
	token, code = outboxCredentials(t, f.db, offerID)
	return offerID, token, code
}

// readURL is the pre-account read URL for one Offer, token and code in
// the query string the way the emailed link and the typed code arrive.
func (f fixture) readURL(offerID, token, code string) string {
	return f.srv + "/offers/" + offerID + "?token=" + url.QueryEscape(token) + "&code=" + url.QueryEscape(code)
}

func TestReadHandler_ServesTheFourFactsAndTerms(t *testing.T) {
	f := newFixture(t)
	offerID, token, code := seedEmailOffer(t, f)

	var got offer.PreAccountOffer
	decode(t, do(t, http.MethodGet, f.readURL(offerID, token, code), "", nil), http.StatusOK, &got)

	if got.OfferID != offerID || got.State != "offered" {
		t.Fatalf("offer = %+v, want the open offer", got)
	}
	if got.ClientFirstInitial != "R" || got.ClientArea != testClientArea || got.DueDate == "" {
		t.Fatalf("decidable facts missing: %+v", got)
	}
	if got.AmountCents == nil || *got.AmountCents != 52000 {
		t.Fatalf("amountCents = %v, want her fee", got.AmountCents)
	}
	if got.Terms == nil {
		t.Fatal("terms missing")
	}
}

// The read is a copy of one row and nothing else: no Client name, no
// Engagement id, no Practice name reaches someone with no account.
func TestReadHandler_LeaksNothingBeyondTheOfferRow(t *testing.T) {
	f := newFixture(t)
	offerID, token, code := seedEmailOffer(t, f)

	var body map[string]any
	decode(t, do(t, http.MethodGet, f.readURL(offerID, token, code), "", nil), http.StatusOK, &body)

	for _, key := range []string{"clientName", "clientId", "engagementId", "practiceId", "practiceName", "offeredBy"} {
		if _, present := body[key]; present {
			t.Fatalf("pre-account read carries %q", key)
		}
	}
}

func TestReadHandler_RefusesTheWrongCredentials(t *testing.T) {
	f := newFixture(t)
	offerID, token, code := seedEmailOffer(t, f)

	cases := []struct {
		name string
		url  string
		want int
	}{
		{"no token", f.srv + "/offers/" + offerID + "?code=" + code, http.StatusBadRequest},
		{"no code", f.srv + "/offers/" + offerID + "?token=" + token, http.StatusBadRequest},
		{"offer id is not a uuid", f.readURL("not-a-uuid", token, code), http.StatusBadRequest},
		{"unknown token", f.readURL(offerID, "11111111-1111-1111-1111-111111111111", code), http.StatusNotFound},
		{"wrong code", f.readURL(offerID, token, "000000"), http.StatusForbidden},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			expectStatus(t, do(t, http.MethodGet, tc.url, "", nil), tc.want)
		})
	}
}

// An Offer to an existing Staff member carries no code at all, so no
// token opens it here: she reads it through her session.
func TestReadHandler_DoesNotReachAStaffTargetOffer(t *testing.T) {
	f := newFixture(t)
	emailOfferID, token, _ := seedEmailOffer(t, f)
	staffOfferID := f.makeOffer(t, offerBody(f.doulaID, 45000))

	expectStatus(t, do(t, http.MethodGet, f.readURL(staffOfferID, token, "123456"), "", nil), http.StatusNotFound)
	// The same token still opens the Offer it was actually mailed with.
	expectStatus(t, do(t, http.MethodGet, f.readURL(emailOfferID, token, "123456"), "", nil), http.StatusForbidden)
}

// Ten wrong guesses burn the Offer: a six-digit code in front of an
// unauthenticated endpoint is otherwise a space anyone may walk.
func TestReadHandler_BoundsCodeGuessing(t *testing.T) {
	f := newFixture(t)
	offerID, token, code := seedEmailOffer(t, f)

	for range 10 {
		expectStatus(t, do(t, http.MethodGet, f.readURL(offerID, token, "000000"), "", nil), http.StatusForbidden)
	}
	// Even the right code no longer opens it.
	expectStatus(t, do(t, http.MethodGet, f.readURL(offerID, token, code), "", nil), http.StatusTooManyRequests)
}

func TestReadHandler_ReportsAnExpiredOffer(t *testing.T) {
	f := newFixture(t)
	offerID, token, code := seedEmailOffer(t, f)
	expireOffer(t, f.db, offerID)

	var got offer.PreAccountOffer
	decode(t, do(t, http.MethodGet, f.readURL(offerID, token, code), "", nil), http.StatusOK, &got)
	if got.State != stateExpired {
		t.Fatalf("state = %q, want expired", got.State)
	}
}

// Declining must not require joining a Practice in order to say no to
// it. decided_by stays NULL: there is no staff row, and inventing one
// would record a person who does not exist.
func TestDeclineByTokenHandler_DeclinesWithoutAnAccount(t *testing.T) {
	f := newFixture(t)
	offerID, token, code := seedEmailOffer(t, f)
	declineURL := f.srv + "/offers/" + offerID + "/decline"
	body := offer.DeclineByTokenRequest{Token: token, Code: code}

	var decided offer.DecisionResponse
	decode(t, do(t, http.MethodPost, declineURL, "", body), http.StatusOK, &decided)
	if decided.State != stateDeclined {
		t.Fatalf("state = %q, want declined", decided.State)
	}
	state, decidedBy := offerState(t, f.db, offerID)
	if state != stateDeclined || decidedBy != nil {
		t.Fatalf("offer = %s/%v, want declined with no actor", state, decidedBy)
	}

	// Durable and repeatable, the same as a Staff member's decline.
	expectStatus(t, do(t, http.MethodPost, declineURL, "", body), http.StatusOK)
}

func TestDeclineByTokenHandler_RefusesBadInput(t *testing.T) {
	f := newFixture(t)
	offerID, token, code := seedEmailOffer(t, f)
	declineURL := f.srv + "/offers/" + offerID + "/decline"

	expectStatus(t, do(t, http.MethodPost, declineURL, "", "not an object"), http.StatusBadRequest)
	expectStatus(t, do(t, http.MethodPost, declineURL, "", offer.DeclineByTokenRequest{Token: token}), http.StatusBadRequest)
	expectStatus(t, do(t, http.MethodPost, declineURL, "",
		offer.DeclineByTokenRequest{Token: token, Code: "000000"}), http.StatusForbidden)

	// An Offer already taken back cannot be declined.
	expectStatus(t, do(t, http.MethodPost, f.offerURL(offerID, "withdraw"), f.ownerSession, nil), http.StatusOK)
	expectStatus(t, do(t, http.MethodPost, declineURL, "",
		offer.DeclineByTokenRequest{Token: token, Code: code}), http.StatusConflict)
}
