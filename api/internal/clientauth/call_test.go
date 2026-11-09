package clientauth_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"doula-cloud/api/internal/authntest"
	"doula-cloud/api/internal/clientauth"
	"doula-cloud/api/internal/portalaccount"
	"doula-cloud/api/internal/reqtest"
	"doula-cloud/api/internal/testdb"
)

func TestHandler_HandsOverWhatMiddlewareResolved(t *testing.T) {
	db := testdb.New(t)
	const identityUID = portalaccount.Prefix + "client-for-call"
	clientID, engagementID := seedClientWithEngagement(t, db, identityUID)

	mux := http.NewServeMux()
	mux.Handle("/portal/engagements/{engagementId}/ping", clientauth.Middleware(db.App)(
		clientauth.Handler(func(w http.ResponseWriter, _ *http.Request, c clientauth.Call) {
			if c.Tx == nil || c.ClientID != clientID || c.EngagementID != engagementID || c.IdentityUID != identityUID {
				http.Error(w, "wrong Call", http.StatusInternalServerError)
				return
			}
			w.WriteHeader(http.StatusNoContent)
		}),
	))
	srv := httptest.NewServer(mux)
	defer srv.Close()

	reqtest.ExpectStatus(t, http.MethodGet, pingURL(srv, engagementID), http.StatusNoContent, reqtest.Session(authntest.SeedSession(t, db.App, identityUID)))
}

func TestHandler_AnswersInternalErrorWhenMiddlewareNeverRan(t *testing.T) {
	rec := httptest.NewRecorder()
	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/", nil)
	clientauth.Handler(func(http.ResponseWriter, *http.Request, clientauth.Call) {
		t.Fatal("handler ran without Middleware")
	}).ServeHTTP(rec, req)
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500", rec.Code)
	}
}
