package contracts_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"doula-cloud/api/internal/authntest"
	"doula-cloud/api/internal/contracts"
	"doula-cloud/api/internal/idempotency"
	"doula-cloud/api/internal/objectstore"
	"doula-cloud/api/internal/push"
	"doula-cloud/api/internal/reqtest"
	"doula-cloud/api/internal/staffauth"
	"doula-cloud/api/internal/testdb"
)

const (
	statusDraft  = "draft"
	statusSent   = "sent"
	statusSigned = "signed"
	statusVoided = "voided"
)

// newPortalServer mounts the same routes main.go wires up for the
// Client-portal Contract view, behind clientauth.Middleware, backed by a
// fresh objectstore.MemoryStore.
func newPortalServer(t *testing.T, db *testdb.DB, uid string) (srv *httptest.Server, session string) {
	t.Helper()
	return newPortalServerWithStore(t, db, uid, objectstore.NewMemoryStore())
}

// newPortalServerWithStore mirrors newPortalServer but lets the caller
// inject store, so a test can inspect what Sign wrote or force a Put/Get
// failure.
func newPortalServerWithStore(t *testing.T, db *testdb.DB, uid string, store objectstore.ObjectStore) (srv *httptest.Server, session string) {
	t.Helper()
	mux := http.NewServeMux()
	g := staffauth.NewGatedRouter(mux, db.App)
	ir := idempotency.NewRouter(g, db.App)
	contracts.Mount(g, ir, db.App, store, push.NewFakePusher())
	return httptest.NewServer(mux), authntest.SeedSession(t, db.App, testdb.PortalUID(uid))
}

func getClientContract(t *testing.T, srv *httptest.Server, session string, engagementID string) *http.Response {
	t.Helper()
	return reqtest.Get(t, srv.URL+"/api/portal/engagements/"+engagementID+"/contract", reqtest.Session(session))
}

func getClientContractPDFRaw(t *testing.T, srv *httptest.Server, session string, engagementID string) *http.Response {
	t.Helper()
	return reqtest.Get(t, srv.URL+"/api/portal/engagements/"+engagementID+"/contract/pdf", reqtest.Session(session))
}

func getClientContractPDF(t *testing.T, srv *httptest.Server, session string, engagementID string) *http.Response {
	t.Helper()
	return getClientContractPDFRaw(t, srv, session, engagementID)
}

// TestClientGetContractHandler_Success proves a Client-portal caller can
// read the filled Contract text for their own sent Contract.
func TestClientGetContractHandler_Success(t *testing.T) {
	db := testdb.New(t)
	const identityUID = "client-viewing-sent-contract"
	practiceID := testdb.SeedPractice(t, db, "Practice")
	clientID, engagementID := testdb.SeedNamedEngagement(t, db, practiceID, "Jordan Client", "jordan@example.com")
	testdb.SeedPortalUser(t, db, testdb.PortalUID(identityUID), clientID)
	seedContract(t, db, engagementID, "sent", mergeFieldProse)

	srv, session := newPortalServer(t, db, identityUID)
	defer srv.Close()

	resp := getClientContract(t, srv, session, engagementID)
	defer resp.Body.Close()

	reqtest.RequireStatus(t, resp, http.StatusOK)
	out := reqtest.Decode[contracts.ContractResponse](t, resp)
	if out.EngagementID != engagementID || out.Status != statusSent || out.Prose != mergeFieldProse {
		t.Fatalf("unexpected response: %+v", out)
	}
}

// TestClientGetContractHandler_SignedAndVoidedAllowed proves the view
// stays reachable for both other post-Send statuses, not just 'sent'.
func TestClientGetContractHandler_SignedAndVoidedAllowed(t *testing.T) {
	for _, status := range []string{statusSigned, statusVoided} {
		t.Run(status, func(t *testing.T) {
			db := testdb.New(t)
			identityUID := "client-viewing-" + status
			practiceID := testdb.SeedPractice(t, db, "Practice")
			clientID, engagementID := testdb.SeedNamedEngagement(t, db, practiceID, "Jordan Client", "jordan@example.com")
			testdb.SeedPortalUser(t, db, testdb.PortalUID(identityUID), clientID)
			seedContract(t, db, engagementID, status, mergeFieldProse)

			srv, session := newPortalServer(t, db, identityUID)
			defer srv.Close()

			resp := getClientContract(t, srv, session, engagementID)
			defer resp.Body.Close()

			reqtest.RequireStatus(t, resp, http.StatusOK)
		})
	}
}

// TestClientGetContractHandler_DraftNeverReturned404s proves a Draft
// Contract on the caller's own Engagement is unreachable through this
// handler -- fetchContract sees zero rows (RLS hides the row), so the
// handler 404s, the same "zero rows, not an error" shape the AC asks for.
func TestClientGetContractHandler_DraftNeverReturned404s(t *testing.T) {
	db := testdb.New(t)
	const identityUID = "client-draft-hidden"
	practiceID := testdb.SeedPractice(t, db, "Practice")
	clientID, engagementID := testdb.SeedNamedEngagement(t, db, practiceID, "Jordan Client", "jordan@example.com")
	testdb.SeedPortalUser(t, db, testdb.PortalUID(identityUID), clientID)
	seedContract(t, db, engagementID, statusDraft, mergeFieldProse)

	srv, session := newPortalServer(t, db, identityUID)
	defer srv.Close()

	resp := getClientContract(t, srv, session, engagementID)
	defer resp.Body.Close()

	reqtest.RequireStatus(t, resp, http.StatusNotFound)
}

// TestClientGetContractHandler_NoContractYet404 proves an Engagement with
// no Contract row at all also 404s, not a crash.
func TestClientGetContractHandler_NoContractYet404(t *testing.T) {
	db := testdb.New(t)
	const identityUID = "client-no-contract-yet"
	practiceID := testdb.SeedPractice(t, db, "Practice")
	clientID, engagementID := testdb.SeedNamedEngagement(t, db, practiceID, "Jordan Client", "jordan@example.com")
	testdb.SeedPortalUser(t, db, testdb.PortalUID(identityUID), clientID)

	srv, session := newPortalServer(t, db, identityUID)
	defer srv.Close()

	resp := getClientContract(t, srv, session, engagementID)
	defer resp.Body.Close()

	reqtest.RequireStatus(t, resp, http.StatusNotFound)
}

// TestClientGetContractHandler_OtherClientsEngagementRejected proves
// clientauth.Middleware's Engagement-ownership check, not this handler's
// own logic, is what rejects a foreign Engagement id.
func TestClientGetContractHandler_OtherClientsEngagementRejected(t *testing.T) {
	db := testdb.New(t)
	const identityUID = "client-not-linked-contract"
	practiceID := testdb.SeedPractice(t, db, "Practice")
	_, otherEngagementID := testdb.SeedNamedEngagement(t, db, practiceID, "Other Client", "other@example.com")
	seedContract(t, db, otherEngagementID, "sent", mergeFieldProse)
	clientID, _ := testdb.SeedNamedEngagement(t, db, practiceID, "Jordan Client", "jordan@example.com")
	testdb.SeedPortalUser(t, db, testdb.PortalUID(identityUID), clientID)

	srv, session := newPortalServer(t, db, identityUID)
	defer srv.Close()

	resp := getClientContract(t, srv, session, otherEngagementID)
	defer resp.Body.Close()

	reqtest.RequireStatus(t, resp, http.StatusForbidden)
}
