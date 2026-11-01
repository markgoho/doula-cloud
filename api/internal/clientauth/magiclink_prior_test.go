package clientauth_test

import (
	"encoding/json"
	"net/http"
	"testing"

	"doula-cloud/api/internal/authtoken"
	"doula-cloud/api/internal/clientauth"
	"doula-cloud/api/internal/testdb"
)

const priorBirthPlan = "/portal/engagements/0d6c7c39-1f53-4b8e-9d63-9a2b3c4d5e6f/birth-plan"

func requestLink(t *testing.T, db *testdb.DB, identifier, body string) string {
	t.Helper()
	srv := newMagicLinkRequestServer(db)
	defer srv.Close()
	requested := postJSON(t, srv, "/api/portal/magic-link/request", body)
	_ = requested.Body.Close()
	if requested.StatusCode != http.StatusAccepted {
		t.Fatalf("request status = %d, want %d", requested.StatusCode, http.StatusAccepted)
	}
	return pendingMagicLinkToken(t, db, identifier)
}

func redeemLink(t *testing.T, db *testdb.DB, token string) (status int, returnTo string) {
	t.Helper()
	srv := newMagicLinkRequestServer(db)
	defer srv.Close()
	redeemed := postJSON(t, srv, "/api/portal/magic-link", `{"token":"`+token+`"}`)
	defer redeemed.Body.Close()
	if redeemed.StatusCode != http.StatusOK {
		return redeemed.StatusCode, ""
	}
	var body struct {
		ReturnTo string `json:"returnTo"`
	}
	if err := json.NewDecoder(redeemed.Body).Decode(&body); err != nil {
		t.Fatalf("decode redeem body: %v", err)
	}
	return redeemed.StatusCode, body.ReturnTo
}

func expireLink(t *testing.T, db *testdb.DB, token string) {
	t.Helper()
	if _, err := db.Admin.ExecContext(t.Context(),
		`UPDATE auth_tokens SET expires_at = now() - interval '1 minute' WHERE token_hash = $1`, authtoken.Digest(token),
	); err != nil {
		t.Fatalf("expire token: %v", err)
	}
}

func requestBody(t *testing.T, email, returnTo, priorToken string) string {
	t.Helper()
	encoded, err := json.Marshal(clientauth.RequestMagicLinkRequest{Email: email, ReturnTo: returnTo, PriorToken: priorToken})
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	return string(encoded)
}

// #2534: a link that expired or was already used still names the page it
// was asked for from, so the new one she asks for lands there too.
func TestMagicLink_ANewLinkAfterADeadOneKeepsThePage(t *testing.T) {
	cases := []struct {
		name string
		kill func(t *testing.T, db *testdb.DB, token string)
	}{
		{"expired", expireLink},
		{"used", func(t *testing.T, db *testdb.DB, token string) {
			if status, _ := redeemLink(t, db, token); status != http.StatusOK {
				t.Fatalf("first redeem status = %d, want %d", status, http.StatusOK)
			}
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			db := testdb.New(t)
			const identifier = "portal_prior-kept"
			testdb.SeedPortalAccount(t, db, identifier, "prior@example.com")

			first := requestLink(t, db, identifier, requestBody(t, "prior@example.com", priorBirthPlan, ""))
			tc.kill(t, db, first)
			if status, _ := redeemLink(t, db, first); status != http.StatusBadRequest {
				t.Fatalf("dead link redeem status = %d, want %d", status, http.StatusBadRequest)
			}

			second := requestLink(t, db, identifier, requestBody(t, "prior@example.com", "", first))
			status, got := redeemLink(t, db, second)
			if status != http.StatusOK {
				t.Fatalf("new link redeem status = %d, want %d", status, http.StatusOK)
			}
			if got != priorBirthPlan {
				t.Fatalf("returnTo = %q, want %q", got, priorBirthPlan)
			}
		})
	}
}

// #2534: the dead token is a handle on a row the BFF stored, never an
// address, so a token it never issued, or one issued to another account,
// carries nothing to the new link.
func TestMagicLink_APriorTokenNamesOnlyItsOwnAccountsPage(t *testing.T) {
	db := testdb.New(t)
	testdb.SeedPortalAccount(t, db, "portal_prior-other", "other@example.com")
	testdb.SeedPortalAccount(t, db, "portal_prior-asker", "asker@example.com")
	others := requestLink(t, db, "portal_prior-other", requestBody(t, "other@example.com", priorBirthPlan, ""))
	expireLink(t, db, others)

	for name, prior := range map[string]string{"another account's": others, "never issued": "crafted-token"} {
		t.Run(name, func(t *testing.T) {
			token := requestLink(t, db, "portal_prior-asker", requestBody(t, "asker@example.com", "", prior))
			status, got := redeemLink(t, db, token)
			if status != http.StatusOK {
				t.Fatalf("redeem status = %d, want %d", status, http.StatusOK)
			}
			if got != "" {
				t.Fatalf("returnTo = %q, want none", got)
			}
		})
	}
}

// #2534: a page she opened just now outranks the one an old link names.
func TestMagicLink_AFreshReturnToOutranksThePriorLink(t *testing.T) {
	db := testdb.New(t)
	const identifier = "portal_prior-fresh"
	testdb.SeedPortalAccount(t, db, identifier, "fresh@example.com")
	first := requestLink(t, db, identifier, requestBody(t, "fresh@example.com", priorBirthPlan, ""))
	expireLink(t, db, first)

	const invoices = "/portal/engagements/0d6c7c39-1f53-4b8e-9d63-9a2b3c4d5e6f/invoices"
	second := requestLink(t, db, identifier, requestBody(t, "fresh@example.com", invoices, first))
	if _, got := redeemLink(t, db, second); got != invoices {
		t.Fatalf("returnTo = %q, want %q", got, invoices)
	}
}
