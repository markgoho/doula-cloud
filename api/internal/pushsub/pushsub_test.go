package pushsub_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"doula-cloud/api/internal/authn"
	"doula-cloud/api/internal/clientauth"
	"doula-cloud/api/internal/pushsub"
	"doula-cloud/api/internal/staffauth"
	"doula-cloud/api/internal/testdb"
)

// fakeVerifier is a test double for authn.Verifier -- see staffauth's own
// middleware_test.go for why: real Identity Platform tokens can't be
// minted without a live GCP project.
type fakeVerifier struct {
	uid string
}

func (f fakeVerifier) VerifyIDToken(_ context.Context, _ string) (*authn.VerifiedToken, error) {
	return &authn.VerifiedToken{UID: f.uid}, nil
}

// newStaffServer mounts the same push-subscription routes main.go wires
// up for the Staff population, behind staffauth.Middleware.
func newStaffServer(verifier authn.Verifier, db *testdb.DB) *httptest.Server {
	mux := http.NewServeMux()
	mux.Handle("POST /practices/{practiceId}/push-subscriptions",
		staffauth.Middleware(verifier, db.App)(pushsub.RegisterHandler()))
	mux.Handle("DELETE /practices/{practiceId}/push-subscriptions",
		staffauth.Middleware(verifier, db.App)(pushsub.UnregisterHandler()))
	return httptest.NewServer(mux)
}

// newPortalServer mirrors newStaffServer for the Client-portal population,
// behind clientauth.Middleware.
func newPortalServer(verifier authn.Verifier, db *testdb.DB) *httptest.Server {
	mux := http.NewServeMux()
	mux.Handle("POST /portal/engagements/{engagementId}/push-subscriptions",
		clientauth.Middleware(verifier, db.App)(pushsub.ClientRegisterHandler()))
	mux.Handle("DELETE /portal/engagements/{engagementId}/push-subscriptions",
		clientauth.Middleware(verifier, db.App)(pushsub.ClientUnregisterHandler()))
	return httptest.NewServer(mux)
}

func authedRequest(t *testing.T, method, url string, body []byte) *http.Response {
	t.Helper()
	var reader *bytes.Reader
	if body != nil {
		reader = bytes.NewReader(body)
	} else {
		reader = bytes.NewReader(nil)
	}
	req, err := http.NewRequestWithContext(t.Context(), method, url, reader)
	if err != nil {
		t.Fatalf("build request: %v", err)
	}
	req.Header.Set("Authorization", "Bearer tok")
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	return resp
}

func subscribeBody(t *testing.T, endpoint string) []byte {
	t.Helper()
	req := pushsub.SubscribeRequest{Endpoint: endpoint}
	req.Keys.P256dh = "p256dh-key"
	req.Keys.Auth = "auth-key"
	body, err := json.Marshal(req)
	if err != nil {
		t.Fatalf("marshal subscribe request: %v", err)
	}
	return body
}

func TestRegisterHandler_Success(t *testing.T) {
	db := testdb.New(t)
	const identityUID = "staff-registers"
	practiceID := seedPractice(t, db, "Practice")
	seedStaffAtPractice(t, db, practiceID, identityUID)

	srv := newStaffServer(fakeVerifier{uid: identityUID}, db)
	defer srv.Close()

	const endpoint = "https://push.example.com/staff-device"
	resp := authedRequest(t, http.MethodPost, srv.URL+"/practices/"+practiceID+"/push-subscriptions", subscribeBody(t, endpoint))
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusNoContent {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusNoContent)
	}
	if got := countSubscriptions(t, db, endpoint); got != 1 {
		t.Fatalf("push_subscriptions rows for endpoint = %d, want 1", got)
	}
}

// TestRegisterHandler_ReregisterSameEndpointUpserts proves re-registering
// the same device (same endpoint, e.g. after the browser rotated its
// keys) updates the existing row in place rather than erroring on the
// endpoint UNIQUE constraint.
func TestRegisterHandler_ReregisterSameEndpointUpserts(t *testing.T) {
	db := testdb.New(t)
	const identityUID = "staff-reregisters"
	practiceID := seedPractice(t, db, "Practice")
	seedStaffAtPractice(t, db, practiceID, identityUID)

	srv := newStaffServer(fakeVerifier{uid: identityUID}, db)
	defer srv.Close()

	const endpoint = "https://push.example.com/staff-rotating-keys"
	first := authedRequest(t, http.MethodPost, srv.URL+"/practices/"+practiceID+"/push-subscriptions", subscribeBody(t, endpoint))
	_ = first.Body.Close()
	if first.StatusCode != http.StatusNoContent {
		t.Fatalf("first register status = %d, want %d", first.StatusCode, http.StatusNoContent)
	}

	second := authedRequest(t, http.MethodPost, srv.URL+"/practices/"+practiceID+"/push-subscriptions", subscribeBody(t, endpoint))
	_ = second.Body.Close()
	if second.StatusCode != http.StatusNoContent {
		t.Fatalf("second register status = %d, want %d", second.StatusCode, http.StatusNoContent)
	}
	if got := countSubscriptions(t, db, endpoint); got != 1 {
		t.Fatalf("push_subscriptions rows for endpoint = %d, want 1 (upserted, not duplicated)", got)
	}
}

func TestRegisterHandler_InvalidJSONBody(t *testing.T) {
	db := testdb.New(t)
	const identityUID = "staff-bad-json"
	practiceID := seedPractice(t, db, "Practice")
	seedStaffAtPractice(t, db, practiceID, identityUID)

	srv := newStaffServer(fakeVerifier{uid: identityUID}, db)
	defer srv.Close()

	resp := authedRequest(t, http.MethodPost, srv.URL+"/practices/"+practiceID+"/push-subscriptions", []byte("not json"))
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusBadRequest)
	}
}

func TestRegisterHandler_MissingFieldsRejected(t *testing.T) {
	db := testdb.New(t)
	const identityUID = "staff-missing-fields"
	practiceID := seedPractice(t, db, "Practice")
	seedStaffAtPractice(t, db, practiceID, identityUID)

	srv := newStaffServer(fakeVerifier{uid: identityUID}, db)
	defer srv.Close()

	body, _ := json.Marshal(pushsub.SubscribeRequest{})
	resp := authedRequest(t, http.MethodPost, srv.URL+"/practices/"+practiceID+"/push-subscriptions", body)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusBadRequest)
	}
}

func TestUnregisterHandler_Success(t *testing.T) {
	db := testdb.New(t)
	const identityUID = "staff-unregisters"
	practiceID := seedPractice(t, db, "Practice")
	seedStaffAtPractice(t, db, practiceID, identityUID)

	srv := newStaffServer(fakeVerifier{uid: identityUID}, db)
	defer srv.Close()

	const endpoint = "https://push.example.com/staff-leaving"
	regResp := authedRequest(t, http.MethodPost, srv.URL+"/practices/"+practiceID+"/push-subscriptions", subscribeBody(t, endpoint))
	_ = regResp.Body.Close()

	unregResp := authedRequest(t, http.MethodDelete, srv.URL+"/practices/"+practiceID+"/push-subscriptions?endpoint="+endpoint, nil)
	defer unregResp.Body.Close()
	if unregResp.StatusCode != http.StatusNoContent {
		t.Fatalf("status = %d, want %d", unregResp.StatusCode, http.StatusNoContent)
	}
	if got := countSubscriptions(t, db, endpoint); got != 0 {
		t.Fatalf("push_subscriptions rows for endpoint = %d, want 0 after unregister", got)
	}
}

func TestUnregisterHandler_MissingEndpointRejected(t *testing.T) {
	db := testdb.New(t)
	const identityUID = "staff-missing-endpoint"
	practiceID := seedPractice(t, db, "Practice")
	seedStaffAtPractice(t, db, practiceID, identityUID)

	srv := newStaffServer(fakeVerifier{uid: identityUID}, db)
	defer srv.Close()

	resp := authedRequest(t, http.MethodDelete, srv.URL+"/practices/"+practiceID+"/push-subscriptions", nil)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusBadRequest)
	}
}

// TestUnregisterHandler_CannotDeleteAnotherStaffMembersSubscription proves
// push_subscriptions' self-identity-only RLS (00008_messaging.sql)
// protects the unregister path too: Staff B's unregister request naming
// Staff A's endpoint deletes nothing.
func TestUnregisterHandler_CannotDeleteAnotherStaffMembersSubscription(t *testing.T) {
	db := testdb.New(t)
	practiceID := seedPractice(t, db, "Shared Practice")
	seedStaffAtPractice(t, db, practiceID, "staff-a-owns-sub")
	seedStaffAtPractice(t, db, practiceID, "staff-b-attacker")

	const endpoint = "https://push.example.com/staff-a-device"
	srvA := newStaffServer(fakeVerifier{uid: "staff-a-owns-sub"}, db)
	defer srvA.Close()
	regResp := authedRequest(t, http.MethodPost, srvA.URL+"/practices/"+practiceID+"/push-subscriptions", subscribeBody(t, endpoint))
	_ = regResp.Body.Close()

	srvB := newStaffServer(fakeVerifier{uid: "staff-b-attacker"}, db)
	defer srvB.Close()
	unregResp := authedRequest(t, http.MethodDelete, srvB.URL+"/practices/"+practiceID+"/push-subscriptions?endpoint="+endpoint, nil)
	defer unregResp.Body.Close()
	if unregResp.StatusCode != http.StatusNoContent {
		t.Fatalf("status = %d, want %d (a no-op delete is still a success response)", unregResp.StatusCode, http.StatusNoContent)
	}
	if got := countSubscriptions(t, db, endpoint); got != 1 {
		t.Fatalf("push_subscriptions rows for endpoint = %d, want 1 (Staff A's row must survive)", got)
	}
}

func TestClientRegisterHandler_Success(t *testing.T) {
	db := testdb.New(t)
	const identityUID = "client-registers"
	practiceID := seedPractice(t, db, "Practice")
	clientID, engagementID := seedClientEngagement(t, db, practiceID)
	seedPortalUser(t, db, identityUID, clientID)

	srv := newPortalServer(fakeVerifier{uid: identityUID}, db)
	defer srv.Close()

	const endpoint = "https://push.example.com/client-device"
	resp := authedRequest(t, http.MethodPost, srv.URL+"/portal/engagements/"+engagementID+"/push-subscriptions", subscribeBody(t, endpoint))
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusNoContent {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusNoContent)
	}
	if got := countSubscriptions(t, db, endpoint); got != 1 {
		t.Fatalf("push_subscriptions rows for endpoint = %d, want 1", got)
	}
}

func TestClientRegisterHandler_InvalidJSONBody(t *testing.T) {
	db := testdb.New(t)
	const identityUID = "client-bad-json"
	practiceID := seedPractice(t, db, "Practice")
	clientID, engagementID := seedClientEngagement(t, db, practiceID)
	seedPortalUser(t, db, identityUID, clientID)

	srv := newPortalServer(fakeVerifier{uid: identityUID}, db)
	defer srv.Close()

	resp := authedRequest(t, http.MethodPost, srv.URL+"/portal/engagements/"+engagementID+"/push-subscriptions", []byte("not json"))
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusBadRequest)
	}
}

func TestClientUnregisterHandler_Success(t *testing.T) {
	db := testdb.New(t)
	const identityUID = "client-unregisters"
	practiceID := seedPractice(t, db, "Practice")
	clientID, engagementID := seedClientEngagement(t, db, practiceID)
	seedPortalUser(t, db, identityUID, clientID)

	srv := newPortalServer(fakeVerifier{uid: identityUID}, db)
	defer srv.Close()

	const endpoint = "https://push.example.com/client-leaving"
	regResp := authedRequest(t, http.MethodPost, srv.URL+"/portal/engagements/"+engagementID+"/push-subscriptions", subscribeBody(t, endpoint))
	_ = regResp.Body.Close()

	unregResp := authedRequest(t, http.MethodDelete, srv.URL+"/portal/engagements/"+engagementID+"/push-subscriptions?endpoint="+endpoint, nil)
	defer unregResp.Body.Close()
	if unregResp.StatusCode != http.StatusNoContent {
		t.Fatalf("status = %d, want %d", unregResp.StatusCode, http.StatusNoContent)
	}
	if got := countSubscriptions(t, db, endpoint); got != 0 {
		t.Fatalf("push_subscriptions rows for endpoint = %d, want 0 after unregister", got)
	}
}

func TestClientUnregisterHandler_MissingEndpointRejected(t *testing.T) {
	db := testdb.New(t)
	const identityUID = "client-missing-endpoint"
	practiceID := seedPractice(t, db, "Practice")
	clientID, engagementID := seedClientEngagement(t, db, practiceID)
	seedPortalUser(t, db, identityUID, clientID)

	srv := newPortalServer(fakeVerifier{uid: identityUID}, db)
	defer srv.Close()

	resp := authedRequest(t, http.MethodDelete, srv.URL+"/portal/engagements/"+engagementID+"/push-subscriptions", nil)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusBadRequest)
	}
}
