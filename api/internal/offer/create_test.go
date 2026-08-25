package offer_test

import (
	"net/http"
	"testing"
	"time"

	"doula-cloud/api/internal/offer"
	"doula-cloud/api/internal/tasknudge"
	"doula-cloud/api/internal/testdb"
)

// fixture is the cast every Offer test needs: an Owner who sends, a
// contractor Doula who receives, and an Engagement the work is on.
type fixture struct {
	db           *testdb.DB
	srv          string
	ownerSession string
	practiceID   string
	engagementID string
	ownerID      string
	doulaID      string
	doulaSession string
	enq          *tasknudge.FakeEnqueuer
}

func newFixture(t *testing.T) fixture {
	t.Helper()
	db := testdb.New(t)
	practiceID := seedPractice(t, db)
	ownerID := seedMember(t, db, practiceID, "uid-owner", []string{ownerRole}, employeeType)
	doulaID := seedMember(t, db, practiceID, "uid-doula", []string{doulaRole}, contractorType)
	enq := &tasknudge.FakeEnqueuer{}
	srv, ownerSession := newServer(t, db, "uid-owner", enq)
	t.Cleanup(srv.Close)
	return fixture{
		db:           db,
		srv:          srv.URL,
		ownerSession: ownerSession,
		practiceID:   practiceID,
		engagementID: seedEngagement(t, db, practiceID),
		ownerID:      ownerID,
		doulaID:      doulaID,
		doulaSession: seedSessionFor(t, db, "uid-doula"),
		enq:          enq,
	}
}

// offersURL is the create/list URL for the fixture's Engagement.
func (f fixture) offersURL() string {
	return f.srv + "/practices/" + f.practiceID + "/engagements/" + f.engagementID + "/offers"
}

// offerURL is one Offer's decision URL, for action.
func (f fixture) offerURL(offerID, action string) string {
	return f.srv + "/practices/" + f.practiceID + "/offers/" + offerID + "/" + action
}

// makeOffer sends body as the Owner and returns the Offer id it created.
func (f fixture) makeOffer(t *testing.T, body offer.CreateRequest) string {
	t.Helper()
	var created offer.CreateResponse
	decode(t, do(t, http.MethodPost, f.offersURL(), f.ownerSession, body), http.StatusCreated, &created)
	return created.OfferID
}

func TestCreateHandler_ToStaffTarget(t *testing.T) {
	f := newFixture(t)

	var created offer.CreateResponse
	decode(t, do(t, http.MethodPost, f.offersURL(), f.ownerSession, offerBody(f.doulaID, 45000)),
		http.StatusCreated, &created)

	if created.OfferID == "" {
		t.Fatal("offerId is empty")
	}
	expiresAt, err := time.Parse(time.RFC3339, created.ExpiresAt)
	if err != nil {
		t.Fatalf("parse expiresAt: %v", err)
	}
	if !expiresAt.After(time.Now().Add(6 * 24 * time.Hour)) {
		t.Fatalf("expiresAt = %v, want about seven days out", expiresAt)
	}

	// The employment type is snapshotted off her Membership, not off the
	// request -- and no Invitation is minted for a Staff target.
	var employmentType string
	var invitationID, codeDigest *string
	if err := f.db.Admin.QueryRowContext(t.Context(),
		`SELECT employment_type::text, invitation_id::text, access_code_digest FROM engagement_offers WHERE id = $1`,
		created.OfferID,
	).Scan(&employmentType, &invitationID, &codeDigest); err != nil {
		t.Fatalf("read offer: %v", err)
	}
	if employmentType != contractorType {
		t.Fatalf("employment_type = %q, want contractor", employmentType)
	}
	if invitationID != nil || codeDigest != nil {
		t.Fatalf("staff-target offer minted an invitation (%v) or a code (%v)", invitationID, codeDigest)
	}
	if len(f.enq.Calls()) != 0 {
		t.Fatalf("staff-target offer nudged the offer outbox: %v", f.enq.Calls())
	}
}

func TestCreateHandler_ToEmailTargetMintsInvitationAndOutbox(t *testing.T) {
	f := newFixture(t)
	fee := int64(52000)

	offerID := f.makeOffer(t, emailOfferBody("Renata@Example.com", &fee))

	var address, status, rolesCSV, employmentType string
	if err := f.db.Admin.QueryRowContext(t.Context(),
		`SELECT pi.address, pi.status::text, array_to_string(pi.roles, ','), pi.employment_type::text
		   FROM practice_invitations pi
		   JOIN engagement_offers o ON o.invitation_id = pi.id
		  WHERE o.id = $1`, offerID,
	).Scan(&address, &status, &rolesCSV, &employmentType); err != nil {
		t.Fatalf("read invitation: %v", err)
	}
	if address != "renata@example.com" {
		t.Fatalf("address = %q, want the normalized form", address)
	}
	if status != "pending" || rolesCSV != doulaRole || employmentType != contractorType {
		t.Fatalf("invitation = %s/%s/%s, want pending/doula/contractor", status, rolesCSV, employmentType)
	}

	token, code := outboxCredentials(t, f.db, offerID)
	if token == "" || len(code) != 6 {
		t.Fatalf("outbox credentials = %q/%q, want a token and a six-digit code", token, code)
	}
	if calls := f.enq.Calls(); len(calls) != 1 || calls[0] != tasknudge.EngagementOffer {
		t.Fatalf("nudges = %v, want one engagement-offer nudge", calls)
	}
}

// The code is mailed, never returned: an Owner must not be able to hand
// herself the credentials that open a mailbox she does not control.
func TestCreateHandler_EmailTargetResponseCarriesNoCredentials(t *testing.T) {
	f := newFixture(t)
	fee := int64(52000)

	resp := do(t, http.MethodPost, f.offersURL(), f.ownerSession, emailOfferBody("renata@example.com", &fee))
	var created map[string]any
	decode(t, resp, http.StatusCreated, &created)

	for _, key := range []string{"token", "inviteToken", "code", "accessCode"} {
		if _, present := created[key]; present {
			t.Fatalf("create response carries %q", key)
		}
	}
}

func TestCreateHandler_RefusesSecondOpenOfferToSameTarget(t *testing.T) {
	f := newFixture(t)
	f.makeOffer(t, offerBody(f.doulaID, 45000))

	expectStatus(t, do(t, http.MethodPost, f.offersURL(), f.ownerSession, offerBody(f.doulaID, 45000)),
		http.StatusConflict)
}

// An Offer that has quietly run out is not a duplicate: the expiry flips
// on the way past and the fresh Offer lands.
func TestCreateHandler_ReoffersAfterTheFirstExpires(t *testing.T) {
	f := newFixture(t)
	first := f.makeOffer(t, offerBody(f.doulaID, 45000))
	expireOffer(t, f.db, first)

	f.makeOffer(t, offerBody(f.doulaID, 45000))

	if state, _ := offerState(t, f.db, first); state != stateExpired {
		t.Fatalf("first offer state = %q, want expired", state)
	}
}

func TestCreateHandler_Validation(t *testing.T) {
	f := newFixture(t)
	employeeID := seedMember(t, f.db, f.practiceID, "uid-employee-doula", []string{doulaRole}, employeeType)
	adminOnlyID := seedMember(t, f.db, f.practiceID, "uid-admin-only", []string{"admin"}, employeeType)
	fee := int64(45000)

	cases := []struct {
		name string
		body offer.CreateRequest
		want int
	}{
		{"no target", offer.CreateRequest{ClientFirstInitial: "R", ClientArea: "N", DueDate: testDueDate}, http.StatusBadRequest},
		{"both targets", offer.CreateRequest{StaffID: f.doulaID, Email: "a@b.test", ClientFirstInitial: "R", ClientArea: "N", DueDate: testDueDate}, http.StatusBadRequest},
		{"no initial", offer.CreateRequest{StaffID: f.doulaID, ClientArea: "N", DueDate: testDueDate}, http.StatusBadRequest},
		{"no area", offer.CreateRequest{StaffID: f.doulaID, ClientFirstInitial: "R", DueDate: testDueDate}, http.StatusBadRequest},
		{"no due date", offer.CreateRequest{StaffID: f.doulaID, ClientFirstInitial: "R", ClientArea: "N"}, http.StatusBadRequest},
		{"contractor with no fee", offer.CreateRequest{StaffID: f.doulaID, ClientFirstInitial: "R", ClientArea: "N", DueDate: testDueDate}, http.StatusBadRequest},
		{"employee with a fee", offer.CreateRequest{StaffID: employeeID, AmountCents: &fee, ClientFirstInitial: "R", ClientArea: "N", DueDate: testDueDate}, http.StatusBadRequest},
		{"target is not a doula", offerBody(adminOnlyID, 45000), http.StatusBadRequest},
		{"target is not at this practice", offerBody("11111111-1111-1111-1111-111111111111", 45000), http.StatusBadRequest},
		{"target is not a uuid", offerBody("not-a-uuid", 45000), http.StatusBadRequest},
		{"email target with no fee", emailOfferBody("new@example.test", nil), http.StatusBadRequest},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			expectStatus(t, do(t, http.MethodPost, f.offersURL(), f.ownerSession, tc.body), tc.want)
		})
	}
}

// An employee target takes no fee and is accepted without one -- the
// Offer settles her claim on the work, not her price for it.
func TestCreateHandler_EmployeeTargetNeedsNoFee(t *testing.T) {
	f := newFixture(t)
	employeeID := seedMember(t, f.db, f.practiceID, "uid-employee-doula", []string{doulaRole}, employeeType)

	body := offerBody(employeeID, 0)
	body.AmountCents = nil
	f.makeOffer(t, body)
}

func TestCreateHandler_RefusesEmailTargetWhoAlreadyHoldsMembership(t *testing.T) {
	f := newFixture(t)
	fee := int64(45000)

	expectStatus(t, do(t, http.MethodPost, f.offersURL(), f.ownerSession,
		emailOfferBody("uid-doula@example.com", &fee)), http.StatusConflict)
}

func TestCreateHandler_RefusesCompletedOrMissingEngagement(t *testing.T) {
	f := newFixture(t)
	expectStatus(t, do(t, http.MethodPost,
		f.srv+"/practices/"+f.practiceID+"/engagements/11111111-1111-1111-1111-111111111111/offers",
		f.ownerSession, offerBody(f.doulaID, 45000)), http.StatusNotFound)
	expectStatus(t, do(t, http.MethodPost,
		f.srv+"/practices/"+f.practiceID+"/engagements/not-a-uuid/offers",
		f.ownerSession, offerBody(f.doulaID, 45000)), http.StatusBadRequest)

	expectStatus(t, do(t, http.MethodPost,
		f.srv+"/practices/"+f.practiceID+"/engagements/"+f.engagementID+"/complete",
		f.ownerSession, nil), http.StatusOK)
	expectStatus(t, do(t, http.MethodPost, f.offersURL(), f.ownerSession, offerBody(f.doulaID, 45000)),
		http.StatusConflict)
}

func TestCreateHandler_RefusesInvalidBodyAndNonPrivilegedCaller(t *testing.T) {
	f := newFixture(t)

	req := do(t, http.MethodPost, f.offersURL(), f.ownerSession, "not an object")
	expectStatus(t, req, http.StatusBadRequest)

	expectStatus(t, do(t, http.MethodPost, f.offersURL(), f.doulaSession, offerBody(f.doulaID, 45000)),
		http.StatusForbidden)
}
