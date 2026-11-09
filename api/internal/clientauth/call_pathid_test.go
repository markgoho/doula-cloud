package clientauth_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"doula-cloud/api/internal/apierrtest"
	"doula-cloud/api/internal/authntest"
	"doula-cloud/api/internal/clientauth"
	"doula-cloud/api/internal/portalaccount"
	"doula-cloud/api/internal/reqtest"
	"doula-cloud/api/internal/testdb"
)

func TestHandler_PathIDExposesAUUIDAndRefusesTheRest(t *testing.T) {
	db := testdb.New(t)
	const identityUID = portalaccount.Prefix + "client-for-pathid"
	_, engagementID := seedClientWithEngagement(t, db, identityUID)
	const messageID = "11111111-1111-4111-8111-111111111111"

	mux := http.NewServeMux()
	mux.Handle("/portal/engagements/{engagementId}/messages/{messageId}", clientauth.Middleware(db.App)(
		clientauth.Handler(func(w http.ResponseWriter, _ *http.Request, c clientauth.Call) {
			w.Header().Set("X-Message", c.ID("messageId"))
			w.WriteHeader(http.StatusNoContent)
		}, clientauth.PathID("message", "messageId")),
	))
	srv := httptest.NewServer(mux)
	defer srv.Close()
	session := reqtest.Session(authntest.SeedSession(t, db.App, identityUID))
	base := srv.URL + "/portal/engagements/" + engagementID + "/messages/"

	ok := reqtest.Get(t, base+messageID, session)
	defer ok.Body.Close()
	reqtest.RequireStatus(t, ok, http.StatusNoContent)
	if got := ok.Header.Get("X-Message"); got != messageID {
		t.Fatalf("Call.ID(messageId) = %q, want %q", got, messageID)
	}

	bad := reqtest.Get(t, base+"nope", session)
	defer bad.Body.Close()
	reqtest.RequireStatus(t, bad, http.StatusBadRequest)
	if got := apierrtest.Decode(t, bad).Message; got != "invalid message id" {
		t.Fatalf("message = %q, want invalid message id", got)
	}
}

func TestCall_IDPanicsOnAnUndeclaredWildcard(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Fatal("expected a panic for a wildcard no PathID declared")
		}
	}()
	clientauth.Call{}.ID("messageId")
}
