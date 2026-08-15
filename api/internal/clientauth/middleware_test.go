package clientauth_test

import (
	"net/http"
	"testing"

	"doula-cloud/api/internal/testdb"
)

func TestMiddleware_MissingToken(t *testing.T) {
	db := testdb.New(t)
	srv := newServer(fakeVerifier{}, db)
	defer srv.Close()

	req, err := http.NewRequestWithContext(t.Context(), http.MethodGet, srv.URL+"/portal/engagements/00000000-0000-0000-0000-000000000000/ping", nil)
	if err != nil {
		t.Fatalf("build request: %v", err)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusUnauthorized)
	}
}

func TestMiddleware_EmptyBearerToken(t *testing.T) {
	db := testdb.New(t)
	srv := newServer(fakeVerifier{}, db)
	defer srv.Close()

	req, _ := http.NewRequestWithContext(t.Context(), http.MethodGet, srv.URL+"/portal/engagements/00000000-0000-0000-0000-000000000000/ping", nil)
	req.Header.Set("Authorization", "Bearer ")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusUnauthorized)
	}
}

func TestMiddleware_TokenVerificationFailure(t *testing.T) {
	db := testdb.New(t)
	srv := newServer(fakeVerifier{err: errBadToken}, db)
	defer srv.Close()

	req, _ := http.NewRequestWithContext(t.Context(), http.MethodGet, srv.URL+"/portal/engagements/00000000-0000-0000-0000-000000000000/ping", nil)
	req.Header.Set("Authorization", "Bearer some-token")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusUnauthorized)
	}
}

func TestMiddleware_InvalidEngagementID(t *testing.T) {
	db := testdb.New(t)
	srv := newServer(fakeVerifier{uid: "some-uid"}, db)
	defer srv.Close()

	req, _ := http.NewRequestWithContext(t.Context(), http.MethodGet, srv.URL+"/portal/engagements/not-a-uuid/ping", nil)
	req.Header.Set("Authorization", "Bearer some-token")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusBadRequest)
	}
}

func TestMiddleware_PopulationResolutionFailure(t *testing.T) {
	db := testdb.New(t)
	// A verified uid with no matching client_portal_users row: population
	// resolution fails even though the token itself is valid.
	srv := newServer(fakeVerifier{uid: "unknown-uid"}, db)
	defer srv.Close()

	req, _ := http.NewRequestWithContext(t.Context(), http.MethodGet, srv.URL+"/portal/engagements/00000000-0000-0000-0000-000000000000/ping", nil)
	req.Header.Set("Authorization", "Bearer some-token")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusForbidden)
	}
}

func TestMiddleware_EngagementNotLinkedToClient(t *testing.T) {
	db := testdb.New(t)
	const identityUID = "client-without-this-engagement"
	_, _ = seedClientWithEngagement(t, db, identityUID)

	// A different, unrelated Client's Engagement: the caller is a known
	// Client-portal user, but not linked to this Engagement.
	otherPracticeID := seedPractice(t, db, "Other Practice")
	_, otherEngagementID := seedClientEngagement(t, db, otherPracticeID, "Other Client", "other@example.com", "intake")

	srv := newServer(fakeVerifier{uid: identityUID}, db)
	defer srv.Close()

	req, _ := http.NewRequestWithContext(t.Context(), http.MethodGet, srv.URL+"/portal/engagements/"+otherEngagementID+"/ping", nil)
	req.Header.Set("Authorization", "Bearer some-token")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusForbidden)
	}
}

func TestMiddleware_Success(t *testing.T) {
	db := testdb.New(t)
	const identityUID = "client-with-engagement"
	clientID, engagementID := seedClientWithEngagement(t, db, identityUID)

	srv := newServer(fakeVerifier{uid: identityUID}, db)
	defer srv.Close()

	req, _ := http.NewRequestWithContext(t.Context(), http.MethodGet, srv.URL+"/portal/engagements/"+engagementID+"/ping", nil)
	req.Header.Set("Authorization", "Bearer some-token")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}
	if got := resp.Header.Get("X-Client-Id"); got != clientID {
		t.Fatalf("X-Client-Id = %q, want %q", got, clientID)
	}
	if got := resp.Header.Get("X-Engagement-Id"); got != engagementID {
		t.Fatalf("X-Engagement-Id = %q, want %q", got, engagementID)
	}
}
