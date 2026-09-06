package offer_test

import (
	"encoding/base64"
	"net/http"
	"net/url"
	"testing"

	"doula-cloud/api/internal/offer"
)

func TestInboxHandler_ServesHerOwnOffersAndNobodyElses(t *testing.T) {
	f := newFixture(t)
	otherID := seedMember(t, f.db, f.practiceID, "uid-doula-2", []string{doulaRole}, contractorType)
	mine := f.makeOffer(t, offerBody(f.doulaID, 45000))
	f.makeOffer(t, offerBody(otherID, 45000))

	var listed offer.ListResponse
	decode(t, do(t, http.MethodGet, f.srv+"/api/practices/"+f.practiceID+"/offers", f.doulaSession, nil),
		http.StatusOK, &listed)

	if len(listed.Items) != 1 {
		t.Fatalf("offers = %d, want only her own", len(listed.Items))
	}
	got := listed.Items[0]
	if got.OfferID != mine {
		t.Fatalf("offerId = %q, want %q", got.OfferID, mine)
	}
	if got.ClientFirstInitial != "R" || got.ClientArea != testClientArea || got.DueDate == "" {
		t.Fatalf("decidable facts missing: %+v", got)
	}
	if got.AmountCents == nil || *got.AmountCents != 45000 {
		t.Fatalf("amountCents = %v, want her fee", got.AmountCents)
	}
	if got.Terms == nil {
		t.Fatal("terms missing")
	}
	// The inbox names nobody: she knows who she is, and the Offer carries
	// no Client name for her to read.
	if got.TargetName != "" || got.TargetAddress != "" {
		t.Fatalf("inbox row names a target: %+v", got)
	}
	if got.DecidedAt != nil {
		t.Fatalf("decidedAt = %v on an undecided offer", got.DecidedAt)
	}
}

func TestInboxHandler_ExpiresOnTheWayPastAndShowsPastOffers(t *testing.T) {
	f := newFixture(t)
	stale := f.makeOffer(t, offerBody(f.doulaID, 45000))
	expireOffer(t, f.db, stale)

	var listed offer.ListResponse
	decode(t, do(t, http.MethodGet, f.srv+"/api/practices/"+f.practiceID+"/offers", f.doulaSession, nil),
		http.StatusOK, &listed)

	if len(listed.Items) != 1 || listed.Items[0].State != stateExpired {
		t.Fatalf("offers = %+v, want one expired row -- past offers stay readable", listed.Items)
	}
}

func TestEngagementListHandler_NamesWhoWasAsked(t *testing.T) {
	f := newFixture(t)
	fee := int64(52000)
	staffOffer := f.makeOffer(t, offerBody(f.doulaID, 45000))
	emailOffer := f.makeOffer(t, emailOfferBody(testAddress, &fee))

	var listed offer.ListResponse
	decode(t, do(t, http.MethodGet, f.offersURL(), f.ownerSession, nil), http.StatusOK, &listed)

	if len(listed.Items) != 2 {
		t.Fatalf("offers = %d, want both", len(listed.Items))
	}
	byID := map[string]offer.Summary{}
	for _, o := range listed.Items {
		byID[o.OfferID] = o
	}
	if got := byID[staffOffer]; got.TargetName == "" || got.TargetAddress == "" {
		t.Fatalf("staff-target row names nobody: %+v", got)
	}
	// An Offer to an email address names nobody until it is accepted, so
	// the address is all there is to show.
	if got := byID[emailOffer]; got.TargetName != "" || got.TargetAddress != testAddress {
		t.Fatalf("email-target row = %+v, want the invited address and no name", got)
	}
}

func TestEngagementListHandler_RefusesBadEngagementID(t *testing.T) {
	f := newFixture(t)
	expectStatus(t, do(t, http.MethodGet,
		f.srv+"/api/practices/"+f.practiceID+"/engagements/not-a-uuid/offers", f.ownerSession, nil),
		http.StatusBadRequest)
}

// Completion closes what is still open on both sides at once: every open
// Offer goes withdrawn with no actor recorded, and every open attachment
// gets its ended_at.
func TestCompleteHandler_RunsTheCascade(t *testing.T) {
	f := newFixture(t)
	accepted := f.makeOffer(t, offerBody(f.doulaID, 45000))
	expectStatus(t, do(t, http.MethodPost, f.offerURL(accepted, "accept"), f.doulaSession, nil), http.StatusOK)

	secondID := seedMember(t, f.db, f.practiceID, "uid-doula-2", []string{doulaRole}, contractorType)
	stillOpen := f.makeOffer(t, offerBody(secondID, 45000))

	expectStatus(t, do(t, http.MethodPost,
		f.srv+"/api/practices/"+f.practiceID+"/engagements/"+f.engagementID+"/complete", f.ownerSession, nil),
		http.StatusOK)

	state, decidedBy := offerState(t, f.db, stillOpen)
	if state != stateWithdrawn {
		t.Fatalf("open offer state = %q, want withdrawn", state)
	}
	if decidedBy != nil {
		t.Fatalf("decided_by = %v, want NULL -- the cascade has no human actor", *decidedBy)
	}
	a, found := readAttachment(t, f, f.doulaID)
	if !found || !a.ended {
		t.Fatalf("attachment = %+v (found=%v), want ended", a, found)
	}
	var endedBy *string
	if err := f.db.Admin.QueryRowContext(t.Context(),
		`SELECT ended_by::text FROM engagement_attachments WHERE engagement_id = $1 AND staff_id = $2`,
		f.engagementID, f.doulaID,
	).Scan(&endedBy); err != nil {
		t.Fatalf("read ended_by: %v", err)
	}
	if endedBy == nil || *endedBy != f.ownerID {
		t.Fatalf("ended_by = %v, want the person who completed the engagement", endedBy)
	}
}

func TestCompleteHandler_RefusesUnknownEngagementAndNonPrivilegedCaller(t *testing.T) {
	f := newFixture(t)
	base := f.srv + "/api/practices/" + f.practiceID + "/engagements/"

	expectStatus(t, do(t, http.MethodPost, base+"11111111-1111-1111-1111-111111111111/complete", f.ownerSession, nil),
		http.StatusNotFound)
	expectStatus(t, do(t, http.MethodPost, base+"not-a-uuid/complete", f.ownerSession, nil), http.StatusBadRequest)
	expectStatus(t, do(t, http.MethodPost, base+f.engagementID+"/complete", f.doulaSession, nil), http.StatusForbidden)
}

// #230: the copied Client fields stop being served the moment an Offer
// goes terminal. She keeps the fact of the asking -- who, when, the fee,
// the terms, and what she answered -- and a stale row in her history is
// not a stranger's due date sitting in it indefinitely.
func TestInboxHandler_LapsesTheClientFieldsOnATerminalOffer(t *testing.T) {
	f := newFixture(t)
	offerID := f.makeOffer(t, offerBody(f.doulaID, 45000))
	expectStatus(t, do(t, http.MethodPost, f.offerURL(offerID, "decline"), f.doulaSession, nil), http.StatusOK)

	var listed offer.ListResponse
	decode(t, do(t, http.MethodGet, f.srv+"/api/practices/"+f.practiceID+"/offers", f.doulaSession, nil),
		http.StatusOK, &listed)

	got := listed.Items[0]
	if got.ClientFirstInitial != "" || got.ClientArea != "" || got.DueDate != "" {
		t.Fatalf("declined offer still serves the client's fields: %+v", got)
	}
	if got.State != stateDeclined || got.DecidedAt == nil {
		t.Fatalf("offer = %+v, want the fact of the asking kept", got)
	}
	if got.AmountCents == nil || *got.AmountCents != 45000 || got.Terms == nil {
		t.Fatalf("offer = %+v, want her fee and terms kept", got)
	}
}

// The list is one page deep with a cursor onto the next, the envelope
// docs/api-design.md section 4 sets.
func TestInboxHandler_PaginatesWithACursor(t *testing.T) {
	f := newFixture(t)
	// 31 Offers: one page of 30, and one row over.
	for range 31 {
		engagementID := seedEngagement(t, f.db, f.practiceID)
		decode(t, do(t, http.MethodPost,
			f.srv+"/api/practices/"+f.practiceID+"/engagements/"+engagementID+"/offers",
			f.ownerSession, offerBody(f.doulaID, 45000)), http.StatusCreated, nil)
	}

	inboxURL := f.srv + "/api/practices/" + f.practiceID + "/offers"
	var first offer.ListResponse
	decode(t, do(t, http.MethodGet, inboxURL, f.doulaSession, nil), http.StatusOK, &first)
	if len(first.Items) != 30 || !first.HasMore || first.NextCursor == nil {
		t.Fatalf("first page = %d items, hasMore=%v, cursor=%v", len(first.Items), first.HasMore, first.NextCursor)
	}

	var second offer.ListResponse
	decode(t, do(t, http.MethodGet, inboxURL+"?cursor="+url.QueryEscape(*first.NextCursor), f.doulaSession, nil),
		http.StatusOK, &second)
	if len(second.Items) != 1 || second.HasMore || second.NextCursor != nil {
		t.Fatalf("second page = %d items, hasMore=%v", len(second.Items), second.HasMore)
	}
	if second.Items[0].OfferID == first.Items[29].OfferID {
		t.Fatal("the second page repeated the first page's last row")
	}
}

// A bad cursor is refused rather than silently serving the wrong page,
// in each of the three ways it can be bad.
func TestListHandlers_RefuseAMalformedCursor(t *testing.T) {
	f := newFixture(t)

	cases := map[string]string{
		"not base64":         "not-base64!",
		"no separator":       base64.URLEncoding.EncodeToString([]byte("nothing-to-split-on")),
		"unparseable moment": base64.URLEncoding.EncodeToString([]byte("not-a-timestamp|" + f.doulaID)),
	}
	for name, raw := range cases {
		t.Run(name, func(t *testing.T) {
			expectStatus(t, do(t, http.MethodGet,
				f.srv+"/api/practices/"+f.practiceID+"/offers?cursor="+url.QueryEscape(raw), f.doulaSession, nil),
				http.StatusBadRequest)
		})
	}
	expectStatus(t, do(t, http.MethodGet, f.offersURL()+"?cursor=not-base64!", f.ownerSession, nil),
		http.StatusBadRequest)
}
