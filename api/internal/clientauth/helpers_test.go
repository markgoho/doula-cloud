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
			tx, ok := clientauth.Tx(r.Context())
			if !ok || tx == nil {
				http.Error(w, "no tx on context", http.StatusInternalServerError)
				return
			}
			w.Header().Set("X-Client-Id", clientID)
			w.Header().Set("X-Engagement-Id", engagementID)
			w.WriteHeader(http.StatusOK)
		}),
	))
	return httptest.NewServer(mux), authntest.SeedSession(t, db.App, uid)
}

// seedPractice inserts a Practice using the superuser Admin connection.
func seedPractice(t *testing.T, db *testdb.DB, name string) string {
	t.Helper()
	var id string
	if err := db.Admin.QueryRowContext(t.Context(), `INSERT INTO practices (name) VALUES ($1) RETURNING id`, name).Scan(&id); err != nil {
		t.Fatalf("seed practice %q: %v", name, err)
	}
	return id
}

// seedClientEngagement inserts a Client and an Engagement linking them to
// practiceID, using the superuser Admin connection.
func seedClientEngagement(t *testing.T, db *testdb.DB, practiceID, name, email, status string) (clientID, engagementID string) {
	t.Helper()

	if err := db.Admin.QueryRowContext(t.Context(),
		`INSERT INTO clients (name, email) VALUES ($1, $2) RETURNING id`,
		name, email,
	).Scan(&clientID); err != nil {
		t.Fatalf("seed client: %v", err)
	}
	if err := db.Admin.QueryRowContext(t.Context(),
		`INSERT INTO engagements (client_id, practice_id, status) VALUES ($1, $2, $3) RETURNING id`,
		clientID, practiceID, status,
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

// seedClientWithEngagement inserts a Practice, a Client with an
// Engagement at it, and a client_portal_users row linking identityUID to
// that Client -- the full fixture most middleware tests need.
func seedClientWithEngagement(t *testing.T, db *testdb.DB, identityUID string) (clientID, engagementID string) {
	t.Helper()

	practiceID := seedPractice(t, db, "Test Practice")
	clientID, engagementID = seedClientEngagement(t, db, practiceID, "Test Client", "client@example.com", "intake")
	seedPortalUser(t, db, identityUID, clientID)
	return clientID, engagementID
}
