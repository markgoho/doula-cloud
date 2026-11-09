package portalinvite_test

import (
	"database/sql"
	"net/http"
	"strings"
	"testing"
	"time"

	"doula-cloud/api/internal/activity"
	"doula-cloud/api/internal/apierr"
	"doula-cloud/api/internal/fixture"
	"doula-cloud/api/internal/portalaccount"
	"doula-cloud/api/internal/portalinvite"
	"doula-cloud/api/internal/reqtest"
	"doula-cloud/api/internal/session"
	"doula-cloud/api/internal/testdb"
)

// sessionCookie returns the __session cookie from resp, or nil if none
// was set.
func sessionCookie(resp *http.Response) *http.Cookie {
	for _, c := range resp.Cookies() {
		if c.Name == session.CookieName {
			return c
		}
	}
	return nil
}

// postAccept claims token and returns the raw response, for a test that
// reads its cookie.
func postAccept(t *testing.T, s *fixture.Scene, token string) *http.Response {
	t.Helper()
	return reqtest.Post(t, s.URL("/api/portal/accept-invite"), reqtest.JSON(portalinvite.AcceptInviteRequest{InviteToken: token}))
}

// portalUserRow reads the portal user row an invitation names.
func portalUserRow(t *testing.T, db *testdb.DB, clientID string) (identityUID sql.NullString, storedToken sql.NullString) {
	t.Helper()
	if err := db.Admin.QueryRowContext(t.Context(), `SELECT identity_uid, invite_token::text FROM client_portal_users WHERE client_id = $1`, clientID).Scan(&identityUID, &storedToken); err != nil {
		t.Fatalf("query portal user row: %v", err)
	}
	return identityUID, storedToken
}

func TestAcceptInviteHandler_Refusals(t *testing.T) {
	cases := []struct {
		name string
		// body is sent raw when set; otherwise token is sent as the invite
		// token, or an expired invitation's own when expired.
		body    string
		token   string
		expired bool
		want    int
		code    apierr.Code
	}{
		{name: "a body that is not JSON", body: "not json", want: http.StatusBadRequest},
		{name: "a missing invite token", want: http.StatusBadRequest},
		{name: "an unknown token", token: unknownUUID, want: http.StatusNotFound, code: apierr.CodeLinkUnusable},
		{
			// #2025: a link cut short when it was copied carries part of a
			// token. That is an invite nobody holds, the same 404 as an
			// unknown one, not a 500 from Postgres refusing to read it as a
			// uuid.
			name: "a token that is not a uuid", token: "00000000-0000-00", want: http.StatusNotFound, code: apierr.CodeLinkUnusable,
		},
		{
			// #616: an invitation past its invite_token_expires_at is
			// refused with 410, and the pending row is left untouched (the
			// doula re-sends rather than this handler flipping any state --
			// client_portal_users has no status column to flip).
			name: "an expired invitation", expired: true, want: http.StatusGone, code: apierr.CodeLinkUnusable,
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			s := newAcceptScene(t)
			token, clientID := c.token, ""
			if c.expired {
				clientID, token = seedPendingPortalInviteExpiringAt(t, s.DB, time.Now().Add(-time.Minute))
			}
			var resp *http.Response
			if c.body != "" {
				resp = reqtest.Post(t, s.URL("/api/portal/accept-invite"), reqtest.RawJSON([]byte(c.body)))
			} else {
				resp = postAccept(t, s, token)
			}
			defer resp.Body.Close()

			reqtest.RequireStatus(t, resp, c.want)
			if c.code != "" {
				if got := reqtest.Decode[apierr.APIError](t, resp).Code; got != c.code {
					t.Errorf("code = %q, want %q", got, c.code)
				}
			}
			if c.expired {
				identityUID, storedToken := portalUserRow(t, s.DB, clientID)
				if identityUID.Valid {
					t.Errorf("expected identity_uid to remain unset for an expired invite, got %q", identityUID.String)
				}
				if storedToken.String != token {
					t.Errorf("expected invite_token to remain %q, got %q", token, storedToken.String)
				}
			}
		})
	}
}

func TestAcceptInviteHandler_Success(t *testing.T) {
	s := newAcceptScene(t)
	clientID, inviteToken := seedPendingPortalInvite(t, s.DB)

	resp := postAccept(t, s, inviteToken)
	defer resp.Body.Close()

	reqtest.RequireStatus(t, resp, http.StatusOK)
	if out := reqtest.Decode[portalinvite.AcceptInviteResponse](t, resp); out.ClientID != clientID {
		t.Fatalf("clientId = %q, want %q", out.ClientID, clientID)
	}
	identityUID, storedToken := portalUserRow(t, s.DB, clientID)
	// identity_uid is a Portal Account DoulaCloud mints itself (#616): with
	// #617 landed, there is no caller-presented Identity Platform uid left
	// to compare it against at all.
	if !strings.HasPrefix(identityUID.String, portalaccount.Prefix) {
		t.Fatalf("identity_uid = %q, want prefix %q", identityUID.String, portalaccount.Prefix)
	}
	if storedToken.Valid {
		t.Fatalf("expected invite_token cleared after accept, got %q", storedToken.String)
	}
	signInAddress := testdb.Scalar[string](t, s.DB.Admin, `SELECT sign_in_address FROM portal_accounts WHERE identifier = $1`, identityUID.String)
	if signInAddress != invitedAddress {
		t.Fatalf("sign_in_address = %q, want the invited Client's own contact address", signInAddress)
	}
	// #145: accept-invite sets the session cookie on its own response, same
	// name as the create-session endpoint's (#144) -- deliberately not
	// asserting on the cookie's value. Its attributes and lifetime are
	// authn.MintSession's own contract, proved once in authn's test suite
	// (#837's one session-mint seam) rather than re-proved at every caller.
	c := sessionCookie(resp)
	if c == nil {
		t.Fatal("no __session cookie set on successful accept")
	}
	if c.Value == "" {
		t.Fatal("cookie value is empty")
	}
}

// TestAcceptInviteHandler_SessionStoreFailure proves an accept that
// cannot create its session commits nothing: no cookie, and the invite
// left exactly as pending as it was before the request, so retrying it
// is safe (#145). Creating the session is the last thing the handler
// does before committing, which is what makes that ordering hold.
func TestAcceptInviteHandler_SessionStoreFailure(t *testing.T) {
	s := newAcceptScene(t)
	clientID, inviteToken := seedPendingPortalInvite(t, s.DB)
	s.Exec(`DROP TABLE sessions`)

	resp := postAccept(t, s, inviteToken)
	defer resp.Body.Close()

	reqtest.RequireStatus(t, resp, http.StatusInternalServerError)
	if c := sessionCookie(resp); c != nil {
		t.Fatalf("cookie set on session store failure: %+v", c)
	}
	identityUID, storedToken := portalUserRow(t, s.DB, clientID)
	if identityUID.Valid {
		t.Fatalf("expected identity_uid to remain unset after rollback, got %q", identityUID.String)
	}
	if storedToken.String != inviteToken {
		t.Fatalf("expected invite_token to remain %q after rollback, got %q", inviteToken, storedToken.String)
	}
}

func TestAcceptInviteHandler_TokenAlreadyClaimed(t *testing.T) {
	s := newAcceptScene(t)
	_, inviteToken := seedPendingPortalInvite(t, s.DB)

	acceptInvite(t, s, inviteToken, http.StatusOK)
	acceptInvite(t, s, inviteToken, http.StatusNotFound)
}

// TestAcceptInviteHandler_SignInAddressReused proves #309's scenario in
// its #616 shape: a person who already holds a Portal Account for a
// given sign-in address, accepting an invite at a *different* Practice,
// attaches to that existing Portal Account rather than being refused --
// ADR-0015's "a Portal Account reaches many Clients, at most one per
// Practice". Cased differently to prove the reuse lookup is
// case-insensitive, matching practice_invitations_one_pending's shape.
func TestAcceptInviteHandler_SignInAddressReused(t *testing.T) {
	s := newAcceptScene(t)
	testdb.SeedPortalAccount(t, s.DB, "portal_existing-account", "Invited@Example.com")
	clientID, inviteToken := seedPendingPortalInvite(t, s.DB)

	acceptInvite(t, s, inviteToken, http.StatusOK)

	identityUID := testdb.Scalar[string](t, s.DB.Admin, `SELECT identity_uid FROM client_portal_users WHERE client_id = $1`, clientID)
	if identityUID != "portal_existing-account" {
		t.Fatalf("identity_uid = %q, want the existing Portal Account reused, not a fresh mint", identityUID)
	}
	accountCount := testdb.Scalar[int](t, s.DB.Admin, `SELECT count(*) FROM portal_accounts WHERE lower(sign_in_address) = lower($1)`, invitedAddress)
	if accountCount != 1 {
		t.Fatalf("portal_accounts rows for this address = %d, want 1 (no second account minted)", accountCount)
	}
	// CLAUDE.md's audit-trail expectation: a reuse accept records its own
	// action, distinct from a fresh mint's, since no Portal Account came
	// into being here.
	action := testdb.Scalar[string](t, s.DB.Admin,
		`SELECT a.action FROM activity a JOIN engagements e ON e.id = a.subject_id
		  WHERE a.subject_kind = 'engagement' AND e.client_id = $1 ORDER BY a.created_at DESC LIMIT 1`, clientID)
	if action != "portal_account_linked" {
		t.Fatalf("activity action = %q, want %q", action, "portal_account_linked")
	}
}

// TestAcceptInviteHandler_SignInAddressReusedSamePracticeConflict proves
// ADR-0015's narrower rule: a Portal Account reaches at most one Client
// *per Practice*. A person who already reaches a Client at the same
// Practice this invite belongs to is refused, not silently merged into a
// second Client record there.
func TestAcceptInviteHandler_SignInAddressReusedSamePracticeConflict(t *testing.T) {
	s := newAcceptScene(t)
	testdb.SeedPortalAccount(t, s.DB, "portal_existing-account", invitedAddress)
	clientID, inviteToken := seedPendingPortalInvite(t, s.DB)
	practiceID := testdb.PracticeOfClient(t, s.DB, clientID)
	// The same Portal Account already reaches a different Client at this
	// same Practice -- e.g. a second, mistaken invite for the same person.
	otherClientID, _ := testdb.SeedNamedEngagement(t, s.DB, practiceID, "Other Client Record", "other@example.com")
	testdb.AttachPortalUser(t, s.DB, "portal_existing-account", otherClientID)

	resp := postAccept(t, s, inviteToken)
	defer resp.Body.Close()

	reqtest.RequireStatus(t, resp, http.StatusConflict)
	if identityUID, _ := portalUserRow(t, s.DB, clientID); identityUID.Valid {
		t.Fatalf("expected the pending row to remain unclaimed, got identity_uid %q", identityUID.String)
	}
	// #2279: the refusal says so in her words, names the Practice, and
	// carries its own code, so the page can offer the way on to her care.
	body := reqtest.Decode[apierr.APIError](t, resp)
	practiceName := testdb.Scalar[string](t, s.DB.Admin, `SELECT name FROM practices WHERE id = $1`, practiceID)
	if body.Code != apierr.CodePortalAccessAlreadyHere {
		t.Fatalf("code = %q, want %q", body.Code, apierr.CodePortalAccessAlreadyHere)
	}
	if want := "You can already see your care at " + practiceName + "."; body.Message != want {
		t.Fatalf("message = %q, want %q", body.Message, want)
	}
	// The refused accept is on the invited Engagement's ledger, by her,
	// so the Practice can see this invite will never be accepted.
	var action, actorKind string
	var actorClientID sql.NullString
	if err := s.DB.Admin.QueryRowContext(t.Context(),
		`SELECT a.action, a.actor_kind::text, a.actor_client_id::text FROM activity a
		   JOIN engagements e ON e.id = a.subject_id
		  WHERE a.subject_kind = 'engagement' AND e.client_id = $1
		  ORDER BY a.created_at DESC LIMIT 1`,
		clientID,
	).Scan(&action, &actorKind, &actorClientID); err != nil {
		t.Fatalf("query activity: %v", err)
	}
	if action != string(activity.ActionPortalInviteRefused) || actorKind != "client" || actorClientID.String != clientID {
		t.Fatalf("activity = (%q, %q, %q), want (%q, client, %q)", action, actorKind, actorClientID.String, activity.ActionPortalInviteRefused, clientID)
	}
	// #2522: and the refusal leaves the fact the invite's status reads.
	refused := testdb.Scalar[bool](t, s.DB.Admin, `SELECT access_refused_at IS NOT NULL FROM client_portal_users WHERE client_id = $1`, clientID)
	if !refused {
		t.Fatal("access_refused_at is unset, want the refused accept recorded on the row")
	}
}
