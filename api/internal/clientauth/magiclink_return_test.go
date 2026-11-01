package clientauth_test

import (
	"encoding/json"
	"net/http"
	"testing"

	"doula-cloud/api/internal/testdb"
)

func pendingMagicLinkToken(t *testing.T, db *testdb.DB, identifier string) string {
	t.Helper()
	var token string
	if err := db.Admin.QueryRowContext(t.Context(),
		`SELECT token FROM portal_magic_link_outbox WHERE identity_uid = $1 AND status = 'pending'`, identifier,
	).Scan(&token); err != nil {
		t.Fatalf("read pending magic link token: %v", err)
	}
	return token
}

func countMagicLinkReturns(t *testing.T, db *testdb.DB, identifier string) int {
	t.Helper()
	var count int
	if err := db.Admin.QueryRowContext(t.Context(),
		`SELECT count(*) FROM portal_magic_link_returns WHERE identifier = $1`, identifier,
	).Scan(&count); err != nil {
		t.Fatalf("count portal_magic_link_returns: %v", err)
	}
	return count
}

func requestThenRedeem(t *testing.T, db *testdb.DB, identifier, requestBody string) (status int, returnTo string) {
	t.Helper()
	srv := newMagicLinkRequestServer(db)
	defer srv.Close()

	requested := postJSON(t, srv, "/api/portal/magic-link/request", requestBody)
	_ = requested.Body.Close()
	if requested.StatusCode != http.StatusAccepted {
		t.Fatalf("request status = %d, want %d", requested.StatusCode, http.StatusAccepted)
	}

	token := pendingMagicLinkToken(t, db, identifier)
	redeemed := postJSON(t, srv, "/api/portal/magic-link", `{"token":"`+token+`"}`)
	defer redeemed.Body.Close()
	var body struct {
		OK       bool    `json:"ok"`
		ReturnTo *string `json:"returnTo"`
	}
	if err := json.NewDecoder(redeemed.Body).Decode(&body); err != nil {
		t.Fatalf("decode redeem body: %v", err)
	}
	if body.ReturnTo == nil {
		return redeemed.StatusCode, ""
	}
	if *body.ReturnTo == "" {
		t.Fatal("returnTo is present and empty; want it left out when there is none")
	}
	return redeemed.StatusCode, *body.ReturnTo
}

// #2496: the page she asked for rides on the token, not on the link, and
// comes back on the redeem.
func TestMagicLink_ReturnToSurvivesTheMailedLink(t *testing.T) {
	cases := []struct{ name, returnTo string }{
		{"a Birth Plan", "/portal/engagements/0d6c7c39-1f53-4b8e-9d63-9a2b3c4d5e6f/birth-plan"},
		{"with its query", "/portal/engagements/0d6c7c39-1f53-4b8e-9d63-9a2b3c4d5e6f/invoices?status=open"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			db := testdb.New(t)
			const identifier = "portal_return-kept"
			testdb.SeedPortalAccount(t, db, identifier, "kept@example.com")

			status, got := requestThenRedeem(t, db, identifier, `{"email":"kept@example.com","returnTo":"`+tc.returnTo+`"}`)
			if status != http.StatusOK {
				t.Fatalf("redeem status = %d, want %d", status, http.StatusOK)
			}
			if got != tc.returnTo {
				t.Fatalf("returnTo = %q, want %q", got, tc.returnTo)
			}
		})
	}
}

// #2496: anything but a path under /portal/engagements/ on this app is
// dropped at the request, so a crafted link cannot send her off-site.
// She still gets her link, and lands where the portal always lands her.
func TestMagicLink_ReturnToOffThePortalIsDropped(t *testing.T) {
	cases := []string{
		"https://evil.example/portal/engagements/x",
		"//evil.example/portal/engagements/x",
		`/\\evil.example/portal/engagements/x`,
		"/portal/engagements//evil.example",
		"/portal/engagements/../../practices/x",
		"/portal/login",
		"/practices/x/engagements/y",
		"javascript:alert(1)",
		"portal/engagements/x",
		"/portal/engagements/x\ty",
		"/portal/engagements/x?next=https://evil.example",
		"/portal/engagements/" + string(make([]byte, 600)),
	}
	for _, returnTo := range cases {
		t.Run(returnTo, func(t *testing.T) {
			db := testdb.New(t)
			const identifier = "portal_return-dropped"
			testdb.SeedPortalAccount(t, db, identifier, "dropped@example.com")

			encoded, err := json.Marshal(returnTo)
			if err != nil {
				t.Fatalf("marshal: %v", err)
			}
			status, got := requestThenRedeem(t, db, identifier, `{"email":"dropped@example.com","returnTo":`+string(encoded)+`}`)
			if status != http.StatusOK {
				t.Fatalf("redeem status = %d, want %d", status, http.StatusOK)
			}
			if got != "" {
				t.Fatalf("returnTo = %q, want none", got)
			}
		})
	}
}

// #2496: a second request with no page retires the first one's, since
// Mint deletes the prior token and the row cascades with it.
func TestMagicLink_ReRequestRetiresThePriorReturn(t *testing.T) {
	db := testdb.New(t)
	const identifier = "portal_return-rerequest"
	testdb.SeedPortalAccount(t, db, identifier, "again@example.com")
	srv := newMagicLinkRequestServer(db)
	defer srv.Close()

	first := postJSON(t, srv, "/api/portal/magic-link/request", `{"email":"again@example.com","returnTo":"/portal/engagements/a/birth-plan"}`)
	_ = first.Body.Close()
	if countMagicLinkReturns(t, db, identifier) != 1 {
		t.Fatal("expected the first request's return address to be stored")
	}

	status, got := requestThenRedeem(t, db, identifier, `{"email":"again@example.com"}`)
	if status != http.StatusOK {
		t.Fatalf("redeem status = %d, want %d", status, http.StatusOK)
	}
	if got != "" {
		t.Fatalf("returnTo = %q, want none after a re-request without one", got)
	}
	if countMagicLinkReturns(t, db, identifier) != 0 {
		t.Fatal("expected the retired token's return address to go with it")
	}
}

// 00141's CHECK is the database's own copy of the rule, so a write that
// skipped the handler still cannot store an off-site address.
func TestMagicLinkReturns_CheckRefusesAnOffSiteAddress(t *testing.T) {
	db := testdb.New(t)
	const identifier = "portal_return-check"
	testdb.SeedPortalAccount(t, db, identifier, "check@example.com")
	if _, err := db.Admin.ExecContext(t.Context(),
		`INSERT INTO auth_tokens (token_hash, purpose, identity_uid, expires_at) VALUES ('h', 'client_magic_link', $1, now() + interval '1 minute')`,
		identifier,
	); err != nil {
		t.Fatalf("seed auth_tokens: %v", err)
	}
	for _, returnTo := range []string{"//evil.example", "https://evil.example", "/practices/x", `/portal/engagements/\evil`} {
		if _, err := db.Admin.ExecContext(t.Context(),
			`INSERT INTO portal_magic_link_returns (token_hash, identifier, return_to) VALUES ('h', $1, $2)`,
			identifier, returnTo,
		); err == nil {
			t.Fatalf("stored %q, want the CHECK to refuse it", returnTo)
		}
	}
}
