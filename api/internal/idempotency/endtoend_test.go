package idempotency_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"doula-cloud/api/internal/idempotency"
	"doula-cloud/api/internal/portalinvite"
	"doula-cloud/api/internal/staffauth"
	"doula-cloud/api/internal/testdb"
)

// TestEndToEnd_PortalInviteReplaysOnRetry wires portalinvite.InviteHandler
// behind staffauth.Middleware(...)(idempotency.Wrap(...)) exactly as
// main.go does, proving the helper is genuinely usable against a real
// mutating endpoint (#126's AC), not just the synthetic handler in
// wrap_test.go.
func TestEndToEnd_PortalInviteReplaysOnRetry(t *testing.T) {
	db := testdb.New(t)
	const identityUID = "e2e-portal-invite-staff"
	practiceID := seedStaffWithMembership(t, db, identityUID)
	_, engagementID := seedClientEngagement(t, db, practiceID, "E2E Client", "e2e@example.com")

	mux := http.NewServeMux()
	mux.Handle("POST /practices/{practiceId}/engagements/{engagementId}/portal-invite",
		staffauth.Middleware(fakeVerifier{uid: identityUID}, db.App)(idempotency.Wrap(portalinvite.InviteHandler())))
	srv := httptest.NewServer(mux)
	defer srv.Close()

	postInvite := func(key string) *http.Response {
		t.Helper()
		url := srv.URL + "/practices/" + practiceID + "/engagements/" + engagementID + "/portal-invite"
		req, err := http.NewRequestWithContext(t.Context(), http.MethodPost, url, nil)
		if err != nil {
			t.Fatalf("build request: %v", err)
		}
		req.Header.Set("Authorization", "Bearer tok")
		if key != "" {
			req.Header.Set("Idempotency-Key", key)
		}
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatalf("request: %v", err)
		}
		return resp
	}
	decode := func(resp *http.Response) portalinvite.InviteResponse {
		t.Helper()
		var out portalinvite.InviteResponse
		if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
			t.Fatalf("decode response: %v", err)
		}
		return out
	}

	first := postInvite("portal-invite-retry-key")
	defer first.Body.Close()
	if first.StatusCode != http.StatusCreated {
		t.Fatalf("first call status = %d, want %d", first.StatusCode, http.StatusCreated)
	}
	firstOut := decode(first)

	retried := postInvite("portal-invite-retry-key")
	defer retried.Body.Close()
	if retried.StatusCode != http.StatusCreated {
		t.Fatalf("retried call status = %d, want %d", retried.StatusCode, http.StatusCreated)
	}
	retriedOut := decode(retried)

	if retriedOut.InviteToken != firstOut.InviteToken {
		t.Fatalf("retried inviteToken = %q, want identical stored token %q -- business logic must not re-run on replay",
			retriedOut.InviteToken, firstOut.InviteToken)
	}

	noKey := postInvite("")
	defer noKey.Body.Close()
	noKeyOut := decode(noKey)
	if noKeyOut.InviteToken == firstOut.InviteToken {
		t.Fatalf("call with no Idempotency-Key returned the same token as the earlier call -- expected the handler to re-run and rotate it")
	}
}
