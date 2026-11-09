package contracts_test

import (
	"net/http"
	"testing"

	"doula-cloud/api/internal/contracts"
	"doula-cloud/api/internal/reqtest"
)

const (
	statusDraft  = "draft"
	statusSent   = "sent"
	statusSigned = "signed"
	statusVoided = "voided"
)

// TestClientGetContractHandler_Success proves a Client-portal caller can
// read the filled Contract text for their own sent Contract.
func TestClientGetContractHandler_Success(t *testing.T) {
	e := newEnv(t, withPortalLogin())
	e.seedContract(statusSent)

	out := reqtest.Expect[contracts.ContractResponse](t, http.MethodGet, e.portalURL(""), http.StatusOK, e.AsClient)

	if out.EngagementID != e.Engagement.ID || out.Status != statusSent || out.Prose != mergeFieldProse {
		t.Fatalf("unexpected response: %+v", out)
	}
}

// TestClientGetContractHandler_SignedAndVoidedAllowed proves the view
// stays reachable for both other post-Send statuses, not just 'sent'.
func TestClientGetContractHandler_SignedAndVoidedAllowed(t *testing.T) {
	for _, status := range []string{statusSigned, statusVoided} {
		t.Run(status, func(t *testing.T) {
			e := newEnv(t, withPortalLogin())
			e.seedContract(status)

			reqtest.ExpectStatus(t, http.MethodGet, e.portalURL(""), http.StatusOK, e.AsClient)
		})
	}
}

// TestClientGetContractHandler_NotReadable404s proves a Draft Contract on
// the caller's own Engagement is unreachable through this handler --
// fetchContract sees zero rows (RLS hides the row), so the handler 404s,
// the same "zero rows, not an error" shape the AC asks for -- and that an
// Engagement with no Contract row at all also 404s, not a crash.
func TestClientGetContractHandler_NotReadable404s(t *testing.T) {
	cases := []struct {
		name string
		seed func(*env)
	}{
		{"a draft", (*env).seedDraft},
		{"no contract yet", func(*env) {}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			e := newEnv(t, withPortalLogin())
			c.seed(e)

			reqtest.ExpectStatus(t, http.MethodGet, e.portalURL(""), http.StatusNotFound, e.AsClient)
		})
	}
}

// TestClientGetContractHandler_OtherClientsEngagementRejected proves
// clientauth.Middleware's Engagement-ownership check, not this handler's
// own logic, is what rejects a foreign Engagement id.
func TestClientGetContractHandler_OtherClientsEngagementRejected(t *testing.T) {
	e := newEnv(t, withPortalLogin())
	other := e.AnotherEngagement()
	seedContract(t, e.DB, other.ID, statusSent, mergeFieldProse)

	reqtest.ExpectStatus(t, http.MethodGet, e.portalURLOf(other.ID, ""), http.StatusForbidden, e.AsClient)
}
