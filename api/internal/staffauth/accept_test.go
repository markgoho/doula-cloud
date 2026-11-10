package staffauth_test

import (
	"bytes"
	"context"
	"encoding/json/v2"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"doula-cloud/api/internal/apierr"
	"doula-cloud/api/internal/apierrtest"
	"doula-cloud/api/internal/authntest"
	"doula-cloud/api/internal/fixture"
	"doula-cloud/api/internal/reqtest"
	"doula-cloud/api/internal/staffauth"
	"doula-cloud/api/internal/testdb"
)

// seedInvitationWithToken seeds a pending Invitation and returns the
// plaintext token to accept it with -- the token only ever exists in the
// outbox row in production, so a test mints its own and stores the
// matching digest.
func seedInvitationWithToken(t *testing.T, db *testdb.DB, practiceID, invitedBy, address, roles, employmentType string, expiresAt time.Time) (invitationID, token string) {
	t.Helper()
	token = "token-for-" + address
	if err := db.Admin.QueryRowContext(t.Context(),
		`INSERT INTO practice_invitations
		     (practice_id, address, roles, employment_type, token_digest, invited_by, expires_at)
		 VALUES ($1, $2, $3::practice_role[], $4::employment_type, $5, $6, $7) RETURNING id`,
		practiceID, address, roles, employmentType, staffauth.TokenDigest(token), invitedBy, expiresAt,
	).Scan(&invitationID); err != nil {
		t.Fatalf("seed invitation for %q: %v", address, err)
	}
	return invitationID, token
}

func postAccept(t *testing.T, srv *httptest.Server, body staffauth.AcceptInviteRequest) *http.Response {
	t.Helper()
	return reqtest.Post(t, srv.URL+"/api/staff/accept-invite", acceptBody(body)...)
}

func newAcceptServer(t *testing.T, db *testdb.DB, uid, email string) *httptest.Server {
	t.Helper()
	// The caller's Identity Platform account already exists with this
	// uid/email -- the same fact authntest.Verifier{UID, Email} asserts
	// for VerifyIDToken -- so accounts.SetEmailVerified (#613) has
	// something to flip.
	accounts := authntest.NewFakeAccountManager()
	accounts.Seed(uid, email, false)
	return newAcceptServerWithAccounts(t, db, uid, email, accounts)
}

func newAcceptServerWithAccounts(t *testing.T, db *testdb.DB, uid, email string, accounts *authntest.FakeAccountManager) *httptest.Server {
	t.Helper()
	return serveStaffauth(db, authntest.Verifier{UID: uid, Email: email}, accounts)
}

// newInviteeEnv is a Practice whose Owner is at the screen as the inviter,
// with a Bearer token that verifies as uid at email, whose Identity
// Platform account already exists unverified -- the same fact
// authntest.Verifier{UID, Email} asserts for VerifyIDToken -- so
// accounts.SetEmailVerified (#613) has something to flip.
func newInviteeEnv(t *testing.T, uid, email string) (*env, *authntest.FakeAccountManager) {
	t.Helper()
	accounts := authntest.NewFakeAccountManager()
	accounts.Seed(uid, email, false)
	e := newEnv(t, signedInAs(fixture.Owner()), withVerifier(authntest.Verifier{UID: uid, Email: email}), withAccounts(accounts))
	return e, accounts
}

// invite seeds a pending Invitation to address from the Owner at the
// screen, and returns the token to accept it with.
func (e *env) invite(address, roles, employmentType string, expiresAt time.Time) (invitationID, token string) {
	e.T.Helper()
	return seedInvitationWithToken(e.T, e.DB, e.Practice.ID, e.Staff.ID, address, roles, employmentType, expiresAt)
}

// inviteLive is invite for a Doula whose Invitation lapses in an hour.
func (e *env) inviteLive(address, employmentType string) (invitationID, token string) {
	e.T.Helper()
	return e.invite(address, "{doula}", employmentType, time.Now().Add(time.Hour))
}

// acceptBody is the acceptance's body and Bearer token. A string is sent
// as it is written, for a body that is not JSON.
func acceptBody(body any) []reqtest.Option {
	payload := reqtest.JSON(body)
	if raw, ok := body.(string); ok {
		payload = reqtest.RawJSON([]byte(raw))
	}
	return []reqtest.Option{payload, reqtest.Header("Authorization", "Bearer any-token")}
}

// acceptResponse posts the acceptance and leaves the response to the
// caller, for a test that reads its cookies.
func (e *env) acceptResponse(body any) *http.Response {
	e.T.Helper()
	return reqtest.Do(e.T, http.MethodPost, e.URL("/api/staff/accept-invite"), acceptBody(body)...)
}

// accept posts the acceptance and requires the 200.
func (e *env) accept(body staffauth.AcceptInviteRequest) staffauth.AcceptInviteResponse {
	e.T.Helper()
	return reqtest.Expect[staffauth.AcceptInviteResponse](e.T, http.MethodPost, e.URL("/api/staff/accept-invite"), http.StatusOK, acceptBody(body)...)
}

// refuseAccept posts the acceptance, requires want and returns the
// refusal's envelope.
func (e *env) refuseAccept(body any, want int) apierr.APIError {
	e.T.Helper()
	return apierrtest.Expect(e.T, http.MethodPost, e.URL("/api/staff/accept-invite"), want, acceptBody(body)...)
}

// TestAcceptInviteHandler_CreatesStaffAndMembership is the accept half of
// #226: the staff row appears here, when someone proves she controls the
// invited address -- not at invite time.
func TestAcceptInviteHandler_CreatesStaffAndMembership(t *testing.T) {
	e, _ := newInviteeEnv(t, "lena-uid", "Lena@Example.com")
	invitationID, token := e.inviteLive("lena@example.com", contractorType)

	resp := e.acceptResponse(staffauth.AcceptInviteRequest{WorkState: "NY", InviteToken: token, FirstName: leaFirstName, LastName: "Vasquez"})
	defer resp.Body.Close()
	reqtest.RequireStatus(t, resp, http.StatusOK)
	accepted := reqtest.Decode[staffauth.AcceptInviteResponse](t, resp)

	if accepted.PracticeID != e.Practice.ID {
		t.Fatalf("practiceId = %q, want %q", accepted.PracticeID, e.Practice.ID)
	}
	// #2239: what the Membership was written with, for the landing to say.
	if accepted.PracticeName != e.Practice.Name || len(accepted.Roles) != 1 || accepted.Roles[0] != doulaRole || accepted.EmploymentType != contractorType {
		t.Fatalf("accepted = %+v, want %q as doula/contractor", accepted, e.Practice.Name)
	}

	// The session cookie is minted on the accept response itself (#145),
	// so the invitee is signed in without a second exchange.
	var hasSession bool
	for _, c := range resp.Cookies() {
		if c.Name == "__session" && c.Value != "" {
			hasSession = true
		}
	}
	if !hasSession {
		t.Fatalf("cookies = %v, want a __session cookie", resp.Cookies())
	}

	var name, email string
	if err := e.DB.Admin.QueryRowContext(t.Context(),
		`SELECT name, email FROM staff WHERE id = $1`, accepted.StaffID,
	).Scan(&name, &email); err != nil {
		t.Fatalf("read staff row: %v", err)
	}
	// The address stored is the verified one, normalized -- not whatever
	// the caller typed, which is what a later invitation is matched on.
	if name != "Lena Vasquez" || email != "lena@example.com" {
		t.Fatalf("staff row = %q/%q", name, email)
	}

	var roles, employmentType string
	if err := e.DB.Admin.QueryRowContext(t.Context(),
		`SELECT array_to_string(roles, ','), employment_type::text FROM practice_memberships
		 WHERE practice_id = $1 AND staff_id = $2`, e.Practice.ID, accepted.StaffID,
	).Scan(&roles, &employmentType); err != nil {
		t.Fatalf("read membership: %v", err)
	}
	if roles != doulaRole || employmentType != contractorType {
		t.Fatalf("membership = %q/%q, want the Invitation's own (#266)", roles, employmentType)
	}

	var status, acceptedStaffID string
	var acceptedAt time.Time
	if err := e.DB.Admin.QueryRowContext(t.Context(),
		`SELECT status::text, accepted_staff_id, accepted_at FROM practice_invitations WHERE id = $1`, invitationID,
	).Scan(&status, &acceptedStaffID, &acceptedAt); err != nil {
		t.Fatalf("read invitation: %v", err)
	}
	if status != "accepted" || acceptedStaffID != accepted.StaffID || acceptedAt.IsZero() {
		t.Fatalf("invitation = %q/%q/%v, want acceptance recorded", status, acceptedStaffID, acceptedAt)
	}

	var eventType, actor string
	if err := e.DB.Admin.QueryRowContext(t.Context(),
		`SELECT action, actor_staff_id FROM activity
		 WHERE practice_id = $1 AND subject_kind = 'membership' AND subject_id = $2`, e.Practice.ID, accepted.StaffID,
	).Scan(&eventType, &actor); err != nil {
		t.Fatalf("read membership event: %v", err)
	}
	if eventType != "joined" || actor != accepted.StaffID {
		t.Fatalf("membership event = %q by %q, want joined by the accepter", eventType, actor)
	}
}

// TestAcceptInviteHandler_ResolvesAnExistingStaffRow covers the other
// half of #226: someone who already works at another Practice joins this
// one on the staff row she already has, not a second one.
func TestAcceptInviteHandler_ResolvesAnExistingStaffRow(t *testing.T) {
	e, _ := newInviteeEnv(t, "known-uid", "known@example.com")
	_, token := e.invite("known@example.com", "{admin}", employeeType, time.Now().Add(time.Hour))
	existingID := seedStaffWithEmail(t, e.DB, "known-uid", "known@example.com")
	seedMembership(t, e.DB, fixture.Practice(t, e.DB, fixture.PracticeName("Her Other Practice")).ID, existingID)

	accepted := e.accept(staffauth.AcceptInviteRequest{WorkState: "NY", InviteToken: token})

	if accepted.StaffID != existingID {
		t.Fatalf("staffId = %q, want the existing staff row %q", accepted.StaffID, existingID)
	}
	staffRows := testdb.Scalar[int](t, e.DB.Admin, `SELECT count(*) FROM staff WHERE lower(email) = 'known@example.com'`)
	if staffRows != 1 {
		t.Fatalf("staff rows on that address = %d, want 1 (#291)", staffRows)
	}
}

// TestAcceptInviteHandler_RefusesAMemberTwice is the AC's named status
// (the old behavior was a 500 from an unhandled unique violation), and
// the same conflict reached the way staff.email's lack of a unique
// constraint allows: a second identity provider, a second staff row, one
// address. Resolving by identity alone would let this through and put two
// rows for one person on the roster -- LV-G8 (#291) exactly.
func TestAcceptInviteHandler_RefusesAMemberTwice(t *testing.T) {
	cases := []struct {
		name       string
		memberUID  string
		inviteeUID string
	}{
		{"the same identity", "member-uid", "member-uid"},
		{"a second identity on one address", "twice-uid-a", "twice-uid-b"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			e, _ := newInviteeEnv(t, tc.inviteeUID, "member@example.com")
			_, token := e.inviteLive("member@example.com", employeeType)
			seedMembership(t, e.DB, e.Practice.ID, seedStaffWithEmail(t, e.DB, tc.memberUID, "member@example.com"))

			e.refuseAccept(staffauth.AcceptInviteRequest{WorkState: "NY", InviteToken: token, FirstName: "Twice", LastName: "Over"}, http.StatusConflict)

			memberships := testdb.Scalar[int](t, e.DB.Admin, `SELECT count(*) FROM practice_memberships pm JOIN staff s ON s.id = pm.staff_id
				 WHERE pm.practice_id = $1 AND lower(s.email) = 'member@example.com'`, e.Practice.ID)
			if memberships != 1 {
				t.Fatalf("memberships on that address = %d, want 1", memberships)
			}
		})
	}
}

// TestAcceptInviteHandler_Refuses covers each way an acceptance is turned
// away: an address that is not the invited one, a token with no verified
// address behind it, an Invitation that lapsed (left pending in the
// column: expiry is derived, so the Owner's Staff screen goes on showing
// it as expired, the same answer this link got), a token nobody holds,
// one that was revoked, and one already accepted.
func TestAcceptInviteHandler_Refuses(t *testing.T) {
	const invited = "invited@example.com"
	cases := []struct {
		name       string
		email      string
		expiresIn  time.Duration
		settle     string
		unknown    bool
		want       int
		code       apierr.Code
		wantStatus string
	}{
		{name: "the wrong address", email: "someone.else@example.com", expiresIn: time.Hour, want: http.StatusForbidden},
		{name: "no verified address", expiresIn: time.Hour, want: http.StatusForbidden},
		{name: "an Invitation that lapsed", email: invited, expiresIn: -time.Hour,
			want: http.StatusGone, code: apierr.CodeLinkUnusable, wantStatus: "pending"},
		{name: "a token nobody holds", email: invited, unknown: true, want: http.StatusNotFound, code: apierr.CodeLinkUnusable},
		{name: "a revoked Invitation", email: invited, expiresIn: time.Hour,
			settle: `UPDATE practice_invitations SET status = 'revoked', revoked_by = $2, revoked_at = now() WHERE id = $1`,
			want:   http.StatusNotFound, code: apierr.CodeLinkUnusable, wantStatus: "revoked"},
		{name: "an accepted Invitation", email: invited, expiresIn: time.Hour,
			settle: `UPDATE practice_invitations SET status = 'accepted', accepted_staff_id = $2, accepted_at = now() WHERE id = $1`,
			want:   http.StatusNotFound, code: apierr.CodeLinkUnusable, wantStatus: "accepted"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			e, _ := newInviteeEnv(t, "invitee-uid", tc.email)
			invitationID, token := e.invite(invited, "{doula}", employeeType, time.Now().Add(tc.expiresIn))
			if tc.unknown {
				token = "not-a-real-token"
			}
			if tc.settle != "" {
				e.Exec(tc.settle, invitationID, e.Staff.ID)
			}

			out := e.refuseAccept(staffauth.AcceptInviteRequest{WorkState: "NY", InviteToken: token, FirstName: "Too", LastName: "Late"}, tc.want)

			if tc.code != "" && out.Code != tc.code {
				t.Fatalf("code = %q, want %q", out.Code, tc.code)
			}
			if tc.wantStatus != "" {
				if status := testdb.Scalar[string](t, e.DB.Admin, `SELECT status::text FROM practice_invitations WHERE id = $1`, invitationID); status != tc.wantStatus {
					t.Fatalf("status = %q, want %s", status, tc.wantStatus)
				}
			}
		})
	}
}

// TestAcceptInviteHandler_LosesToAWithdrawalThatCommitsFirst is the race
// the unguarded write lost: accept reads the Invitation as pending, an
// Owner's withdrawal commits before accept writes, and accept then wrote
// 'accepted' over the revoked row by id alone -- an accepted Invitation
// still carrying revoked_by and revoked_at, and a Membership the Owner had
// just refused. The test holds the row's lock itself so the withdrawal
// lands in exactly that gap: the request reads, reaches its write and
// waits on the lock, then the withdrawal commits under it.
func TestAcceptInviteHandler_LosesToAWithdrawalThatCommitsFirst(t *testing.T) {
	e, _ := newInviteeEnv(t, "racer-uid", "racer@example.com")
	invitationID, token := e.inviteLive("racer@example.com", employeeType)

	withdrawal, err := e.DB.Admin.BeginTx(t.Context(), nil)
	if err != nil {
		t.Fatalf("begin withdrawal: %v", err)
	}
	defer func() { _ = withdrawal.Rollback() }()
	if _, err := withdrawal.ExecContext(t.Context(), `SELECT 1 FROM practice_invitations WHERE id = $1 FOR UPDATE`, invitationID); err != nil {
		t.Fatalf("lock invitation: %v", err)
	}

	status := make(chan int, 1)
	go func() {
		status <- postWithoutT(e.URL("/api/staff/accept-invite"), staffauth.AcceptInviteRequest{
			InviteToken: token, FirstName: "Rae", LastName: "Cer", WorkState: "NY",
		})
	}()
	awaitAWaitingLock(t, e.DB)

	if _, err := withdrawal.ExecContext(t.Context(),
		`UPDATE practice_invitations SET status = 'revoked', revoked_by = $2, revoked_at = now() WHERE id = $1`,
		invitationID, e.Staff.ID,
	); err != nil {
		t.Fatalf("withdraw: %v", err)
	}
	if err := withdrawal.Commit(); err != nil {
		t.Fatalf("commit withdrawal: %v", err)
	}

	if got := <-status; got != http.StatusNotFound {
		t.Fatalf("accept status = %d, want %d -- the withdrawal committed first", got, http.StatusNotFound)
	}
	var rowStatus string
	var acceptedStaffID *string
	if err := e.DB.Admin.QueryRowContext(t.Context(),
		`SELECT status::text, accepted_staff_id FROM practice_invitations WHERE id = $1`, invitationID,
	).Scan(&rowStatus, &acceptedStaffID); err != nil {
		t.Fatalf("read invitation: %v", err)
	}
	if rowStatus != "revoked" || acceptedStaffID != nil {
		t.Fatalf("invitation = %q accepted by %v, want it left revoked", rowStatus, acceptedStaffID)
	}
	members := testdb.Scalar[int](t, e.DB.Admin, `SELECT count(*) FROM staff WHERE lower(email) = 'racer@example.com'`)
	if members != 0 {
		t.Fatalf("staff rows for the racer = %d, want 0 -- the refused acceptance rolls back whole", members)
	}
}

// TestAcceptInviteHandler_ALapsedLinkLeavesTheStaffScreenAsItWas is the
// disagreement the stored 'expired' made: opening a lapsed link used to
// flip the row, which took it off the Owner's Staff screen, while a lapsed
// Invitation nobody had opened stayed on it marked expired. The same
// Invitation now reads the same on the Staff screen whether or not the
// link was opened.
func TestAcceptInviteHandler_ALapsedLinkLeavesTheStaffScreenAsItWas(t *testing.T) {
	e, _ := newInviteeEnv(t, "opener-uid", "opener@example.com")
	openedID, token := e.invite("opener@example.com", "{doula}", employeeType, time.Now().Add(-time.Hour))
	unopenedID, _ := e.invite("unopened@example.com", "{doula}", employeeType, time.Now().Add(-time.Hour))

	e.refuseAccept(staffauth.AcceptInviteRequest{WorkState: "NY", InviteToken: token, FirstName: "Open", LastName: "Er"}, http.StatusGone)

	listed := map[string]bool{}
	for _, inv := range e.roster("").Invitations.Items {
		listed[inv.InvitationID] = inv.Expired
	}
	for name, id := range map[string]string{"opened": openedID, "unopened": unopenedID} {
		if expired, ok := listed[id]; !ok || !expired {
			t.Errorf("%s lapsed Invitation listed=%v expired=%v, want listed and expired", name, ok, expired)
		}
	}
}

// postWithoutT posts body as an accept-invite request and returns the
// status, or 0 if the request could not be made -- for a goroutine, where
// t.Fatalf is not safe.
func postWithoutT(url string, body staffauth.AcceptInviteRequest) int {
	payload, err := json.Marshal(body)
	if err != nil {
		return 0
	}
	req, err := http.NewRequestWithContext(context.Background(), http.MethodPost, url, bytes.NewReader(payload))
	if err != nil {
		return 0
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer any-token")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return 0
	}
	_ = resp.Body.Close()
	return resp.StatusCode
}

// awaitAWaitingLock returns once some backend is blocked on a lock: the
// request under test has reached its write and is waiting for the row.
func awaitAWaitingLock(t *testing.T, db *testdb.DB) {
	t.Helper()
	for range 500 {
		if testdb.Scalar[bool](t, db.Admin, `SELECT EXISTS (SELECT 1 FROM pg_locks WHERE NOT granted)`) {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("the accept request never reached the locked Invitation row")
}

func TestAcceptInviteHandler_BadRequests(t *testing.T) {
	e, _ := newInviteeEnv(t, "nameless-uid", "nameless@example.com")
	_, token := e.inviteLive("nameless@example.com", employeeType)

	cases := []struct {
		name string
		body any
	}{
		{"invalid body", notJSON},
		{"missing token", staffauth.AcceptInviteRequest{WorkState: "NY", FirstName: "Nameless", LastName: placeholderLastName}},
		// A new person must name herself: practice_invitations carries no
		// name column, so there is nothing to fall back to.
		{"missing name for a new person", staffauth.AcceptInviteRequest{WorkState: "NY", InviteToken: token}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(*testing.T) {
			e.refuseAccept(tc.body, http.StatusBadRequest)
		})
	}
}

func TestAcceptInviteHandler_NoCredential(t *testing.T) {
	e := newEnv(t)

	reqtest.ExpectStatus(t, http.MethodPost, e.URL("/api/staff/accept-invite"), http.StatusUnauthorized, reqtest.JSON(staffauth.AcceptInviteRequest{WorkState: "NY", InviteToken: "whatever"}))
}

// TestAcceptInviteHandler_BackfillsOfferStaffID is ADR-0008's
// email-target Offer path closing (#317): an Offer mailed to an address
// names the Invitation, not a person, because no person existed to name.
// Accepting the Invitation is the moment one does, so every Offer on it
// gets the staff id it was always going to have -- past Offers included,
// since they are part of the history she reads.
func TestAcceptInviteHandler_BackfillsOfferStaffID(t *testing.T) {
	e, _ := newInviteeEnv(t, "renata-uid", "renata@example.com")
	invitationID, token := e.inviteLive("renata@example.com", contractorType)
	engagement := e.AnotherEngagement()
	offerID := testdb.Scalar[string](t, e.DB.Admin, `INSERT INTO engagement_offers
		     (engagement_id, invitation_id, employment_type, amount_cents, client_first_initial, client_area, due_date, offered_by, expires_at)
		 VALUES ($1, $2, 'contractor', 52000, 'R', 'North side', now() + interval '90 days', $3, now() + interval '7 days')
		 RETURNING id`, engagement.ID, invitationID, e.Staff.ID)

	accepted := e.accept(staffauth.AcceptInviteRequest{WorkState: "NY", InviteToken: token, FirstName: "Renata", LastName: "Alvarez"})

	offerStaffID := testdb.Scalar[*string](t, e.DB.Admin, `SELECT staff_id::text FROM engagement_offers WHERE id = $1`, offerID)
	if offerStaffID == nil || *offerStaffID != accepted.StaffID {
		t.Fatalf("offer staff_id = %v, want the just-accepted %q", offerStaffID, accepted.StaffID)
	}
}

// TestAcceptInviteHandler_SetsEmailVerifiedAndQueuesNoMail is #613/#169's
// AC: accepting an Invitation proves mailbox control directly, so it
// flips emailVerified via the Admin SDK and never queues a verification
// mail the way self-signup does.
func TestAcceptInviteHandler_SetsEmailVerifiedAndQueuesNoMail(t *testing.T) {
	const uid, email = "verified-by-accept-uid", "verified-by-accept@example.com"
	e, accounts := newInviteeEnv(t, uid, email)
	_, token := e.inviteLive(email, contractorType)

	e.accept(staffauth.AcceptInviteRequest{WorkState: "NY", InviteToken: token, FirstName: "Verified", LastName: "Person"})

	account, err := accounts.GetAccount(t.Context(), uid)
	if err != nil {
		t.Fatalf("GetAccount: %v", err)
	}
	if !account.EmailVerified {
		t.Fatal("EmailVerified = false, want true after accepting an invitation")
	}

	count := testdb.Scalar[int](t, e.DB.Admin, `SELECT count(*) FROM staff_token_mail_outbox WHERE identity_uid = $1`, uid)
	if count != 0 {
		t.Fatalf("token mail rows = %d, want 0 -- accepting an invitation must queue no verification mail", count)
	}
}

// TestAcceptInviteHandler_SetEmailVerifiedFailureRollsBackTheMembership
// proves a failed Admin SDK write refuses the acceptance outright, rather
// than joining the Practice behind an account Identity Platform still
// calls unverified.
func TestAcceptInviteHandler_SetEmailVerifiedFailureRollsBackTheMembership(t *testing.T) {
	const uid, email = "verify-fail-accept-uid", "verify-fail-accept@example.com"
	e, accounts := newInviteeEnv(t, uid, email)
	_, token := e.inviteLive(email, contractorType)
	accounts.Err = errors.New("admin sdk unreachable")

	e.refuseAccept(staffauth.AcceptInviteRequest{WorkState: "NY", InviteToken: token, FirstName: "Verify", LastName: "Fail Person"}, http.StatusInternalServerError)

	count := testdb.Scalar[int](t, e.DB.Admin, `SELECT count(*) FROM staff WHERE identity_uid = $1`, uid)
	if count != 0 {
		t.Fatal("expected the whole acceptance to roll back, including the new staff row")
	}
}
