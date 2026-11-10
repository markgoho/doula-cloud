package clientauth_test

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"doula-cloud/api/internal/portalaccount"
	"doula-cloud/api/internal/reqtest"
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

func TestMiddleware_MissingCredential(t *testing.T) {
	db := testdb.New(t)
	srv, _ := newServer(t, db, "no-cookie-sent")
	defer srv.Close()

	resp := reqtest.Get(t, pingURL(srv, emptyUUID))
	defer resp.Body.Close()
	reqtest.RequireStatus(t, resp, http.StatusUnauthorized)
}

// TestMiddleware_BearerTokenAloneIsRejected is #151's AC on the Client
// portal: a request carrying only a Bearer ID token gets a 401. The
// portal user behind the token exists and is linked to the Engagement,
// so a 401 can only mean the header was never read.
func TestMiddleware_BearerTokenAloneIsRejected(t *testing.T) {
	db := testdb.New(t)
	const identityUID = portalaccount.Prefix + "client-holding-only-a-bearer-token"
	_, engagementID := seedClientWithEngagement(t, db, identityUID)

	srv, _ := newServer(t, db, identityUID)
	defer srv.Close()

	resp := reqtest.Get(t, pingURL(srv, engagementID), reqtest.Header("Authorization", "Bearer would-verify-fine"))
	defer resp.Body.Close()
	reqtest.RequireStatus(t, resp, http.StatusUnauthorized)
}

// TestMiddleware_UnknownSession covers a cookie that names no live
// session -- the shape a stale or forged cookie arrives in.
func TestMiddleware_UnknownSession(t *testing.T) {
	db := testdb.New(t)
	srv, _ := newServer(t, db, "irrelevant")
	defer srv.Close()

	resp := reqtest.Get(t, pingURL(srv, emptyUUID), reqtest.Session("never-issued"))
	defer resp.Body.Close()
	reqtest.RequireStatus(t, resp, http.StatusUnauthorized)
}

func TestMiddleware_InvalidEngagementID(t *testing.T) {
	db := testdb.New(t)
	srv, session := newServer(t, db, portalaccount.Prefix+"some-uid")
	defer srv.Close()

	resp := reqtest.Get(t, pingURL(srv, "not-a-uuid"), reqtest.Session(session))
	defer resp.Body.Close()
	reqtest.RequireStatus(t, resp, http.StatusBadRequest)
}

func TestMiddleware_PopulationResolutionFailure(t *testing.T) {
	db := testdb.New(t)
	// A verified uid with no matching client_portal_users row: population
	// resolution fails even though the token itself is valid.
	srv, session := newServer(t, db, portalaccount.Prefix+"unknown-uid")
	defer srv.Close()

	resp := reqtest.Get(t, pingURL(srv, emptyUUID), reqtest.Session(session))
	defer resp.Body.Close()

	reqtest.RequireStatus(t, resp, http.StatusForbidden)
}

func TestMiddleware_EngagementNotLinkedToClient(t *testing.T) {
	db := testdb.New(t)
	const identityUID = portalaccount.Prefix + "client-without-this-engagement"
	_, _ = seedClientWithEngagement(t, db, identityUID)

	// A different, unrelated Client's Engagement: the caller is a known
	// Client-portal user, but not linked to this Engagement.
	otherPracticeID := testdb.SeedPractice(t, db, "Other Practice")
	_, otherEngagementID := testdb.SeedEngagementInStatus(t, db, otherPracticeID, "Other Client", "other@example.com", "intake")

	srv, session := newServer(t, db, identityUID)
	defer srv.Close()

	resp := reqtest.Get(t, pingURL(srv, otherEngagementID), reqtest.Session(session))
	defer resp.Body.Close()

	reqtest.RequireStatus(t, resp, http.StatusForbidden)
}

func TestMiddleware_Success(t *testing.T) {
	db := testdb.New(t)
	const identityUID = portalaccount.Prefix + "client-with-engagement"
	clientID, engagementID := seedClientWithEngagement(t, db, identityUID)

	srv, session := newServer(t, db, identityUID)
	defer srv.Close()

	resp := reqtest.Get(t, pingURL(srv, engagementID), reqtest.Session(session))
	defer resp.Body.Close()

	reqtest.RequireStatus(t, resp, http.StatusOK)
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

// TestMiddleware_ResolvesTheClientThatOwnsTheEngagementAcrossPractices
// proves #309's own correctness requirement on the read side: a Portal
// Account with client_portal_users rows at two Practices (ADR-0015 --
// possible once #309 lifted identity_uid's UNIQUE constraint) must still
// authorize a request for either Practice's Engagement, not just
// whichever row Postgres happens to return first.
func TestMiddleware_ResolvesTheClientThatOwnsTheEngagementAcrossPractices(t *testing.T) {
	db := testdb.New(t)
	const identityUID = portalaccount.Prefix + "client-at-two-practices"

	practiceA := testdb.SeedPractice(t, db, "Practice A")
	clientA, _ := testdb.SeedEngagementInStatus(t, db, practiceA, "Client A", "a@example.com", "intake")
	testdb.SeedPortalUser(t, db, identityUID, clientA)

	practiceB := testdb.SeedPractice(t, db, "Practice B")
	clientB, engagementB := testdb.SeedEngagementInStatus(t, db, practiceB, "Client B", "b@example.com", "intake")
	// The same Portal Account reaching a second Practice (#309): a second
	// client_portal_users row, not a second seedPortalUser call -- the
	// Portal Account itself already exists from clientA's call above.
	testdb.AttachPortalUser(t, db, identityUID, clientB)

	srv, session := newServer(t, db, identityUID)
	defer srv.Close()

	resp := reqtest.Get(t, pingURL(srv, engagementB), reqtest.Session(session))
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want %d (Practice B's own Engagement, reached through the second client_portal_users row)", resp.StatusCode, http.StatusOK)
	}
	if got := resp.Header.Get("X-Client-Id"); got != clientB {
		t.Fatalf("X-Client-Id = %q, want %q", got, clientB)
	}
}

// TestMiddleware_PendingDeletionLockout is #871's decision 3 on the
// Client population: "No Staff or Client login succeeds against a
// Practice pending deletion" -- unlike staffauth.Middleware, there is no
// Owner-shaped exemption to test here, since a Client has nothing to
// restore.
func TestMiddleware_PendingDeletionLockout(t *testing.T) {
	db := testdb.New(t)
	const identityUID = portalaccount.Prefix + "client-pending-deletion"
	_, engagementID := seedClientWithEngagement(t, db, identityUID)

	practiceID := testdb.Scalar[string](t, db.Admin, `SELECT practice_id FROM engagements WHERE id = $1`, engagementID)
	testdb.SeedPracticeDeletion(t, db, practiceID, time.Now(), time.Now().AddDate(0, 0, 30), nil)

	srv, session := newServer(t, db, identityUID)
	defer srv.Close()

	reqtest.ExpectStatus(t, http.MethodGet, pingURL(srv, engagementID), http.StatusForbidden, reqtest.Session(session))
}
