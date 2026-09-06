package clientauth_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"doula-cloud/api/internal/authntest"
	"doula-cloud/api/internal/authtoken"
	"doula-cloud/api/internal/clientauth"
	"doula-cloud/api/internal/portalaccount"
	"doula-cloud/api/internal/testdb"
)

// The two addresses every case in this file moves between.
const (
	oldSignInAddress = "old@example.com"
	newSignInAddress = "new@example.com"
)

func newAddressChangeServer(db *testdb.DB) *httptest.Server {
	mux := http.NewServeMux()
	mux.Handle("POST /portal/sign-in-address/request", clientauth.RequestAddressChangeHandler(db.App))
	mux.Handle("POST /portal/sign-in-address", clientauth.SpendAddressChangeHandler(db.App))
	return httptest.NewServer(mux)
}

// postAddressJSON is postJSON with a __session cookie attached, since
// the request half of #619 is the one authenticated by a live portal
// session.
func postAddressJSON(t *testing.T, srv *httptest.Server, path, session, body string) *http.Response {
	t.Helper()
	req, err := http.NewRequestWithContext(t.Context(), http.MethodPost, srv.URL+path, strings.NewReader(body))
	if err != nil {
		t.Fatalf("build request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	if session != "" {
		authntest.AddSessionCookie(req, session)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	return resp
}

// seedSignedInClient builds the whole fixture the change flow needs: a
// Practice, a Client with an Engagement, a Portal Account signing in at
// oldAddress, and a live session for it.
func seedSignedInClient(t *testing.T, db *testdb.DB, oldAddress string) (identifier, clientID, session string) {
	t.Helper()
	identifier = portalaccount.NewIdentifier()
	practiceID := seedPractice(t, db, "Address Change Practice")
	clientID, _ = seedClientEngagement(t, db, practiceID, "Test Client", "contact@example.com")
	testdb.SeedPortalAccount(t, db, identifier, oldAddress)
	if _, err := db.Admin.ExecContext(t.Context(),
		`INSERT INTO client_portal_users (identity_uid, client_id) VALUES ($1, $2)`, identifier, clientID,
	); err != nil {
		t.Fatalf("seed client_portal_users: %v", err)
	}
	return identifier, clientID, authntest.SeedSession(t, db.App, identifier)
}

func signInAddressOf(t *testing.T, db *testdb.DB, identifier string) string {
	t.Helper()
	var address string
	if err := db.Admin.QueryRowContext(t.Context(),
		`SELECT sign_in_address FROM portal_accounts WHERE identifier = $1`, identifier,
	).Scan(&address); err != nil {
		t.Fatalf("read sign_in_address: %v", err)
	}
	return address
}

// pendingAddressChangeToken reads the plaintext token off the pending
// outbox row -- the only place it exists, since auth_tokens holds a
// digest. Standing in for reading the mail.
func pendingAddressChangeToken(t *testing.T, db *testdb.DB, identifier string) (token, toAddress string) {
	t.Helper()
	if err := db.Admin.QueryRowContext(t.Context(),
		`SELECT token, to_address FROM portal_address_change_outbox WHERE identity_uid = $1 AND status = 'pending'`,
		identifier,
	).Scan(&token, &toAddress); err != nil {
		t.Fatalf("read pending address change mail: %v", err)
	}
	return token, toAddress
}

func TestRequestAddressChangeHandler_MailsTheNewAddressOnly(t *testing.T) {
	db := testdb.New(t)
	srv := newAddressChangeServer(db)
	defer srv.Close()

	identifier, _, session := seedSignedInClient(t, db, oldSignInAddress)

	resp := postAddressJSON(t, srv, "/portal/sign-in-address/request", session, `{"email":"New@Example.com"}`)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusAccepted {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusAccepted)
	}

	_, toAddress := pendingAddressChangeToken(t, db, identifier)
	if toAddress != newSignInAddress {
		t.Fatalf("to_address = %q, want the normalized new address", toAddress)
	}
	// The whole point of the flow: nothing has moved yet.
	if got := signInAddressOf(t, db, identifier); got != oldSignInAddress {
		t.Fatalf("sign_in_address = %q, want it unmoved until the link is spent", got)
	}
}

// TestRequestAddressChangeHandler_OldAddressStillSignsIn is the "old
// address keeps working" AC read literally: with a confirmation pending,
// a magic-link request at the old address still mints a sign-in token.
func TestRequestAddressChangeHandler_OldAddressStillSignsIn(t *testing.T) {
	db := testdb.New(t)
	srv := newAddressChangeServer(db)
	defer srv.Close()
	linkSrv := newMagicLinkRequestServer(db)
	defer linkSrv.Close()

	identifier, _, session := seedSignedInClient(t, db, oldSignInAddress)

	resp := postAddressJSON(t, srv, "/portal/sign-in-address/request", session, `{"email":"new@example.com"}`)
	_ = resp.Body.Close()

	oldResp := postJSON(t, linkSrv, "/portal/magic-link/request", `{"email":"old@example.com"}`)
	defer oldResp.Body.Close()
	if countLiveMagicLinkTokens(t, db, identifier) != 1 {
		t.Fatal("the old address stopped signing her in while the change was still pending")
	}

	// And the new address signs nobody in yet.
	newResp := postJSON(t, linkSrv, "/portal/magic-link/request", `{"email":"new@example.com"}`)
	defer newResp.Body.Close()
	if countPendingMagicLinkMail(t, db, identifier) != 1 {
		t.Fatal("the unproved new address minted a sign-in link of its own")
	}
}

// TestRequestAddressChangeHandler_AddressInUseAnswersIdentically is
// #168's account-enumeration rule on this endpoint: an address another
// Portal Account already signs in with gets byte-for-byte the response a
// free address gets.
func TestRequestAddressChangeHandler_AddressInUseAnswersIdentically(t *testing.T) {
	db := testdb.New(t)
	srv := newAddressChangeServer(db)
	defer srv.Close()

	_, _, freeSession := seedSignedInClient(t, db, "first@example.com")
	_, _, takenSession := seedSignedInClient(t, db, "second@example.com")
	testdb.SeedPortalAccount(t, db, portalaccount.NewIdentifier(), "taken@example.com")

	free := postAddressJSON(t, srv, "/portal/sign-in-address/request", freeSession, `{"email":"nobody@example.com"}`)
	defer free.Body.Close()
	taken := postAddressJSON(t, srv, "/portal/sign-in-address/request", takenSession, `{"email":"taken@example.com"}`)
	defer taken.Body.Close()

	if free.StatusCode != taken.StatusCode {
		t.Fatalf("status differs: free = %d, in use = %d", free.StatusCode, taken.StatusCode)
	}
	if free.ContentLength != taken.ContentLength {
		t.Fatalf("body length differs: free = %d, in use = %d", free.ContentLength, taken.ContentLength)
	}
}

// TestRequestAddressChangeHandler_ReRequestRetiresTheFirstAddress proves
// the cascade off token_hash: asking twice leaves exactly one live token,
// naming the second address, so a link delivered on a late retry cannot
// move her to the address she changed her mind about.
func TestRequestAddressChangeHandler_ReRequestRetiresTheFirstAddress(t *testing.T) {
	db := testdb.New(t)
	srv := newAddressChangeServer(db)
	defer srv.Close()

	identifier, _, session := seedSignedInClient(t, db, oldSignInAddress)

	first := postAddressJSON(t, srv, "/portal/sign-in-address/request", session, `{"email":"first@example.com"}`)
	_ = first.Body.Close()
	firstToken, _ := pendingAddressChangeToken(t, db, identifier)

	second := postAddressJSON(t, srv, "/portal/sign-in-address/request", session, `{"email":"second@example.com"}`)
	_ = second.Body.Close()

	var live int
	if err := db.Admin.QueryRowContext(t.Context(),
		`SELECT count(*) FROM auth_tokens WHERE identity_uid = $1 AND purpose = 'client_sign_in_address_change' AND used_at IS NULL`,
		identifier,
	).Scan(&live); err != nil {
		t.Fatalf("count auth_tokens: %v", err)
	}
	if live != 1 {
		t.Fatalf("live tokens = %d, want 1", live)
	}

	var stranded int
	if err := db.Admin.QueryRowContext(t.Context(),
		`SELECT count(*) FROM portal_sign_in_address_changes WHERE token_hash = $1`, authtoken.Digest(firstToken),
	).Scan(&stranded); err != nil {
		t.Fatalf("count portal_sign_in_address_changes: %v", err)
	}
	if stranded != 0 {
		t.Fatal("the superseded address survived the re-request")
	}

	_, toAddress := pendingAddressChangeToken(t, db, identifier)
	if toAddress != "second@example.com" {
		t.Fatalf("to_address = %q, want the second address", toAddress)
	}
}

func TestRequestAddressChangeHandler_Refusals(t *testing.T) {
	db := testdb.New(t)
	srv := newAddressChangeServer(db)
	defer srv.Close()

	_, _, session := seedSignedInClient(t, db, oldSignInAddress)

	cases := []struct {
		name    string
		session string
		body    string
		want    int
	}{
		{"no session", "", `{"email":"new@example.com"}`, http.StatusUnauthorized},
		{"invalid body", session, `not json`, http.StatusBadRequest},
		{"empty address", session, `{"email":"  "}`, http.StatusBadRequest},
		{"malformed address", session, `{"email":"new.example.com"}`, http.StatusBadRequest},
		{"address is only an at sign", session, `{"email":"@"}`, http.StatusBadRequest},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			resp := postAddressJSON(t, srv, "/portal/sign-in-address/request", tc.session, tc.body)
			defer resp.Body.Close()
			if resp.StatusCode != tc.want {
				t.Fatalf("status = %d, want %d", resp.StatusCode, tc.want)
			}
		})
	}
}

// TestRequestAddressChangeHandler_StaffSessionRefused covers the one
// cookie, two populations trap (ADR-0026): a Staff session is a real
// session naming no Portal Account, and it must not reach this door.
func TestRequestAddressChangeHandler_StaffSessionRefused(t *testing.T) {
	db := testdb.New(t)
	srv := newAddressChangeServer(db)
	defer srv.Close()

	staffSession := authntest.SeedSession(t, db.App, "identity-platform-uid")

	resp := postAddressJSON(t, srv, "/portal/sign-in-address/request", staffSession, `{"email":"new@example.com"}`)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusForbidden)
	}
}

func TestSpendAddressChangeHandler_MovesTheAddressAndRecordsIt(t *testing.T) {
	db := testdb.New(t)
	srv := newAddressChangeServer(db)
	defer srv.Close()

	identifier, clientID, session := seedSignedInClient(t, db, oldSignInAddress)
	req := postAddressJSON(t, srv, "/portal/sign-in-address/request", session, `{"email":"new@example.com"}`)
	_ = req.Body.Close()
	token, _ := pendingAddressChangeToken(t, db, identifier)

	// No session cookie: the link is read in the new mailbox, possibly on
	// a device she has never signed in on.
	resp := postAddressJSON(t, srv, "/portal/sign-in-address", "", `{"token":"`+token+`"}`)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}
	var body struct {
		SignInAddress string `json:"signInAddress"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	if body.SignInAddress != newSignInAddress {
		t.Fatalf("signInAddress = %q, want the new address", body.SignInAddress)
	}
	if got := signInAddressOf(t, db, identifier); got != newSignInAddress {
		t.Fatalf("sign_in_address = %q, want the new address", got)
	}

	// Spending mints no session -- it moves an address, it does not sign
	// anybody in.
	for _, c := range resp.Cookies() {
		if c.Name == "__session" {
			t.Fatal("the confirmation minted a session")
		}
	}

	var actorClient string
	if err := db.Admin.QueryRowContext(t.Context(),
		`SELECT actor_client_id FROM activity WHERE subject_kind = 'client' AND subject_id = $1 AND action = 'portal_sign_in_address_changed'`,
		clientID,
	).Scan(&actorClient); err != nil {
		t.Fatalf("read activity row: %v", err)
	}
	if actorClient != clientID {
		t.Fatalf("actor_client_id = %q, want %q", actorClient, clientID)
	}
}

// TestSpendAddressChangeHandler_OldAddressStopsSigningIn is the other
// half of the "until the new one is proved" AC: once proved, the old
// address is nobody's.
func TestSpendAddressChangeHandler_OldAddressStopsSigningIn(t *testing.T) {
	db := testdb.New(t)
	srv := newAddressChangeServer(db)
	defer srv.Close()
	linkSrv := newMagicLinkRequestServer(db)
	defer linkSrv.Close()

	identifier, _, session := seedSignedInClient(t, db, oldSignInAddress)
	req := postAddressJSON(t, srv, "/portal/sign-in-address/request", session, `{"email":"new@example.com"}`)
	_ = req.Body.Close()
	token, _ := pendingAddressChangeToken(t, db, identifier)
	spend := postAddressJSON(t, srv, "/portal/sign-in-address", "", `{"token":"`+token+`"}`)
	_ = spend.Body.Close()

	oldResp := postJSON(t, linkSrv, "/portal/magic-link/request", `{"email":"old@example.com"}`)
	defer oldResp.Body.Close()
	if oldResp.StatusCode != http.StatusAccepted {
		t.Fatalf("status = %d, want %d", oldResp.StatusCode, http.StatusAccepted)
	}
	if got := countLiveMagicLinkTokens(t, db, identifier); got != 0 {
		t.Fatalf("live sign-in tokens for the old address = %d, want 0", got)
	}

	newResp := postJSON(t, linkSrv, "/portal/magic-link/request", `{"email":"new@example.com"}`)
	defer newResp.Body.Close()
	if got := countLiveMagicLinkTokens(t, db, identifier); got != 1 {
		t.Fatalf("live sign-in tokens for the new address = %d, want 1", got)
	}
}

// TestSpendAddressChangeHandler_AddressTakenSinceTheLinkWasSent is the
// collision the request endpoint deliberately does not answer: another
// Portal Account claimed the address while the mail sat in her inbox.
func TestSpendAddressChangeHandler_AddressTakenSinceTheLinkWasSent(t *testing.T) {
	db := testdb.New(t)
	srv := newAddressChangeServer(db)
	defer srv.Close()

	identifier, _, session := seedSignedInClient(t, db, oldSignInAddress)
	req := postAddressJSON(t, srv, "/portal/sign-in-address/request", session, `{"email":"contested@example.com"}`)
	_ = req.Body.Close()
	token, _ := pendingAddressChangeToken(t, db, identifier)

	testdb.SeedPortalAccount(t, db, portalaccount.NewIdentifier(), "contested@example.com")

	resp := postAddressJSON(t, srv, "/portal/sign-in-address", "", `{"token":"`+token+`"}`)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusConflict {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusConflict)
	}
	if got := signInAddressOf(t, db, identifier); got != oldSignInAddress {
		t.Fatalf("sign_in_address = %q, want it unmoved", got)
	}

	// The whole transaction rolled back, token included, so the link she
	// holds still works once she picks another address.
	var used any
	if err := db.Admin.QueryRowContext(t.Context(),
		`SELECT used_at FROM auth_tokens WHERE token_hash = $1`, authtoken.Digest(token),
	).Scan(&used); err != nil {
		t.Fatalf("read auth_tokens: %v", err)
	}
	if used != nil {
		t.Fatal("the refused confirmation burned her link")
	}
}

func TestSpendAddressChangeHandler_Refusals(t *testing.T) {
	db := testdb.New(t)
	srv := newAddressChangeServer(db)
	defer srv.Close()

	identifier, _, session := seedSignedInClient(t, db, oldSignInAddress)
	req := postAddressJSON(t, srv, "/portal/sign-in-address/request", session, `{"email":"new@example.com"}`)
	_ = req.Body.Close()
	spent, _ := pendingAddressChangeToken(t, db, identifier)
	first := postAddressJSON(t, srv, "/portal/sign-in-address", "", `{"token":"`+spent+`"}`)
	_ = first.Body.Close()

	cases := []struct {
		name string
		body string
		want int
	}{
		{"invalid body", `not json`, http.StatusBadRequest},
		{"empty token", `{"token":"  "}`, http.StatusBadRequest},
		{"unknown token", `{"token":"never-minted"}`, http.StatusBadRequest},
		{"already spent", `{"token":"` + spent + `"}`, http.StatusBadRequest},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			resp := postAddressJSON(t, srv, "/portal/sign-in-address", "", tc.body)
			defer resp.Body.Close()
			if resp.StatusCode != tc.want {
				t.Fatalf("status = %d, want %d", resp.StatusCode, tc.want)
			}
		})
	}
}

// TestSpendAddressChangeHandler_MagicLinkTokenIsNotAConfirmation proves
// the purpose column does real work: a sign-in link cannot be posted here
// to move an address.
func TestSpendAddressChangeHandler_MagicLinkTokenIsNotAConfirmation(t *testing.T) {
	db := testdb.New(t)
	srv := newAddressChangeServer(db)
	defer srv.Close()
	linkSrv := newMagicLinkRequestServer(db)
	defer linkSrv.Close()

	identifier, _, _ := seedSignedInClient(t, db, oldSignInAddress)
	req := postJSON(t, linkSrv, "/portal/magic-link/request", `{"email":"old@example.com"}`)
	_ = req.Body.Close()

	var token string
	if err := db.Admin.QueryRowContext(t.Context(),
		`SELECT token FROM portal_magic_link_outbox WHERE identity_uid = $1`, identifier,
	).Scan(&token); err != nil {
		t.Fatalf("read magic link token: %v", err)
	}

	resp := postAddressJSON(t, srv, "/portal/sign-in-address", "", `{"token":"`+token+`"}`)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusBadRequest)
	}
}
