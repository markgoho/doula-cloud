package offer_test

import (
	"net/http"
	"testing"

	"doula-cloud/api/internal/offer"
	"doula-cloud/api/internal/testdb"
)

const adminRole = "admin"

func (f fixture) declinedURL() string {
	return f.srv + "/api/practices/" + f.practiceID + "/offers/declined"
}

func (f fixture) declined(t *testing.T, session string) offer.DeclinedResponse {
	t.Helper()
	var got offer.DeclinedResponse
	decode(t, do(t, http.MethodGet, f.declinedURL(), session, nil), http.StatusOK, &got)
	return got
}

func (f fixture) declineOne(t *testing.T) string {
	t.Helper()
	offerID := f.makeOffer(t, offerBody(f.doulaID, 45000))
	expectStatus(t, do(t, http.MethodPost, f.offerURL(offerID, "decline"), f.doulaSession, nil), http.StatusOK)
	return offerID
}

func TestDeclinedHandler_ListsADeclineOnWorkNobodyHas(t *testing.T) {
	f := newFixture(t)
	offerID := f.declineOne(t)
	testdb.SeedStaffAtPractice(t, f.db, f.practiceID, "uid-admin", []string{adminRole}, employeeType)

	for name, session := range map[string]string{ownerRole: f.ownerSession, adminRole: seedSessionFor(t, f.db, "uid-admin")} {
		got := f.declined(t, session)
		if len(got.Items) != 1 {
			t.Fatalf("%s: items = %+v, want the one decline", name, got.Items)
		}
		d := got.Items[0]
		if d.OfferID != offerID || d.EngagementID != f.engagementID || d.DeclinedBy != "Test Staff uid-doula" ||
			d.ClientName == "" || d.Kind == "" || d.DeclinedAt.IsZero() {
			t.Fatalf("%s: item = %+v", name, d)
		}
	}
}

func TestDeclinedHandler_ClearsOnceSomeoneHasTheWork(t *testing.T) {
	cases := map[string]func(t *testing.T, f fixture){
		"a new Offer is open": func(t *testing.T, f fixture) {
			otherID := testdb.SeedStaffAtPractice(t, f.db, f.practiceID, "uid-doula-2", []string{doulaRole}, contractorType)
			f.makeOffer(t, offerBody(otherID, 45000))
		},
		"a Doula is on it": func(t *testing.T, f fixture) {
			employeeID := testdb.SeedStaffAtPractice(t, f.db, f.practiceID, "uid-employee", []string{doulaRole}, employeeType)
			if _, err := f.db.Admin.ExecContext(t.Context(),
				`INSERT INTO engagement_attachments (engagement_id, staff_id, origin, attached_by) VALUES ($1, $2, 'granted', $3)`,
				f.engagementID, employeeID, f.ownerID,
			); err != nil {
				t.Fatalf("attach: %v", err)
			}
		},
		"the Engagement is completed": completeEngagement,
	}
	for name, change := range cases {
		t.Run(name, func(t *testing.T) {
			f := newFixture(t)
			f.declineOne(t)
			change(t, f)
			if got := f.declined(t, f.ownerSession); len(got.Items) != 0 {
				t.Fatalf("items = %+v, want none", got.Items)
			}
		})
	}
}

func TestDeclinedHandler_ComesBackOnceTheNewOfferExpires(t *testing.T) {
	f := newFixture(t)
	f.declineOne(t)
	otherID := testdb.SeedStaffAtPractice(t, f.db, f.practiceID, "uid-doula-2", []string{doulaRole}, contractorType)
	expireOffer(t, f.db, f.makeOffer(t, offerBody(otherID, 45000)))

	if got := f.declined(t, f.ownerSession); len(got.Items) != 1 {
		t.Fatalf("items = %+v, want the decline back", got.Items)
	}
}

func TestDeclinedHandler_RefusesADoulaWithoutOwnerOrAdmin(t *testing.T) {
	f := newFixture(t)
	f.declineOne(t)
	expectStatus(t, do(t, http.MethodGet, f.declinedURL(), f.doulaSession, nil), http.StatusForbidden)
}

func TestDeclinedHandler_PagesByCursor(t *testing.T) {
	f := newFixture(t)
	for range 31 {
		_, engagementID := testdb.SeedEngagement(t, f.db, f.practiceID)
		f.engagementID = engagementID
		f.declineOne(t)
	}

	first := f.declined(t, f.ownerSession)
	if len(first.Items) != 30 || !first.HasMore || first.NextCursor == nil {
		t.Fatalf("first page = %d items, hasMore %v", len(first.Items), first.HasMore)
	}
	var second offer.DeclinedResponse
	decode(t, do(t, http.MethodGet, f.declinedURL()+"?cursor="+*first.NextCursor, f.ownerSession, nil), http.StatusOK, &second)
	if len(second.Items) != 1 || second.HasMore {
		t.Fatalf("second page = %+v", second)
	}
	expectStatus(t, do(t, http.MethodGet, f.declinedURL()+"?cursor=not-a-cursor", f.ownerSession, nil), http.StatusBadRequest)
}
