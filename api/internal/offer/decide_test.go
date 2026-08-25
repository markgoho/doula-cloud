package offer_test

import (
	"net/http"
	"testing"

	"doula-cloud/api/internal/offer"
)

// attachment is one engagement_attachments row as these tests read it.
type attachment struct {
	origin     string
	attachedBy string
	fee        *int64
	terms      *string
	ended      bool
}

func readAttachment(t *testing.T, f fixture, staffID string) (attachment, bool) {
	t.Helper()
	var a attachment
	err := f.db.Admin.QueryRowContext(t.Context(),
		`SELECT origin::text, attached_by::text, fee_amount_cents, fee_terms, ended_at IS NOT NULL
		   FROM engagement_attachments WHERE engagement_id = $1 AND staff_id = $2`,
		f.engagementID, staffID,
	).Scan(&a.origin, &a.attachedBy, &a.fee, &a.terms, &a.ended)
	if err != nil {
		return attachment{}, false
	}
	return a, true
}

func TestAcceptHandler_MintsGrantedAttachmentWithFeeCopied(t *testing.T) {
	f := newFixture(t)
	offerID := f.makeOffer(t, offerBody(f.doulaID, 45000))

	var decided offer.DecisionResponse
	decode(t, do(t, http.MethodPost, f.offerURL(offerID, "accept"), f.doulaSession, nil), http.StatusOK, &decided)
	if decided.State != "accepted" {
		t.Fatalf("state = %q, want accepted", decided.State)
	}

	a, found := readAttachment(t, f, f.doulaID)
	if !found {
		t.Fatal("acceptance minted no attachment")
	}
	if a.origin != "granted" {
		t.Fatalf("origin = %q, want granted", a.origin)
	}
	if a.attachedBy != f.doulaID {
		t.Fatalf("attached_by = %q, want the accepter herself", a.attachedBy)
	}
	if a.fee == nil || *a.fee != 45000 {
		t.Fatalf("fee = %v, want the offer's 45000 copied onto the attachment", a.fee)
	}
	if a.terms == nil || *a.terms == "" {
		t.Fatal("terms were not copied onto the attachment")
	}
}

// Fan-out is uncapped and the first yes wins: the loser's Offer is
// superseded, named to the person whose acceptance closed it.
func TestAcceptHandler_SupersedesEveryOtherOpenOffer(t *testing.T) {
	f := newFixture(t)
	secondID := seedMember(t, f.db, f.practiceID, "uid-doula-2", []string{doulaRole}, contractorType)
	secondSession := seedSessionFor(t, f.db, "uid-doula-2")

	winner := f.makeOffer(t, offerBody(f.doulaID, 45000))
	loser := f.makeOffer(t, offerBody(secondID, 45000))

	expectStatus(t, do(t, http.MethodPost, f.offerURL(winner, "accept"), f.doulaSession, nil), http.StatusOK)

	state, decidedBy := offerState(t, f.db, loser)
	if state != "superseded" {
		t.Fatalf("loser state = %q, want superseded", state)
	}
	if decidedBy == nil || *decidedBy != f.doulaID {
		t.Fatalf("loser decided_by = %v, want the accepter", decidedBy)
	}

	// And the second acceptance is refused rather than racing through.
	expectStatus(t, do(t, http.MethodPost, f.offerURL(loser, "accept"), secondSession, nil), http.StatusConflict)
	if _, found := readAttachment(t, f, secondID); found {
		t.Fatal("a superseded offer still minted an attachment")
	}
}

func TestAcceptHandler_RefusesAnExpiredOffer(t *testing.T) {
	f := newFixture(t)
	offerID := f.makeOffer(t, offerBody(f.doulaID, 45000))
	expireOffer(t, f.db, offerID)

	expectStatus(t, do(t, http.MethodPost, f.offerURL(offerID, "accept"), f.doulaSession, nil), http.StatusConflict)
	if state, _ := offerState(t, f.db, offerID); state != stateExpired {
		t.Fatalf("state = %q, want expired", state)
	}
}

func TestAcceptHandler_RefusesSomeoneElsesOffer(t *testing.T) {
	f := newFixture(t)
	offerID := f.makeOffer(t, offerBody(f.doulaID, 45000))
	otherSession := seedSessionFor(t, f.db, "uid-owner")

	expectStatus(t, do(t, http.MethodPost, f.offerURL(offerID, "accept"), otherSession, nil), http.StatusNotFound)
	expectStatus(t, do(t, http.MethodPost, f.offerURL("not-a-uuid", "accept"), f.doulaSession, nil), http.StatusBadRequest)
}

// A decline is durable and repeatable, carries no reason, and does not
// bar the same Engagement being offered to her again.
func TestDeclineHandler_IsRepeatableAndReoffered(t *testing.T) {
	f := newFixture(t)
	offerID := f.makeOffer(t, offerBody(f.doulaID, 45000))

	var decided offer.DecisionResponse
	decode(t, do(t, http.MethodPost, f.offerURL(offerID, "decline"), f.doulaSession, nil), http.StatusOK, &decided)
	if decided.State != stateDeclined {
		t.Fatalf("state = %q, want declined", decided.State)
	}
	// Again -- same answer, not a 409.
	expectStatus(t, do(t, http.MethodPost, f.offerURL(offerID, "decline"), f.doulaSession, nil), http.StatusOK)

	state, decidedBy := offerState(t, f.db, offerID)
	if state != stateDeclined || decidedBy == nil || *decidedBy != f.doulaID {
		t.Fatalf("offer = %s/%v, want declined by the doula herself", state, decidedBy)
	}
	if _, found := readAttachment(t, f, f.doulaID); found {
		t.Fatal("a decline minted an attachment")
	}

	// And the Practice may ask her again.
	f.makeOffer(t, offerBody(f.doulaID, 50000))
}

func TestDeclineHandler_RefusesSomeoneElsesOffer(t *testing.T) {
	f := newFixture(t)
	offerID := f.makeOffer(t, offerBody(f.doulaID, 45000))

	expectStatus(t, do(t, http.MethodPost, f.offerURL(offerID, "decline"), f.ownerSession, nil), http.StatusNotFound)
}

// An employee's Offer carries no fee and may carry no terms, so the
// attachment it mints carries neither -- fee columns are NULL for any
// granted attachment opened without one.
func TestAcceptHandler_EmployeeOfferCopiesNoFee(t *testing.T) {
	f := newFixture(t)
	employeeID := seedMember(t, f.db, f.practiceID, "uid-employee-doula", []string{doulaRole}, employeeType)
	employeeSession := seedSessionFor(t, f.db, "uid-employee-doula")

	body := offerBody(employeeID, 0)
	body.AmountCents = nil
	body.Terms = ""
	offerID := f.makeOffer(t, body)

	expectStatus(t, do(t, http.MethodPost, f.offerURL(offerID, "accept"), employeeSession, nil), http.StatusOK)

	a, found := readAttachment(t, f, employeeID)
	if !found {
		t.Fatal("acceptance minted no attachment")
	}
	if a.fee != nil || a.terms != nil {
		t.Fatalf("attachment = %+v, want no fee and no terms", a)
	}

	// And the decided Offer reads back with the moment it was decided.
	var listed offer.ListResponse
	decode(t, do(t, http.MethodGet, f.srv+"/practices/"+f.practiceID+"/offers", employeeSession, nil),
		http.StatusOK, &listed)
	if len(listed.Offers) != 1 || listed.Offers[0].DecidedAt == nil {
		t.Fatalf("offers = %+v, want one row carrying decidedAt", listed.Offers)
	}
	if listed.Offers[0].AmountCents != nil || listed.Offers[0].Terms != nil {
		t.Fatalf("employee offer = %+v, want no fee and no terms", listed.Offers[0])
	}
}

func TestDeclineHandler_RefusesAnAlreadyAcceptedOffer(t *testing.T) {
	f := newFixture(t)
	offerID := f.makeOffer(t, offerBody(f.doulaID, 45000))
	expectStatus(t, do(t, http.MethodPost, f.offerURL(offerID, "accept"), f.doulaSession, nil), http.StatusOK)

	expectStatus(t, do(t, http.MethodPost, f.offerURL(offerID, "decline"), f.doulaSession, nil), http.StatusConflict)
}

func TestWithdrawHandler_ClosesAnOpenOffer(t *testing.T) {
	f := newFixture(t)
	offerID := f.makeOffer(t, offerBody(f.doulaID, 45000))

	var decided offer.DecisionResponse
	decode(t, do(t, http.MethodPost, f.offerURL(offerID, "withdraw"), f.ownerSession, nil), http.StatusOK, &decided)
	if decided.State != stateWithdrawn {
		t.Fatalf("state = %q, want withdrawn", decided.State)
	}
	state, decidedBy := offerState(t, f.db, offerID)
	if state != stateWithdrawn || decidedBy == nil || *decidedBy != f.ownerID {
		t.Fatalf("offer = %s/%v, want withdrawn by the owner -- a real withdrawal names its actor", state, decidedBy)
	}

	// Withdrawing twice is a 404: there is no open Offer left to take
	// back, unlike a repeated decline, which is the same person saying
	// the same thing.
	expectStatus(t, do(t, http.MethodPost, f.offerURL(offerID, "withdraw"), f.ownerSession, nil), http.StatusNotFound)
}

func TestWithdrawHandler_RefusesNonPrivilegedCallerAndBadID(t *testing.T) {
	f := newFixture(t)
	offerID := f.makeOffer(t, offerBody(f.doulaID, 45000))

	expectStatus(t, do(t, http.MethodPost, f.offerURL(offerID, "withdraw"), f.doulaSession, nil), http.StatusForbidden)
	expectStatus(t, do(t, http.MethodPost, f.offerURL("not-a-uuid", "withdraw"), f.ownerSession, nil), http.StatusBadRequest)
}

// An Offer that ran out is withdrawn by nobody: the expiry flips first,
// so there is no open Offer left for the withdraw to close.
func TestWithdrawHandler_ExpiresOnTheWayPast(t *testing.T) {
	f := newFixture(t)
	offerID := f.makeOffer(t, offerBody(f.doulaID, 45000))
	expireOffer(t, f.db, offerID)

	expectStatus(t, do(t, http.MethodPost, f.offerURL(offerID, "withdraw"), f.ownerSession, nil), http.StatusNotFound)
	if state, _ := offerState(t, f.db, offerID); state != stateExpired {
		t.Fatalf("state = %q, want expired", state)
	}
}
