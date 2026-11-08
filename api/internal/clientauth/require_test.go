package clientauth_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"doula-cloud/api/internal/authntest"
	"doula-cloud/api/internal/clientauth"
	"doula-cloud/api/internal/portalaccount"
	"doula-cloud/api/internal/testdb"
)

// TestRequireTx_HandsBackTheMiddlewareValues proves the three Require
// helpers return what Middleware resolved, so a handler behind it reads
// the tx and the ids in one call.
func TestRequireTx_HandsBackTheMiddlewareValues(t *testing.T) {
	db := testdb.New(t)
	const identityUID = portalaccount.Prefix + "client-for-require-tx"
	clientID, engagementID := seedClientWithEngagement(t, db, identityUID)

	mux := http.NewServeMux()
	mux.Handle("/portal/engagements/{engagementId}/ping", clientauth.Middleware(db.App)(
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			tx, ok := clientauth.RequireTx(w, r)
			if !ok || tx == nil {
				http.Error(w, "RequireTx", http.StatusInternalServerError)
				return
			}
			_, gotEngagement, ok := clientauth.RequireEngagementTx(w, r)
			if !ok || gotEngagement != engagementID {
				http.Error(w, "RequireEngagementTx", http.StatusInternalServerError)
				return
			}
			_, gotClient, ok := clientauth.RequireClientTx(w, r)
			if !ok || gotClient != clientID {
				http.Error(w, "RequireClientTx", http.StatusInternalServerError)
				return
			}
			w.WriteHeader(http.StatusNoContent)
		}),
	))
	srv := httptest.NewServer(mux)
	defer srv.Close()

	resp := get(t, pingURL(srv, engagementID), func(req *http.Request) {
		authntest.AddSessionCookie(req, authntest.SeedSession(t, db.App, identityUID))
	})
	defer resp.Body.Close()
	assertStatus(t, resp, http.StatusNoContent)
}
