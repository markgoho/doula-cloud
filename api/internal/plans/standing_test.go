package plans_test

import (
	"net/http"
	"testing"

	"doula-cloud/api/internal/apierrtest"
	"doula-cloud/api/internal/clientstanding"
	"doula-cloud/api/internal/fixture"
	"doula-cloud/api/internal/testdb"
)

// TestPostInstanceHandler_RefusesAPlanForAnErasedClient: a new Plan
// Instance is new free text about the Client, which erasure could never
// scrub (ADR-0027), so it is refused before anything is written.
func TestPostInstanceHandler_RefusesAPlanForAnErasedClient(t *testing.T) {
	e := newEnv(t)
	fixture.EraseClient(t, e.DB, e.Engagement.Client.ID)

	got := apierrtest.Expect(t, http.MethodPost, e.instanceURL(carePlanType, ""), http.StatusConflict, e.SignedIn)

	if _, want := clientstanding.ActStartPlan.Check(clientstanding.Erased); got.Message != want {
		t.Errorf("message = %q, want %q", got.Message, want)
	}
	if n := testdb.Scalar[int](t, e.DB.Admin, `SELECT count(*) FROM plan_instances WHERE engagement_id = $1`, e.Engagement.ID); n != 0 {
		t.Errorf("plan_instances = %d, want 0", n)
	}
}
