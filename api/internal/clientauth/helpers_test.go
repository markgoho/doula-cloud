package clientauth_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"doula-cloud/api/internal/authntest"
	"doula-cloud/api/internal/clientauth"
	"doula-cloud/api/internal/testdb"
)

// newServer wires the middleware in front of a handler that echoes the
// resolved Client/Engagement ids and confirms a usable *sql.Tx was placed
// on the request context, so tests can assert on the middleware's
// contract with downstream handlers, not just the HTTP status code. It
// also seeds a live session for uid and hands back the token its
// __session cookie carries, since #151 that cookie is the only
// credential the middleware reads.
func newServer(t *testing.T, db *testdb.DB, uid string) (srv *httptest.Server, session string) {
	t.Helper()
	mux := http.NewServeMux()
	mux.Handle("/portal/engagements/{engagementId}/ping", clientauth.Middleware(db.App)(
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			clientID, _ := clientauth.ClientID(r.Context())
			engagementID, _ := clientauth.EngagementID(r.Context())
			identityUID, _ := clientauth.IdentityUID(r.Context())
			tx, ok := clientauth.Tx(r.Context())
			if !ok || tx == nil {
				http.Error(w, "no tx on context", http.StatusInternalServerError)
				return
			}
			w.Header().Set("X-Client-Id", clientID)
			w.Header().Set("X-Engagement-Id", engagementID)
			w.Header().Set("X-Identity-Uid", identityUID)
			w.WriteHeader(http.StatusOK)
		}),
	))
	return httptest.NewServer(mux), authntest.SeedSession(t, db.App, uid)
}

// seedClientWithEngagement inserts a Practice, a Client with an
// Engagement at it, and a client_portal_users row linking identityUID to
// that Client -- the full fixture most middleware tests need.
func seedClientWithEngagement(t *testing.T, db *testdb.DB, identityUID string) (clientID, engagementID string) {
	t.Helper()

	practiceID := testdb.SeedPractice(t, db, "Test Practice")
	clientID, engagementID = testdb.SeedEngagementInStatus(t, db, practiceID, "Test Client", "client@example.com", "intake")
	testdb.SeedPortalUser(t, db, identityUID, clientID)
	return clientID, engagementID
}
