package clientauth_test

import (
	"encoding/json/v2"
	"net/http"
	"net/http/httptest"
	"testing"

	"doula-cloud/api/internal/authntest"
	"doula-cloud/api/internal/reqtest"
	"doula-cloud/api/internal/testdb"
)

func postPriorLink(t *testing.T, srv *httptest.Server, session, body string) *http.Response {
	t.Helper()
	return reqtest.Post(t, srv.URL+"/api/portal/session/prior-link", reqtest.Session(session), reqtest.RawJSON([]byte(body)))
}

func priorLinkReturnTo(t *testing.T, srv *httptest.Server, session, token string) string {
	t.Helper()
	resp := postPriorLink(t, srv, session, `{"token":"`+token+`"}`)
	defer resp.Body.Close()
	reqtest.RequireStatus(t, resp, http.StatusOK)
	var body struct {
		ReturnTo string `json:"returnTo"`
	}
	if err := json.UnmarshalRead(resp.Body, &body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	return body.ReturnTo
}

// #2536: a Client already signed in who opens a used link of her own is
// told the page it was asked for from.
func TestPriorLink_ASignedInClientGetsHerUsedLinksPage(t *testing.T) {
	db := testdb.New(t)
	const identifier = "portal_prior-signedin"
	testdb.SeedPortalAccount(t, db, identifier, "signedin@example.com")
	token := requestLink(t, db, identifier, requestBody(t, "signedin@example.com", priorBirthPlan, ""))
	if status, _ := redeemLink(t, db, token); status != http.StatusOK {
		t.Fatalf("redeem status = %d, want %d", status, http.StatusOK)
	}
	session := authntest.SeedSession(t, db.App, identifier)

	srv := newMagicLinkRequestServer(db)
	defer srv.Close()
	if got := priorLinkReturnTo(t, srv, session, token); got != priorBirthPlan {
		t.Fatalf("returnTo = %q, want %q", got, priorBirthPlan)
	}
}

// #2536: the token is a handle on a row stored for one account, so
// another account's link, or one never issued, names nothing.
func TestPriorLink_NamesOnlyTheSessionsOwnAccountsPage(t *testing.T) {
	db := testdb.New(t)
	testdb.SeedPortalAccount(t, db, "portal_prior-owner", "owner@example.com")
	testdb.SeedPortalAccount(t, db, "portal_prior-reader", "reader@example.com")
	others := requestLink(t, db, "portal_prior-owner", requestBody(t, "owner@example.com", priorBirthPlan, ""))
	session := authntest.SeedSession(t, db.App, "portal_prior-reader")

	srv := newMagicLinkRequestServer(db)
	defer srv.Close()
	for name, token := range map[string]string{"another account's": others, "never issued": "crafted-token", "empty": ""} {
		t.Run(name, func(t *testing.T) {
			if got := priorLinkReturnTo(t, srv, session, token); got != "" {
				t.Fatalf("returnTo = %q, want none", got)
			}
		})
	}
}

func TestPriorLink_RefusesWithoutASession(t *testing.T) {
	db := testdb.New(t)
	srv := newMagicLinkRequestServer(db)
	defer srv.Close()

	resp := postPriorLink(t, srv, "", `{"token":"anything"}`)
	defer resp.Body.Close()
	reqtest.RequireStatus(t, resp, http.StatusUnauthorized)
}

func TestPriorLink_RefusesABodyThatIsNotJSON(t *testing.T) {
	db := testdb.New(t)
	testdb.SeedPortalAccount(t, db, "portal_prior-badbody", "badbody@example.com")
	session := authntest.SeedSession(t, db.App, "portal_prior-badbody")
	srv := newMagicLinkRequestServer(db)
	defer srv.Close()

	resp := postPriorLink(t, srv, session, `not json`)
	defer resp.Body.Close()
	reqtest.RequireStatus(t, resp, http.StatusBadRequest)
}
