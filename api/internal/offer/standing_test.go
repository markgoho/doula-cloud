package offer_test

import (
	"net/http"
	"testing"

	"doula-cloud/api/internal/clientstanding"
	scene "doula-cloud/api/internal/fixture"
	"doula-cloud/api/internal/testdb"
)

// TestCreateHandler_RefusesAnOfferForAnErasedClient: an Offer copies the
// Client's initial, area and due date to a Doula, which erasure redacts
// from every Offer it finds (ADR-0027's 2026-10-31 amendment). A new one
// after it would carry them again, so it is refused and nothing is sent.
func TestCreateHandler_RefusesAnOfferForAnErasedClient(t *testing.T) {
	f := newFixture(t)
	clientID := testdb.Scalar[string](t, f.db.Admin, `SELECT client_id FROM engagements WHERE id = $1`, f.engagementID)
	scene.EraseClient(t, f.db, clientID)

	resp := do(t, http.MethodPost, f.offersURL(), f.ownerSession, offerBody(f.doulaID, 45000))

	expectStatus(t, resp, http.StatusConflict)
	if _, want := clientstanding.ActOfferWork.Check(clientstanding.Erased); decodeRefusal(t, resp).Message != want {
		t.Errorf("message = %q, want %q", decodeRefusal(t, resp).Message, want)
	}
	if n := testdb.Scalar[int](t, f.db.Admin, `SELECT count(*) FROM engagement_offers WHERE engagement_id = $1`, f.engagementID); n != 0 {
		t.Errorf("engagement_offers = %d, want 0", n)
	}
}
