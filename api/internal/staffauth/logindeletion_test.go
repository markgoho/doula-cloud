package staffauth_test

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"doula-cloud/api/internal/activity"
	"doula-cloud/api/internal/authntest"
	"doula-cloud/api/internal/idempotency"
	"doula-cloud/api/internal/staffauth"
	"doula-cloud/api/internal/tasknudge"
	"doula-cloud/api/internal/testdb"
)

// newDeleteLoginServer mounts the real route table -- not a bare handler
// -- so every test here goes through the same rate limiter, gated router
// and session cookie a browser would.
func newDeleteLoginServer(t *testing.T, db *testdb.DB, accounts *authntest.FakeAccountManager) *httptest.Server {
	t.Helper()
	mux := http.NewServeMux()
	g := staffauth.NewGatedRouter(mux, db.App)
	ir := idempotency.NewRouter(g, db.App)
	staffauth.Mount(g, ir, db.App, authntest.Verifier{}, accounts, &tasknudge.FakeEnqueuer{}, neverSuppressed)
	return httptest.NewServer(mux)
}

// deleteLogin issues the request under test. confirmed controls the
// X-Confirmed backstop, which is the only header this endpoint reads.
func deleteLogin(t *testing.T, srv *httptest.Server, session string, confirmed bool) *http.Response {
	t.Helper()
	req, err := http.NewRequestWithContext(t.Context(), http.MethodDelete, srv.URL+"/api/staff/account", nil)
	if err != nil {
		t.Fatalf("build request: %v", err)
	}
	authntest.AddSessionCookie(req, session)
	if confirmed {
		req.Header.Set("X-Confirmed", "true")
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	return resp
}

// seedCoOwner gives practiceID a second Owner, so the person under test
// is no longer its last one.
func seedCoOwner(t *testing.T, db *testdb.DB, practiceID, identityUID string) (staffID string) {
	t.Helper()
	return testdb.SeedStaffAtPractice(t, db, practiceID, identityUID, []string{ownerRole}, employeeType)
}

// readStaffRow reads the redaction's four columns straight out of the
// database, past RLS -- the redacted row is unreachable through any
// live session by construction, so an admin read is the only way to
// assert on it.
func readStaffRow(t *testing.T, db *testdb.DB, staffID string) (name, email, identityUID string, deleted bool) {
	t.Helper()
	if err := db.Admin.QueryRowContext(t.Context(),
		`SELECT name, email, identity_uid, deleted_at IS NOT NULL FROM staff WHERE id = $1`, staffID,
	).Scan(&name, &email, &identityUID, &deleted); err != nil {
		t.Fatalf("read staff row: %v", err)
	}
	return name, email, identityUID, deleted
}

func countRows(t *testing.T, db *testdb.DB, query string, args ...any) int {
	t.Helper()
	var n int
	if err := db.Admin.QueryRowContext(t.Context(), query, args...).Scan(&n); err != nil {
		t.Fatalf("count: %v", err)
	}
	return n
}

// TestDeleteLoginHandler_RedactsRowAndDestroysAccount is the whole act on
// the simplest shape there is: a contractor doula at one Practice she does
// not own.
func TestDeleteLoginHandler_RedactsRowAndDestroysAccount(t *testing.T) {
	db := testdb.New(t)
	const uid = "doula-deletes-her-own-login"
	practiceID, staffID := testdb.SeedStaffAtNewPractice(t, db, uid, []string{doulaRole}, contractorType)
	seedCoOwner(t, db, practiceID, "owner-who-stays-put")

	accounts := authntest.NewFakeAccountManager()
	accounts.Seed(uid, "tasha@example.com", true)
	srv := newDeleteLoginServer(t, db, accounts)
	defer srv.Close()
	session := authntest.SeedSession(t, db.App, uid)

	resp := deleteLogin(t, srv, session, true)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusNoContent {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusNoContent)
	}

	name, email, identityUID, deleted := readStaffRow(t, db, staffID)
	if name != staffauth.DeletedStaffName {
		t.Fatalf("name = %q, want %q", name, staffauth.DeletedStaffName)
	}
	if email != staffauth.DeletedStaffEmail {
		t.Fatalf("email = %q, want %q", email, staffauth.DeletedStaffEmail)
	}
	if identityUID != "deleted:"+staffID {
		t.Fatalf("identity_uid = %q, want the sentinel derived from the row's own id", identityUID)
	}
	if !deleted {
		t.Fatal("deleted_at is not stamped, so nothing proves the act ran")
	}
	if accounts.Exists(uid) {
		t.Fatal("her Identity Platform account survived, so she can still authenticate")
	}
	if n := countRows(t, db, `SELECT count(*) FROM practice_memberships WHERE staff_id = $1`, staffID); n != 0 {
		t.Fatalf("memberships left = %d, want every one ended", n)
	}
	if n := countRows(t, db,
		`SELECT count(*) FROM staff_auth_events WHERE staff_id = $1 AND actor_staff_id = $1 AND reason = 'login_deleted'`,
		staffID); n != 1 {
		t.Fatalf("login_deleted events = %d, want the act recorded exactly once with her as her own actor", n)
	}
}

// TestDeleteLoginHandler_EndsEveryMembershipWithItsOwnEvent is the
// cross-Practice case the whole ticket exists for: a contractor doula at
// two Practices leaves both in one act, and each Practice's own Activity
// log carries the 'removed' event naming what the Membership held.
func TestDeleteLoginHandler_EndsEveryMembershipWithItsOwnEvent(t *testing.T) {
	db := testdb.New(t)
	const uid = "doula-at-two-practices"
	firstPractice, staffID := testdb.SeedStaffAtNewPractice(t, db, uid, []string{doulaRole}, contractorType)
	seedCoOwner(t, db, firstPractice, "owner-of-the-first")
	secondPractice := testdb.SeedPractice(t, db, "Second Practice")
	testdb.SeedStaffAtPractice(t, db, secondPractice, "owner-of-the-second", []string{ownerRole}, employeeType)
	if _, err := db.Admin.ExecContext(t.Context(),
		`INSERT INTO practice_memberships (practice_id, staff_id, roles, employment_type)
		 VALUES ($1, $2, '{doula,admin}', 'contractor')`,
		secondPractice, staffID,
	); err != nil {
		t.Fatalf("seed second membership: %v", err)
	}

	accounts := authntest.NewFakeAccountManager()
	accounts.Seed(uid, "tasha@example.com", true)
	srv := newDeleteLoginServer(t, db, accounts)
	defer srv.Close()

	resp := deleteLogin(t, srv, authntest.SeedSession(t, db.App, uid), true)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusNoContent {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusNoContent)
	}

	for _, practiceID := range []string{firstPractice, secondPractice} {
		if n := countRows(t, db,
			`SELECT count(*) FROM activity
			  WHERE practice_id = $1 AND subject_kind = 'membership' AND subject_id = $2
			    AND action = 'removed' AND actor_staff_id = $2`,
			practiceID, staffID); n != 1 {
			t.Fatalf("practice %s: 'removed' events = %d, want exactly one naming her as her own actor", practiceID, n)
		}
	}

	var diff string
	if err := db.Admin.QueryRowContext(t.Context(),
		`SELECT diff::text FROM activity
		  WHERE practice_id = $1 AND subject_id = $2 AND action = 'removed'`,
		secondPractice, staffID,
	).Scan(&diff); err != nil {
		t.Fatalf("read removal diff: %v", err)
	}
	if !strings.Contains(diff, "admin") || !strings.Contains(diff, contractorType) {
		t.Fatalf("removal diff = %s, want the roles and employment type the membership held", diff)
	}
}

// TestDeleteLoginHandler_EndsEverySession is the ordering the handler's
// own comment calls load-bearing: sessions is keyed on identity_uid, so a
// redaction that overwrote it first would leave her signed in forever on
// a live cookie.
func TestDeleteLoginHandler_EndsEverySession(t *testing.T) {
	db := testdb.New(t)
	const uid = "doula-signed-in-two-places"
	practiceID, _ := testdb.SeedStaffAtNewPractice(t, db, uid, []string{doulaRole}, employeeType)
	seedCoOwner(t, db, practiceID, "owner-of-the-only-practice")

	accounts := authntest.NewFakeAccountManager()
	accounts.Seed(uid, "tasha@example.com", true)
	srv := newDeleteLoginServer(t, db, accounts)
	defer srv.Close()
	otherBrowser := authntest.SeedSession(t, db.App, uid)
	session := authntest.SeedSession(t, db.App, uid)

	resp := deleteLogin(t, srv, session, true)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusNoContent {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusNoContent)
	}
	if got := authntest.CountFor(t, db.App, uid); got != 0 {
		t.Fatalf("session rows left = %d, want none anywhere", got)
	}

	// The token from the browser she was not holding must be dead too --
	// a live cookie is the whole of a session's credential.
	after := deleteLogin(t, srv, otherBrowser, true)
	defer after.Body.Close()
	if after.StatusCode != http.StatusUnauthorized {
		t.Fatalf("status on the other browser's live token = %d, want %d", after.StatusCode, http.StatusUnauthorized)
	}
}

// TestDeleteLoginHandler_RefusesWhileSoleOwner is the first of the two
// refusals the ticket names, and proves the refusal names the Practice
// rather than merely saying no.
func TestDeleteLoginHandler_RefusesWhileSoleOwner(t *testing.T) {
	db := testdb.New(t)
	const uid = "sole-owner-tries-to-leave"
	practiceID, staffID := testdb.SeedStaffAtNewPractice(t, db, uid, []string{ownerRole}, employeeType)
	if _, err := db.Admin.ExecContext(t.Context(),
		`UPDATE practices SET name = 'Rochester Birth Collective' WHERE id = $1`, practiceID,
	); err != nil {
		t.Fatalf("name the practice: %v", err)
	}

	accounts := authntest.NewFakeAccountManager()
	accounts.Seed(uid, "owner@example.com", true)
	srv := newDeleteLoginServer(t, db, accounts)
	defer srv.Close()

	resp := deleteLogin(t, srv, authntest.SeedSession(t, db.App, uid), true)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusConflict {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusConflict)
	}
	body, err := readBody(resp)
	if err != nil {
		t.Fatalf("read body: %v", err)
	}
	if !strings.Contains(body, "Rochester Birth Collective") {
		t.Fatalf("body = %s, want the practice in the way named", body)
	}
	if _, _, _, deleted := readStaffRow(t, db, staffID); deleted {
		t.Fatal("the refused request redacted her row anyway")
	}
	if !accounts.Exists(uid) {
		t.Fatal("the refused request destroyed her Identity Platform account anyway")
	}
}

// TestDeleteLoginHandler_RefusesWhileSoleOwnerOfPracticePendingDeletion is
// the second refusal: a Practice inside ADR-0031's 30-day window still
// needs an Owner, because letting her go would foreclose the restore that
// window exists to promise.
func TestDeleteLoginHandler_RefusesWhileSoleOwnerOfPracticePendingDeletion(t *testing.T) {
	db := testdb.New(t)
	const uid = "sole-owner-of-a-practice-winding-down"
	practiceID, staffID := testdb.SeedStaffAtNewPractice(t, db, uid, []string{ownerRole}, employeeType)
	if _, err := db.Admin.ExecContext(t.Context(),
		`UPDATE practices
		    SET name = 'Winding Down Doulas', deletion_requested_at = now(),
		        deletion_requested_by = $2, deletion_finalize_at = now() + interval '30 days'
		  WHERE id = $1`,
		practiceID, staffID,
	); err != nil {
		t.Fatalf("start the practice's deletion: %v", err)
	}

	accounts := authntest.NewFakeAccountManager()
	accounts.Seed(uid, "owner@example.com", true)
	srv := newDeleteLoginServer(t, db, accounts)
	defer srv.Close()

	resp := deleteLogin(t, srv, authntest.SeedSession(t, db.App, uid), true)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusConflict {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusConflict)
	}
	body, err := readBody(resp)
	if err != nil {
		t.Fatalf("read body: %v", err)
	}
	if !strings.Contains(body, "Winding Down Doulas") {
		t.Fatal("a practice pending deletion did not block, so its restore window has no Owner to restore into")
	}
}

// TestDeleteLoginHandler_AllowsSoleOwnerOfFinalizedPractice is the other
// side of that filter: once a Practice is finalized nobody can reach it,
// so being its Owner no longer means anything and must not trap her.
func TestDeleteLoginHandler_AllowsSoleOwnerOfFinalizedPractice(t *testing.T) {
	db := testdb.New(t)
	const uid = "sole-owner-of-a-finished-practice"
	practiceID, staffID := testdb.SeedStaffAtNewPractice(t, db, uid, []string{ownerRole}, employeeType)
	if _, err := db.Admin.ExecContext(t.Context(),
		`UPDATE practices SET deleted_at = now() WHERE id = $1`, practiceID,
	); err != nil {
		t.Fatalf("finalize the practice: %v", err)
	}

	accounts := authntest.NewFakeAccountManager()
	accounts.Seed(uid, "owner@example.com", true)
	srv := newDeleteLoginServer(t, db, accounts)
	defer srv.Close()

	resp := deleteLogin(t, srv, authntest.SeedSession(t, db.App, uid), true)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusNoContent {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusNoContent)
	}
	if _, _, _, deleted := readStaffRow(t, db, staffID); !deleted {
		t.Fatal("she was refused over a practice nobody can reach any more")
	}
}

// TestDeleteLoginHandler_AllowsOwnerWithACoOwner proves the refusal is
// about being the *last* Owner and not about holding the role -- the case
// that would otherwise make every Owner permanently undeletable.
func TestDeleteLoginHandler_AllowsOwnerWithACoOwner(t *testing.T) {
	db := testdb.New(t)
	const uid = "owner-with-a-co-owner"
	practiceID, staffID := testdb.SeedStaffAtNewPractice(t, db, uid, []string{ownerRole}, employeeType)
	coOwnerID := seedCoOwner(t, db, practiceID, "the-remaining-owner")

	accounts := authntest.NewFakeAccountManager()
	accounts.Seed(uid, "owner@example.com", true)
	srv := newDeleteLoginServer(t, db, accounts)
	defer srv.Close()

	resp := deleteLogin(t, srv, authntest.SeedSession(t, db.App, uid), true)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusNoContent {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusNoContent)
	}
	if _, _, _, deleted := readStaffRow(t, db, staffID); !deleted {
		t.Fatal("an Owner with a co-Owner was refused")
	}
	// #615's rule, inherited from RemoveMembershipHandler: the co-Owner is
	// newly sole, so she is newly entitled to saved recovery codes.
	if n := countRows(t, db,
		`SELECT count(*) FROM staff_mfa_recovery_codes
		  WHERE staff_id = $1 AND used_at IS NULL AND revoked_at IS NULL`, coOwnerID); n == 0 {
		t.Fatal("the remaining Owner became sole and was not issued saved codes")
	}
}

// TestDeleteLoginHandler_RequiresConfirmation is CLAUDE.md's hard block
// with a deliberate override, at the boundary that can enforce it: the
// frontend's dialog is not an enforcement point, the header is.
func TestDeleteLoginHandler_RequiresConfirmation(t *testing.T) {
	db := testdb.New(t)
	const uid = "doula-forgets-to-confirm"
	practiceID, staffID := testdb.SeedStaffAtNewPractice(t, db, uid, []string{doulaRole}, employeeType)
	seedCoOwner(t, db, practiceID, "owner-untouched-by-an-unconfirmed-request")

	accounts := authntest.NewFakeAccountManager()
	accounts.Seed(uid, "tasha@example.com", true)
	srv := newDeleteLoginServer(t, db, accounts)
	defer srv.Close()

	resp := deleteLogin(t, srv, authntest.SeedSession(t, db.App, uid), false)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusBadRequest)
	}
	if _, _, _, deleted := readStaffRow(t, db, staffID); deleted {
		t.Fatal("an unconfirmed request deleted her login")
	}
}

// TestDeleteLoginHandler_RefusesWithoutASession proves the endpoint acts
// only on the caller's own identity: there is no staff id anywhere in the
// route or the body, so with no credential there is nobody to act on.
func TestDeleteLoginHandler_RefusesWithoutASession(t *testing.T) {
	db := testdb.New(t)
	srv := newDeleteLoginServer(t, db, authntest.NewFakeAccountManager())
	defer srv.Close()

	req, err := http.NewRequestWithContext(t.Context(), http.MethodDelete, srv.URL+"/api/staff/account", nil)
	if err != nil {
		t.Fatalf("build request: %v", err)
	}
	req.Header.Set("X-Confirmed", "true")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusUnauthorized)
	}
}

// TestDeleteLoginHandler_RefusesASessionWithNoStaffRow covers the person
// who holds a live session minted before her staff row existed -- the
// same MsgNoMatchingStaffAccount shape RotateSavedCodesHandler answers
// with.
func TestDeleteLoginHandler_RefusesASessionWithNoStaffRow(t *testing.T) {
	db := testdb.New(t)
	srv := newDeleteLoginServer(t, db, authntest.NewFakeAccountManager())
	defer srv.Close()

	resp := deleteLogin(t, srv, authntest.SeedSession(t, db.App, "a-uid-with-no-staff-row"), true)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusNotFound)
	}
}

// TestDeleteLoginHandler_RefusesASecondDelete is the ticket's
// run-it-twice criterion. The second call presents a token that named the
// deleted row, and there is nothing left for it to name: her sessions are
// gone with the first act, so it never reaches the handler at all. That
// is the refusal, and it is stronger than a 409 -- there is no live
// credential in existence that can address a deleted login.
func TestDeleteLoginHandler_RefusesASecondDelete(t *testing.T) {
	db := testdb.New(t)
	const uid = "doula-deletes-twice"
	practiceID, staffID := testdb.SeedStaffAtNewPractice(t, db, uid, []string{doulaRole}, employeeType)
	seedCoOwner(t, db, practiceID, "owner-watching-a-double-delete")

	accounts := authntest.NewFakeAccountManager()
	accounts.Seed(uid, "tasha@example.com", true)
	srv := newDeleteLoginServer(t, db, accounts)
	defer srv.Close()
	session := authntest.SeedSession(t, db.App, uid)

	first := deleteLogin(t, srv, session, true)
	defer first.Body.Close()
	if first.StatusCode != http.StatusNoContent {
		t.Fatalf("first status = %d, want %d", first.StatusCode, http.StatusNoContent)
	}

	second := deleteLogin(t, srv, session, true)
	defer second.Body.Close()
	if second.StatusCode != http.StatusUnauthorized {
		t.Fatalf("second status = %d, want %d", second.StatusCode, http.StatusUnauthorized)
	}
	if n := countRows(t, db,
		`SELECT count(*) FROM staff_auth_events WHERE staff_id = $1 AND reason = 'login_deleted'`, staffID); n != 1 {
		t.Fatalf("login_deleted events = %d, want the act recorded once, not twice", n)
	}
}

// TestDeleteLoginHandler_IdentityPlatformFailureRollsEverythingBack is why
// the Admin SDK call sits inside the transaction rather than after the
// commit: a person must never be left with a redacted row and a live
// account, nor with an account destroyed and a row that still works.
func TestDeleteLoginHandler_IdentityPlatformFailureRollsEverythingBack(t *testing.T) {
	db := testdb.New(t)
	const uid = "doula-whose-idp-call-fails"
	practiceID, staffID := testdb.SeedStaffAtNewPractice(t, db, uid, []string{doulaRole}, employeeType)
	seedCoOwner(t, db, practiceID, "owner-untouched-by-a-failed-delete")

	accounts := authntest.NewFakeAccountManager()
	accounts.Seed(uid, "tasha@example.com", true)
	accounts.DeleteAccountErr = errors.New("identity platform unreachable")
	srv := newDeleteLoginServer(t, db, accounts)
	defer srv.Close()

	resp := deleteLogin(t, srv, authntest.SeedSession(t, db.App, uid), true)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusInternalServerError {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusInternalServerError)
	}
	if _, _, _, deleted := readStaffRow(t, db, staffID); deleted {
		t.Fatal("her row was redacted even though the account delete failed")
	}
	if n := countRows(t, db, `SELECT count(*) FROM practice_memberships WHERE staff_id = $1`, staffID); n != 1 {
		t.Fatalf("memberships = %d, want the failed act to have ended none", n)
	}
	if got := authntest.CountFor(t, db.App, uid); got != 1 {
		t.Fatalf("session rows = %d, want the failed act to have ended none", got)
	}
}

// TestDeleteLoginHandler_AbsentIdentityPlatformAccountIsSuccess is the
// idempotency the interface promises: an account Identity Platform
// already reports absent must not stop the rest of the act.
func TestDeleteLoginHandler_AbsentIdentityPlatformAccountIsSuccess(t *testing.T) {
	db := testdb.New(t)
	const uid = "doula-with-no-idp-account-left"
	practiceID, staffID := testdb.SeedStaffAtNewPractice(t, db, uid, []string{doulaRole}, employeeType)
	seedCoOwner(t, db, practiceID, "owner-of-a-practice-with-a-ghost")

	// Seeded nowhere: the fake holds no account for this uid at all.
	accounts := authntest.NewFakeAccountManager()
	srv := newDeleteLoginServer(t, db, accounts)
	defer srv.Close()

	resp := deleteLogin(t, srv, authntest.SeedSession(t, db.App, uid), true)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusNoContent {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusNoContent)
	}
	if _, _, _, deleted := readStaffRow(t, db, staffID); !deleted {
		t.Fatal("an already-absent Identity Platform account stopped the redaction")
	}
}

// TestDeleteLoginHandler_KeepsHerAuthoredWork is ADR-0027's rule applied
// to a Staff person: redact the identity, keep the record. Every
// actor_staff_id still resolves -- to a row that no longer names her.
func TestDeleteLoginHandler_KeepsHerAuthoredWork(t *testing.T) {
	db := testdb.New(t)
	const uid = "doula-with-a-history"
	practiceID, staffID := testdb.SeedStaffAtNewPractice(t, db, uid, []string{doulaRole}, employeeType)
	seedCoOwner(t, db, practiceID, "owner-of-a-practice-with-history")
	clientID, _ := testdb.SeedEngagement(t, db, practiceID)
	testdb.SeedActivity(t, db, practiceID, "client", clientID, "created", activity.StaffActor(staffID))

	accounts := authntest.NewFakeAccountManager()
	accounts.Seed(uid, "tasha@example.com", true)
	srv := newDeleteLoginServer(t, db, accounts)
	defer srv.Close()

	resp := deleteLogin(t, srv, authntest.SeedSession(t, db.App, uid), true)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusNoContent {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusNoContent)
	}

	if n := countRows(t, db,
		`SELECT count(*) FROM activity a JOIN staff s ON s.id = a.actor_staff_id
		  WHERE a.subject_id = $1 AND a.action = 'created'`, clientID); n != 1 {
		t.Fatalf("resolvable authored rows = %d, want her work to still resolve to the redacted row", n)
	}
}

// TestDeleteLoginHandler_ResolvesQueuedMailAddressedToHer covers the four
// outbox tables keyed on her Identity Platform uid. Each is resolved at
// source, before the sentinel makes the row unfindable -- without it a
// verification link, an address-change notice, a sign-in notice and a
// recovery code all dead-letter on an account Identity Platform no longer
// holds.
func TestDeleteLoginHandler_ResolvesQueuedMailAddressedToHer(t *testing.T) {
	db := testdb.New(t)
	const uid = "doula-with-mail-in-flight"
	practiceID, staffID := testdb.SeedStaffAtNewPractice(t, db, uid, []string{doulaRole}, employeeType)
	seedCoOwner(t, db, practiceID, "owner-whose-mail-is-unaffected")

	seedPendingMail(t, db, uid, staffID)

	accounts := authntest.NewFakeAccountManager()
	accounts.Seed(uid, "tasha@example.com", true)
	srv := newDeleteLoginServer(t, db, accounts)
	defer srv.Close()

	resp := deleteLogin(t, srv, authntest.SeedSession(t, db.App, uid), true)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusNoContent {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusNoContent)
	}

	for _, table := range []string{
		"staff_token_mail_outbox",
		"session_notice_outbox",
		"staff_mfa_recovery_outbox",
	} {
		//nolint:gosec // the table name is this test's own literal, from the slice above
		if n := countRows(t, db,
			`SELECT count(*) FROM `+table+` WHERE status = 'pending'`); n != 0 {
			t.Fatalf("%s: pending rows = %d, want every one addressed to her resolved", table, n)
		}
	}

	// And the one deliberately left to send: somebody changed the address
	// on an account, and the person who used to own that mailbox is
	// exactly who needs to hear it. Suppressing it would hand anyone who
	// reached her session a way to move her address and then silence the
	// notice by deleting the login.
	if n := countRows(t, db,
		`SELECT count(*) FROM staff_email_change_outbox WHERE status = 'pending'`); n != 1 {
		t.Fatalf("staff_email_change_outbox: pending rows = %d, want the address-change notice still to go out", n)
	}
}

// seedPendingMail queues one pending row addressed to uid in each of the
// four outbox tables the deletion has to resolve.
func seedPendingMail(t *testing.T, db *testdb.DB, uid, staffID string) {
	t.Helper()
	exec := func(query string, args ...any) {
		t.Helper()
		if _, err := db.Admin.ExecContext(t.Context(), query, args...); err != nil {
			t.Fatalf("seed pending mail: %v", err)
		}
	}
	exec(`INSERT INTO staff_token_mail_outbox (identity_uid, kind, token) VALUES ($1, 'email_verification', 'a-token')`, uid)
	exec(`INSERT INTO staff_email_change_outbox (identity_uid, old_email) VALUES ($1, 'old@example.com')`, uid)
	exec(`INSERT INTO session_notice_outbox (identity_uid, kind) VALUES ($1, 'new_signin')`, uid)
	exec(`INSERT INTO staff_mfa_recovery_outbox (recipient_identity_uid, subject_staff_id, token)
	      VALUES ($1, $2, '12345678')`, uid, staffID)
}
