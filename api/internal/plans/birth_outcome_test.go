package plans_test

import (
	"net/http"
	"testing"

	"doula-cloud/api/internal/fixture"
	"doula-cloud/api/internal/reqtest"
)

// TestClientBirthPlan_SuppressionIsDerivedNotStored is #294's whole
// claim in one walk. Nadia Haddad's pregnancy ends in a loss, and the
// Birth Plan stops being offered to her at the API -- read, PDF and
// acknowledgment alike -- with nothing on the Plan Instance changed to
// make that happen. The outcome is then corrected, as ADR-0015's own
// motivating typo requires, and the plan comes back with the same
// answers it always had: no retirement to reverse, because nothing was
// ever retired.
func TestClientBirthPlan_SuppressionIsDerivedNotStored(t *testing.T) {
	e := newEnv(t, withClient(fixture.ClientName("Nadia Haddad")), withPortalLogin())
	e.seedBirthPlan()

	assertOffered := func(when string) {
		t.Helper()
		// The Plan Instance itself is untouched throughout: the same
		// answers Staff wrote, still readable and still whole.
		if out := e.birthPlan(); out.Answers["location"] != hospital {
			t.Fatalf("%s: answers = %v, want the Plan Instance untouched", when, out.Answers)
		}
	}

	assertOffered("before any outcome is recorded")

	endedOn := "2027-03-04"
	e.recordOutcome("loss", &endedOn)

	for _, route := range portalRoutes {
		reqtest.ExpectStatus(t, route.method, e.portalURL(route.path), http.StatusNotFound, e.AsClient)
	}

	born := "2027-06-10"
	e.recordOutcome("live_birth", &born)
	assertOffered("after the outcome is corrected to a live birth")
}
