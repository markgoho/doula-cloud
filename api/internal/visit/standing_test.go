package visit_test

import (
	"net/http"
	"testing"

	"doula-cloud/api/internal/apierrtest"
	"doula-cloud/api/internal/clientstanding"
	"doula-cloud/api/internal/fixture"
	"doula-cloud/api/internal/reqtest"
	"doula-cloud/api/internal/testdb"
	"doula-cloud/api/internal/visit"
)

// TestCreateHandler_RefusesAVisitForAnErasedClient: erasure completes no
// Engagement (ADR-0027), so the route itself refuses a new Visit, in the
// gate's words, and writes no row.
func TestCreateHandler_RefusesAVisitForAnErasedClient(t *testing.T) {
	e := newEnv(t, signedInAs(fixture.Owner()))
	fixture.EraseClient(t, e.DB, e.Engagement.Client.ID)

	got := apierrtest.Expect(t, http.MethodPost, e.visitsURL(""), http.StatusConflict, e.SignedIn, reqtest.JSON(visit.CreateRequest{}))

	if _, want := clientstanding.ActAddVisit.Check(clientstanding.Erased); got.Message != want {
		t.Errorf("message = %q, want %q", got.Message, want)
	}
	if n := testdb.Scalar[int](t, e.DB.Admin, `SELECT count(*) FROM visits WHERE engagement_id = $1`, e.Engagement.ID); n != 0 {
		t.Errorf("visits = %d, want 0", n)
	}
}
