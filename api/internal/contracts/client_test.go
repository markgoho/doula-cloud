package contracts_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"doula-cloud/api/internal/authntest"
	"doula-cloud/api/internal/clientauth"
	"doula-cloud/api/internal/contracts"
	"doula-cloud/api/internal/objectstore"
	"doula-cloud/api/internal/testdb"
)

const (
	statusDraft  = "draft"
	statusSent   = "sent"
	statusSigned = "signed"
	statusVoided = "voided"
)

// seedClientEngagement inserts a Client and an Engagement linking them to
// practiceID, using the superuser Admin connection -- mirrors
// plans/client_test.go's seedClientEngagement (package-local, not
// exported across packages).
func seedClientEngagement(t *testing.T, db *testdb.DB, practiceID, name, email string) (clientID, engagementID string) {
	t.Helper()
	if err := db.Admin.QueryRowContext(t.Context(),
		`INSERT INTO clients (practice_id, given_name, email) VALUES ($1, $2, $3) RETURNING id`,
		practiceID, name, email,
	).Scan(&clientID); err != nil {
		t.Fatalf("seed client: %v", err)
	}
	if err := db.Admin.QueryRowContext(t.Context(),
		`INSERT INTO engagements (client_id, practice_id, kind) VALUES ($1, $2, 'birth') RETURNING id`,
		clientID, practiceID,
	).Scan(&engagementID); err != nil {
		t.Fatalf("seed engagement: %v", err)
	}
	return clientID, engagementID
}

// seedPortalUser links identityUID to clientID via client_portal_users,
// using the superuser Admin connection.
func seedPortalUser(t *testing.T, db *testdb.DB, identityUID, clientID string) {
	t.Helper()
	if _, err := db.Admin.ExecContext(t.Context(),
		`INSERT INTO client_portal_users (identity_uid, client_id) VALUES ($1, $2)`,
		identityUID, clientID,
	); err != nil {
		t.Fatalf("seed client_portal_users: %v", err)
	}
}

// seedPushSubscription inserts a push_subscriptions row for
// ownerType/ownerID, using the superuser Admin connection -- mirrors
// message/helpers_test.go's seedPushSubscription.
func seedPushSubscription(t *testing.T, db *testdb.DB, ownerType, ownerID, endpoint string) {
	t.Helper()
	if _, err := db.Admin.ExecContext(t.Context(),
		`INSERT INTO push_subscriptions (owner_type, owner_id, endpoint, p256dh_key, auth_key)
		 VALUES ($1, $2, $3, 'p256dh-key', 'auth-key')`,
		ownerType, ownerID, endpoint,
	); err != nil {
		t.Fatalf("seed push_subscription: %v", err)
	}
}

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
	mux.Handle("GET /portal/engagements/{engagementId}/contract",
		clientauth.Middleware(db.App)(contracts.ClientGetContractHandler()))
	mux.Handle("POST /portal/engagements/{engagementId}/contract/sign",
		clientauth.Middleware(db.App)(contracts.ClientPostSignContractHandler(store)))
	mux.Handle("GET /portal/engagements/{engagementId}/contract/pdf",
		clientauth.Middleware(db.App)(contracts.ClientGetSignedContractPDFHandler(store)))
	return httptest.NewServer(mux), authntest.SeedSession(t, db.App, uid)
}

func getClientContract(t *testing.T, srv *httptest.Server, session string, engagementID string) *http.Response {
	t.Helper()
	req, err := http.NewRequestWithContext(t.Context(), http.MethodGet, srv.URL+"/portal/engagements/"+engagementID+"/contract", nil)
	if err != nil {
		t.Fatalf("build request: %v", err)
	}
	authntest.AddSessionCookie(req, session)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	return resp
}

func getClientContractPDFRaw(t *testing.T, srv *httptest.Server, session string, engagementID string) *http.Response {
	t.Helper()
	req, err := http.NewRequestWithContext(t.Context(), http.MethodGet, srv.URL+"/portal/engagements/"+engagementID+"/contract/pdf", nil)
	if err != nil {
		t.Fatalf("build request: %v", err)
	}
	if session != "" {
		authntest.AddSessionCookie(req, session)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	return resp
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
	practiceID := seedPractice(t, db, "Practice")
	clientID, engagementID := seedClientEngagement(t, db, practiceID, "Jordan Client", "jordan@example.com")
	seedPortalUser(t, db, identityUID, clientID)
	seedContract(t, db, engagementID, "sent", mergeFieldProse)

	srv, session := newPortalServer(t, db, identityUID)
	defer srv.Close()

	resp := getClientContract(t, srv, session, engagementID)
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}
	var out contracts.ContractResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		t.Fatalf("decode response: %v", err)
	}
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
			practiceID := seedPractice(t, db, "Practice")
			clientID, engagementID := seedClientEngagement(t, db, practiceID, "Jordan Client", "jordan@example.com")
			seedPortalUser(t, db, identityUID, clientID)
			seedContract(t, db, engagementID, status, mergeFieldProse)

			srv, session := newPortalServer(t, db, identityUID)
			defer srv.Close()

			resp := getClientContract(t, srv, session, engagementID)
			defer resp.Body.Close()

			if resp.StatusCode != http.StatusOK {
				t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusOK)
			}
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
	practiceID := seedPractice(t, db, "Practice")
	clientID, engagementID := seedClientEngagement(t, db, practiceID, "Jordan Client", "jordan@example.com")
	seedPortalUser(t, db, identityUID, clientID)
	seedContract(t, db, engagementID, statusDraft, mergeFieldProse)

	srv, session := newPortalServer(t, db, identityUID)
	defer srv.Close()

	resp := getClientContract(t, srv, session, engagementID)
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusNotFound)
	}
}

// TestClientGetContractHandler_NoContractYet404 proves an Engagement with
// no Contract row at all also 404s, not a crash.
func TestClientGetContractHandler_NoContractYet404(t *testing.T) {
	db := testdb.New(t)
	const identityUID = "client-no-contract-yet"
	practiceID := seedPractice(t, db, "Practice")
	clientID, engagementID := seedClientEngagement(t, db, practiceID, "Jordan Client", "jordan@example.com")
	seedPortalUser(t, db, identityUID, clientID)

	srv, session := newPortalServer(t, db, identityUID)
	defer srv.Close()

	resp := getClientContract(t, srv, session, engagementID)
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusNotFound)
	}
}

// TestClientGetContractHandler_OtherClientsEngagementRejected proves
// clientauth.Middleware's Engagement-ownership check, not this handler's
// own logic, is what rejects a foreign Engagement id.
func TestClientGetContractHandler_OtherClientsEngagementRejected(t *testing.T) {
	db := testdb.New(t)
	const identityUID = "client-not-linked-contract"
	practiceID := seedPractice(t, db, "Practice")
	_, otherEngagementID := seedClientEngagement(t, db, practiceID, "Other Client", "other@example.com")
	seedContract(t, db, otherEngagementID, "sent", mergeFieldProse)
	clientID, _ := seedClientEngagement(t, db, practiceID, "Jordan Client", "jordan@example.com")
	seedPortalUser(t, db, identityUID, clientID)

	srv, session := newPortalServer(t, db, identityUID)
	defer srv.Close()

	resp := getClientContract(t, srv, session, otherEngagementID)
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusForbidden)
	}
}
