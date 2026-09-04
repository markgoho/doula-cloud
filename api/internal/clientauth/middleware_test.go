package clientauth_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"doula-cloud/api/internal/authntest"
	"doula-cloud/api/internal/testdb"
)

// emptyUUID is a well-formed Engagement id that matches nothing, for the
// tests whose request never gets far enough for it to matter.
const emptyUUID = "00000000-0000-0000-0000-000000000000"

// pingURL is the middleware-guarded route every test in this file hits;
// the engagement id only matters once a credential has been accepted.
func pingURL(srv *httptest.Server, engagementID string) string {
	return srv.URL + "/portal/engagements/" + engagementID + "/ping"
}

// get issues a GET with whatever credential setup applies to req.
func get(t *testing.T, url string, setup func(*http.Request)) *http.Response {
	t.Helper()
	req, err := http.NewRequestWithContext(t.Context(), http.MethodGet, url, nil)
	if err != nil {
		t.Fatalf("build request: %v", err)
	}
	setup(req)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	return resp
}

// assertStatus fails the test unless resp carries want.
func assertStatus(t *testing.T, resp *http.Response, want int) {
	t.Helper()
	if resp.StatusCode != want {
		t.Fatalf("status = %d, want %d", resp.StatusCode, want)
	}
}

func TestMiddleware_MissingCredential(t *testing.T) {
	db := testdb.New(t)
	srv, _ := newServer(t, db, "no-cookie-sent")
	defer srv.Close()

	resp := get(t, pingURL(srv, emptyUUID), func(*http.Request) {})
	defer resp.Body.Close()
	assertStatus(t, resp, http.StatusUnauthorized)
}

// TestMiddleware_BearerTokenAloneIsRejected is #151's AC on the Client
// portal: a request carrying only a Bearer ID token gets a 401. The
// portal user behind the token exists and is linked to the Engagement,
// so a 401 can only mean the header was never read.
func TestMiddleware_BearerTokenAloneIsRejected(t *testing.T) {
	db := testdb.New(t)
	const identityUID = "client-holding-only-a-bearer-token"
	_, engagementID := seedClientWithEngagement(t, db, identityUID)

	srv, _ := newServer(t, db, identityUID)
	defer srv.Close()

	resp := get(t, pingURL(srv, engagementID), func(req *http.Request) {
		req.Header.Set("Authorization", "Bearer would-verify-fine")
	})
	defer resp.Body.Close()
	assertStatus(t, resp, http.StatusUnauthorized)
}

// TestMiddleware_UnknownSession covers a cookie that names no live
// session -- the shape a stale or forged cookie arrives in.
func TestMiddleware_UnknownSession(t *testing.T) {
	db := testdb.New(t)
	srv, _ := newServer(t, db, "irrelevant")
	defer srv.Close()

	resp := get(t, pingURL(srv, emptyUUID), func(req *http.Request) {
		authntest.AddSessionCookie(req, "never-issued")
	})
	defer resp.Body.Close()
	assertStatus(t, resp, http.StatusUnauthorized)
}

func TestMiddleware_InvalidEngagementID(t *testing.T) {
	db := testdb.New(t)
	srv, session := newServer(t, db, "some-uid")
	defer srv.Close()

	resp := get(t, pingURL(srv, "not-a-uuid"), func(req *http.Request) {
		authntest.AddSessionCookie(req, session)
	})
	defer resp.Body.Close()
	assertStatus(t, resp, http.StatusBadRequest)
}

func TestMiddleware_PopulationResolutionFailure(t *testing.T) {
	db := testdb.New(t)
	// A verified uid with no matching client_portal_users row: population
	// resolution fails even though the token itself is valid.
	srv, session := newServer(t, db, "unknown-uid")
	defer srv.Close()

	resp := get(t, pingURL(srv, emptyUUID), func(req *http.Request) {
		authntest.AddSessionCookie(req, session)
	})
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

	srv, session := newServer(t, db, identityUID)
	defer srv.Close()

	resp := get(t, pingURL(srv, otherEngagementID), func(req *http.Request) {
		authntest.AddSessionCookie(req, session)
	})
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusForbidden)
	}
}

func TestMiddleware_Success(t *testing.T) {
	db := testdb.New(t)
	const identityUID = "client-with-engagement"
	clientID, engagementID := seedClientWithEngagement(t, db, identityUID)

	srv, session := newServer(t, db, identityUID)
	defer srv.Close()

	resp := get(t, pingURL(srv, engagementID), func(req *http.Request) {
		authntest.AddSessionCookie(req, session)
	})
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
	// #303: IdentityUID is the Portal Account itself, distinct from
	// ClientID -- notificationpref keys its preference store on this.
	if got := resp.Header.Get("X-Identity-Uid"); got != identityUID {
		t.Fatalf("X-Identity-Uid = %q, want %q", got, identityUID)
	}
}
