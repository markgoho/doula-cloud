package offer_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/url"
	"testing"

	"doula-cloud/api/internal/offer"
	"doula-cloud/api/internal/testdb"
)

// seedEmailOffer makes an Offer to a bare email address and returns its
// id together with the two credentials the worker would mail.
func seedEmailOffer(t *testing.T, f fixture) (offerID, token, code string) {
	t.Helper()
	fee := int64(52000)
	offerID = f.makeOffer(t, emailOfferBody(testAddress, &fee))
	token, code = outboxCredentials(t, f.db, offerID)
	return offerID, token, code
}

// readURL is the pre-account read URL for one Offer, token and code in
// the query string the way the emailed link and the typed code arrive.
func (f fixture) readURL(offerID, token, code string) string {
	return f.srv + "/api/offers/" + offerID + "?token=" + url.QueryEscape(token) + "&code=" + url.QueryEscape(code)
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
		{"no token", f.srv + "/api/offers/" + offerID + "?code=" + code, http.StatusBadRequest},
		{"no code", f.srv + "/api/offers/" + offerID + "?token=" + token, http.StatusBadRequest},
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
//
// #836 mounted this read behind offer.Mount's real ratelimit.Wrap
// (offerRules' PathValueRule also caps 10 requests per offerId per hour,
// by design -- see offerRules' own doc comment). Without
// resetOfferReadBucket, the 11th request below would 429 at that outer
// layer instead of resolveByToken's own maxAccessCodeAttempts check,
// which is what this test is actually about.
func TestReadHandler_BoundsCodeGuessing(t *testing.T) {
	f := newFixture(t)
	offerID, token, code := seedEmailOffer(t, f)

	for range 10 {
		expectStatus(t, do(t, http.MethodGet, f.readURL(offerID, token, "000000"), "", nil), http.StatusForbidden)
	}
	resetOfferReadBucket(t, f.db)
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
	declineURL := f.srv + "/api/offers/" + offerID + "/decline"
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

// #473: an unconfirmed decline must not even spend one of the ten
// code-guess attempts 00041 bounds -- checked before withTokenTx.
func TestDeclineByTokenHandler_RequiresConfirmation(t *testing.T) {
	f := newFixture(t)
	offerID, token, code := seedEmailOffer(t, f)
	declineURL := f.srv + "/api/offers/" + offerID + "/decline"
	payload, err := json.Marshal(offer.DeclineByTokenRequest{Token: token, Code: code})
	if err != nil {
		t.Fatalf("marshal body: %v", err)
	}

	req, err := http.NewRequestWithContext(t.Context(), http.MethodPost, declineURL, bytes.NewReader(payload))
	if err != nil {
		t.Fatalf("build request: %v", err)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusBadRequest)
	}

	state, _ := offerState(t, f.db, offerID)
	if state != "offered" {
		t.Fatalf("state = %q, want the unconfirmed request to have declined nothing", state)
	}

	// The right code, still unconfirmed, still spent no guess -- the
	// correct code works a moment later, confirmed.
	var decided offer.DecisionResponse
	decode(t, do(t, http.MethodPost, declineURL, "", offer.DeclineByTokenRequest{Token: token, Code: code}), http.StatusOK, &decided)
	if decided.State != stateDeclined {
		t.Fatalf("state = %q, want declined", decided.State)
	}
}

func TestDeclineByTokenHandler_RefusesBadInput(t *testing.T) {
	f := newFixture(t)
	offerID, token, code := seedEmailOffer(t, f)
	declineURL := f.srv + "/api/offers/" + offerID + "/decline"

	expectStatus(t, do(t, http.MethodPost, declineURL, "", "not an object"), http.StatusBadRequest)
	expectStatus(t, do(t, http.MethodPost, declineURL, "", offer.DeclineByTokenRequest{Token: token}), http.StatusBadRequest)
	expectStatus(t, do(t, http.MethodPost, declineURL, "",
		offer.DeclineByTokenRequest{Token: token, Code: "000000"}), http.StatusForbidden)

	// An Offer already taken back cannot be declined.
	expectStatus(t, do(t, http.MethodPost, f.offerURL(offerID, "withdraw"), f.ownerSession, nil), http.StatusOK)
	expectStatus(t, do(t, http.MethodPost, declineURL, "",
		offer.DeclineByTokenRequest{Token: token, Code: code}), http.StatusConflict)
}

// #230's terminal rule on the pre-account read: she opens the link in
// March and gets a closed offer, not the Client's due date.
func TestReadHandler_LapsesTheClientFieldsOnATerminalOffer(t *testing.T) {
	f := newFixture(t)
	offerID, token, code := seedEmailOffer(t, f)
	expectStatus(t, do(t, http.MethodPost, f.offerURL(offerID, "withdraw"), f.ownerSession, nil), http.StatusOK)

	var got offer.PreAccountOffer
	decode(t, do(t, http.MethodGet, f.readURL(offerID, token, code), "", nil), http.StatusOK, &got)

	if got.State != stateWithdrawn {
		t.Fatalf("state = %q, want withdrawn", got.State)
	}
	if got.ClientFirstInitial != "" || got.ClientArea != "" || got.DueDate != "" {
		t.Fatalf("withdrawn offer still serves the client's fields: %+v", got)
	}
	// The fee and the terms are hers, not the Client's, and stay.
	if got.AmountCents == nil || got.Terms == nil {
		t.Fatalf("offer = %+v, want the fee and terms kept", got)
	}
}

// A second Offer to the same address rotates the Invitation's token,
// which would leave the first Offer's emailed link opening nothing. The
// first Offer is re-issued instead: a fresh code, and a fresh email
// carrying the new token.
func TestCreate_ReissuesAnOpenOfferWhenTheTokenRotates(t *testing.T) {
	f := newFixture(t)
	firstOffer, firstToken, firstCode := seedEmailOffer(t, f)

	secondEngagement := seedEngagement(t, f.db, f.practiceID)
	fee := int64(52000)
	var created offer.CreateResponse
	decode(t, do(t, http.MethodPost,
		f.srv+"/api/practices/"+f.practiceID+"/engagements/"+secondEngagement+"/offers",
		f.ownerSession, emailOfferBody(testAddress, &fee)), http.StatusCreated, &created)

	// The credentials the first Offer was mailed with no longer open it.
	expectStatus(t, do(t, http.MethodGet, f.readURL(firstOffer, firstToken, firstCode), "", nil), http.StatusNotFound)

	// The ones queued for it now do -- a fresh code against the new token.
	reissuedToken, reissuedCode := outboxCredentials(t, f.db, firstOffer)
	if reissuedCode == firstCode {
		t.Fatal("the re-issued offer kept its old code")
	}
	var got offer.PreAccountOffer
	decode(t, do(t, http.MethodGet, f.readURL(firstOffer, reissuedToken, reissuedCode), "", nil), http.StatusOK, &got)
	if got.OfferID != firstOffer {
		t.Fatalf("re-issued credentials opened %q, want %q", got.OfferID, firstOffer)
	}

	// And the second Offer's own credentials work too: one token, two
	// Offers, a code each.
	newToken, newCode := outboxCredentials(t, f.db, created.OfferID)
	expectStatus(t, do(t, http.MethodGet, f.readURL(created.OfferID, newToken, newCode), "", nil), http.StatusOK)
}

// A re-issue resets the guess counter with the code it replaces: guesses
// spent against a code nobody can use any more are not held against its
// successor.
//
// #836 mounted this package's pre-account read through offer.Mount, the
// same ratelimit.Wrap(offerRules) production always ran it behind --
// PathValueRule caps 10 requests per offerId per hour, matching
// maxAccessCodeAttempts by design (offerRules' own doc comment). This
// test's 10 deliberate wrong guesses spend that whole budget on purpose,
// so the reissue's own correct read below would 429 on the rate limit
// rather than exercise what this test is actually about -- a real,
// pre-existing gap between the read-rate-limit and the reissue flow,
// filed as #846 rather than fixed here (#836 is the routing seam, not
// ratelimit's sizing policy). resetOfferReadBucket clears the counter the
// same way expireOffer above backdates one, so the test can still prove
// the guess-count reset without waiting on -- or changing -- that limit.
func TestCreate_ReissueResetsTheGuessCounter(t *testing.T) {
	f := newFixture(t)
	firstOffer, firstToken, _ := seedEmailOffer(t, f)
	for range 10 {
		expectStatus(t, do(t, http.MethodGet, f.readURL(firstOffer, firstToken, "000000"), "", nil), http.StatusForbidden)
	}

	secondEngagement := seedEngagement(t, f.db, f.practiceID)
	fee := int64(52000)
	decode(t, do(t, http.MethodPost,
		f.srv+"/api/practices/"+f.practiceID+"/engagements/"+secondEngagement+"/offers",
		f.ownerSession, emailOfferBody(testAddress, &fee)), http.StatusCreated, nil)

	resetOfferReadBucket(t, f.db)
	reissuedToken, reissuedCode := outboxCredentials(t, f.db, firstOffer)
	expectStatus(t, do(t, http.MethodGet, f.readURL(firstOffer, reissuedToken, reissuedCode), "", nil), http.StatusOK)
}

// resetOfferReadBucket clears every offer_read rate-limit bucket, the
// same "reach past the seam under test to move time/state along" pattern
// expireOffer already uses for expires_at.
func resetOfferReadBucket(t *testing.T, db *testdb.DB) {
	t.Helper()
	if _, err := db.Admin.ExecContext(t.Context(), `DELETE FROM rate_limit_buckets WHERE key LIKE 'offer_read:%'`); err != nil {
		t.Fatalf("reset offer_read rate limit bucket: %v", err)
	}
}
