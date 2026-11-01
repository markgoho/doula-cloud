package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"doula-cloud/api/internal/activity"
	"doula-cloud/api/internal/apierr"
	"doula-cloud/api/internal/apierrtest"
	"doula-cloud/api/internal/authntest"
	"doula-cloud/api/internal/legal"
	"doula-cloud/api/internal/staffauth"
	"doula-cloud/api/internal/testdb"
)

// TestAgreementGate_RealRoutes is #1549's first three boxes driven
// through the real route table, after a published material version of
// the Terms has started and before any Owner agreed: an Owner is refused
// an ordinary Practice-scoped read with TERMS_AGREEMENT_REQUIRED; the
// export, the deletion status, signing out and the Credit refund are not
// refused for this reason; and a Doula at the same Practice opens a
// Client as usual.
func TestAgreementGate_RealRoutes(t *testing.T) {
	shipped := staffauth.AgreementDocuments
	staffauth.AgreementDocuments = []legal.Document{
		{Name: "Terms of Service", Path: "/terms", Versions: []legal.Version{
			{Effective: "2026-09-29", Material: false, Change: "First version."},
			{Effective: "2026-11-01", Material: true, Change: "Raises the price of a Credit."},
		}},
		legal.Privacy,
	}
	t.Cleanup(func() { staffauth.AgreementDocuments = shipped })

	now := time.Date(2026, 11, 2, 15, 0, 0, 0, time.UTC)
	db := testdb.New(t)
	const ownerUID, doulaUID = "owner-real-routes", "doula-real-routes"
	practiceID, ownerID := testdb.SeedStaffAtNewPractice(t, db, ownerUID, []string{roleOwner}, "employee")
	testdb.SeedStaffAtPractice(t, db, practiceID, doulaUID, []string{roleDoula}, "employee")
	clientID := testdb.SeedNamedClient(t, db, practiceID, "Sarah", "sarah@example.test")
	testdb.SeedActivityWithDiff(t, db, practiceID, activity.SubjectPractice, practiceID, staffauth.ActionTermsAccepted,
		activity.StaffActor(ownerID), json.RawMessage(`{"termsVersion":"2026-09-29","privacyVersion":"2026-10-22","ipAddress":"127.0.0.1"}`))
	testdb.SeedLegalPublication(t, db, "terms", "2026-11-01", true, now)

	deps := testDeps()
	deps.DB = db.App
	deps.Now = func() time.Time { return now }
	deps.Verifier = authntest.Verifier{UID: ownerUID, AuthTime: now}
	mux, _, _ := routes(deps)
	srv := httptest.NewServer(mux)
	defer srv.Close()
	owner := authntest.SeedSessionAt(t, db.App, ownerUID, now)
	doula := authntest.SeedSessionAt(t, db.App, doulaUID, now)

	// do sends one request and answers its status and, for a 403, the
	// refusal's code.
	do := func(method, path, session string, body any) (int, apierr.Code) {
		t.Helper()
		var buf bytes.Buffer
		if body != nil {
			if err := json.NewEncoder(&buf).Encode(body); err != nil {
				t.Fatalf("encode: %v", err)
			}
		}
		req, err := http.NewRequestWithContext(t.Context(), method, srv.URL+path, &buf)
		if err != nil {
			t.Fatalf("build request: %v", err)
		}
		if session != "" {
			authntest.AddSessionCookie(req, session)
		}
		req.Header.Set("X-Internal-Secret", testWorkerSecret)
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatalf("request: %v", err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusForbidden {
			return resp.StatusCode, ""
		}
		return resp.StatusCode, apierrtest.Decode(t, resp).Code
	}
	base := "/api/practices/" + practiceID

	if _, code := do(http.MethodGet, base+"/clients/"+clientID, owner, nil); code != apierr.CodeTermsAgreementRequired {
		t.Fatalf("the Owner's Client read code = %q, want %q", code, apierr.CodeTermsAgreementRequired)
	}
	if status, _ := do(http.MethodGet, base+"/clients/"+clientID, doula, nil); status != http.StatusOK {
		t.Fatalf("the Doula's Client read status = %d, want %d", status, http.StatusOK)
	}

	for _, open := range []struct {
		name, method, path string
		body               any
	}{
		{"export", http.MethodGet, base + "/export", nil},
		{"deletion status", http.MethodGet, base + "/deletion", nil},
		{"sign out", http.MethodDelete, "/api/session", nil},
	} {
		if _, code := do(open.method, open.path, owner, open.body); code == apierr.CodeTermsAgreementRequired {
			t.Errorf("%s refused for want of an agreement, want it left open", open.name)
		}
	}
}
