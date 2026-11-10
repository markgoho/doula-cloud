package staffauth_test

import (
	"errors"
	"net/http"
	"strings"
	"testing"
	"time"

	"doula-cloud/api/internal/activity"
	"doula-cloud/api/internal/authntest"
	"doula-cloud/api/internal/fixture"
	"doula-cloud/api/internal/reqtest"
	"doula-cloud/api/internal/staffauth"
	"doula-cloud/api/internal/testdb"
)

// newDeleter is a Staff member about to delete her own login: a Practice
// with a second Owner to stay behind, and an Identity Platform account for
// her. The env mounts the real route table -- not a bare handler -- so
// every test here goes through the same rate limiter, gated router and
// session cookie a browser would.
func newDeleter(t *testing.T, staff ...fixture.StaffOption) *env {
	t.Helper()
	e := newSoleStaffEnv(t, staff...)
	e.Colleague(fixture.Owner())
	return e
}

// newSoleStaffEnv is newDeleter with nobody to stay behind: she is the
// Practice's only Staff member, with an Identity Platform account.
func newSoleStaffEnv(t *testing.T, staff ...fixture.StaffOption) *env {
	t.Helper()
	e := newEnv(t, signedInAs(staff...), withAccounts(authntest.NewFakeAccountManager()))
	e.accounts.Seed(e.Staff.UID, "tasha@example.com", true)
	return e
}

// deleteLogin issues the request under test as who and requires want.
// confirmed controls the X-Confirmed backstop, which is the only header
// this endpoint reads.
func (e *env) deleteLogin(want int, who reqtest.Option, confirm bool) []byte {
	e.T.Helper()
	return reqtest.ExpectBody(e.T, http.MethodDelete, e.URL("/api/staff/account"), want, who, reqtest.HeaderIf(confirm, "X-Confirmed", "true"))
}

// erase deletes the login of the Staff member at the screen and requires
// the 204.
func (e *env) erase() { e.deleteLogin(http.StatusNoContent, e.SignedIn, true) }

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

// requireNotErased fails unless a refused request left her row and her
// Identity Platform account as they were.
func requireNotErased(t *testing.T, e *env) {
	t.Helper()
	if _, _, _, deleted := readStaffRow(t, e.DB, e.Staff.ID); deleted {
		t.Fatal("the refused request redacted her row anyway")
	}
	if !e.accounts.Exists(e.Staff.UID) {
		t.Fatal("the refused request destroyed her Identity Platform account anyway")
	}
}

// TestDeleteLoginHandler_RedactsRowAndDestroysAccount is the whole act on
// the simplest shape there is: a contractor doula at one Practice she does
// not own.
func TestDeleteLoginHandler_RedactsRowAndDestroysAccount(t *testing.T) {
	e := newDeleter(t, fixture.Doula(), fixture.Contractor())

	e.erase()

	name, email, identityUID, deleted := readStaffRow(t, e.DB, e.Staff.ID)
	if name != staffauth.DeletedStaffName {
		t.Fatalf("name = %q, want %q", name, staffauth.DeletedStaffName)
	}
	if email != staffauth.DeletedStaffEmail {
		t.Fatalf("email = %q, want %q", email, staffauth.DeletedStaffEmail)
	}
	if identityUID != "deleted:"+e.Staff.ID {
		t.Fatalf("identity_uid = %q, want the sentinel derived from the row's own id", identityUID)
	}
	if !deleted {
		t.Fatal("deleted_at is not stamped, so nothing proves the act ran")
	}
	if e.accounts.Exists(e.Staff.UID) {
		t.Fatal("her Identity Platform account survived, so she can still authenticate")
	}
	if n := countRows(t, e.DB, `SELECT count(*) FROM practice_memberships WHERE staff_id = $1`, e.Staff.ID); n != 0 {
		t.Fatalf("memberships left = %d, want every one ended", n)
	}
	if n := countRows(t, e.DB,
		`SELECT count(*) FROM staff_auth_events WHERE staff_id = $1 AND actor_staff_id = $1 AND reason = 'login_deleted'`,
		e.Staff.ID); n != 1 {
		t.Fatalf("login_deleted events = %d, want the act recorded exactly once with her as her own actor", n)
	}
}

// TestDeleteLoginHandler_EndsEveryMembershipWithItsOwnEvent is the
// cross-Practice case the whole ticket exists for: a contractor doula at
// two Practices leaves both in one act, and each Practice's own Activity
// log carries the 'removed' event naming what the Membership held.
func TestDeleteLoginHandler_EndsEveryMembershipWithItsOwnEvent(t *testing.T) {
	e := newDeleter(t, fixture.Doula(), fixture.Contractor())
	second := fixture.Practice(t, e.DB, fixture.PracticeName("Second Practice"))
	fixture.Staff(t, e.DB, second, fixture.Owner())
	e.Exec(`INSERT INTO practice_memberships (practice_id, staff_id, roles, employment_type)
		 VALUES ($1, $2, '{doula,admin}', 'contractor')`, second.ID, e.Staff.ID)

	e.erase()

	for _, practiceID := range []string{e.Practice.ID, second.ID} {
		if n := countRows(t, e.DB,
			`SELECT count(*) FROM activity
			  WHERE practice_id = $1 AND subject_kind = 'membership' AND subject_id = $2
			    AND action = 'removed' AND actor_staff_id = $2`,
			practiceID, e.Staff.ID); n != 1 {
			t.Fatalf("practice %s: 'removed' events = %d, want exactly one naming her as her own actor", practiceID, n)
		}
	}

	diff := testdb.Scalar[string](t, e.DB.Admin, `SELECT diff::text FROM activity
		  WHERE practice_id = $1 AND subject_id = $2 AND action = 'removed'`, second.ID, e.Staff.ID)
	if !strings.Contains(diff, "admin") || !strings.Contains(diff, contractorType) {
		t.Fatalf("removal diff = %s, want the roles and employment type the membership held", diff)
	}
}

// TestDeleteLoginHandler_EndsEverySession is the ordering the handler's
// own comment calls load-bearing: sessions is keyed on identity_uid, so a
// redaction that overwrote it first would leave her signed in forever on
// a live cookie.
func TestDeleteLoginHandler_EndsEverySession(t *testing.T) {
	e := newDeleter(t)
	otherBrowser := authntest.SeedSession(t, e.DB.App, e.Staff.UID)

	e.erase()
	if got := authntest.CountFor(t, e.DB.App, e.Staff.UID); got != 0 {
		t.Fatalf("session rows left = %d, want none anywhere", got)
	}

	// The token from the browser she was not holding must be dead too --
	// a live cookie is the whole of a session's credential.
	e.deleteLogin(http.StatusUnauthorized, reqtest.Session(otherBrowser), true)
}

// TestDeleteLoginHandler_RefusesWhileSoleOwner is the refusal the ticket
// names for the last Owner, and proves it names the Practice rather than
// merely saying no. A Practice inside ADR-0031's 30-day deletion window
// still needs an Owner too, because letting her go would foreclose the
// restore that window exists to promise.
func TestDeleteLoginHandler_RefusesWhileSoleOwner(t *testing.T) {
	cases := []struct {
		name            string
		practiceName    string
		pendingDeletion bool
	}{
		{"a Practice in use", "Rochester Birth Collective", false},
		{"a Practice pending deletion", "Winding Down Doulas", true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			e := newSoleStaffEnv(t, fixture.Owner())
			e.Exec(`UPDATE practices SET name = $2 WHERE id = $1`, e.Practice.ID, tc.practiceName)
			if tc.pendingDeletion {
				e.Exec(`UPDATE practices
				    SET deletion_requested_at = now(), deletion_requested_by = $2, deletion_finalize_at = now() + interval '30 days'
				  WHERE id = $1`, e.Practice.ID, e.Staff.ID)
			}

			body := e.deleteLogin(http.StatusConflict, e.SignedIn, true)

			if !strings.Contains(string(body), tc.practiceName) {
				t.Fatalf("body = %s, want the practice in the way named", body)
			}
			requireNotErased(t, e)
		})
	}
}

// TestDeleteLoginHandler_AllowsSoleOwnerOfFinalizedPractice is the other
// side of that filter: once a Practice is finalized nobody can reach it,
// so being its Owner no longer means anything and must not trap her.
func TestDeleteLoginHandler_AllowsSoleOwnerOfFinalizedPractice(t *testing.T) {
	e := newSoleStaffEnv(t, fixture.Owner())
	now := time.Now()
	testdb.SeedPracticeDeletion(t, e.DB, e.Practice.ID, now.AddDate(0, 0, -30), now, &now)

	e.erase()

	if _, _, _, deleted := readStaffRow(t, e.DB, e.Staff.ID); !deleted {
		t.Fatal("she was refused over a practice nobody can reach any more")
	}
}

// TestDeleteLoginHandler_AllowsOwnerWithACoOwner proves the refusal is
// about being the *last* Owner and not about holding the role -- the case
// that would otherwise make every Owner permanently undeletable.
func TestDeleteLoginHandler_AllowsOwnerWithACoOwner(t *testing.T) {
	e := newSoleStaffEnv(t, fixture.Owner())
	coOwner := e.Colleague(fixture.Owner())
	seedEnrolled(t, e.DB, coOwner.ID)

	e.erase()

	if _, _, _, deleted := readStaffRow(t, e.DB, e.Staff.ID); !deleted {
		t.Fatal("an Owner with a co-Owner was refused")
	}
	// #615's rule, inherited from RemoveMembershipHandler: the co-Owner is
	// newly sole, and has a second factor (#1531), so she is newly
	// entitled to saved recovery codes.
	if n := countRows(t, e.DB,
		`SELECT count(*) FROM staff_mfa_recovery_codes
		  WHERE staff_id = $1 AND used_at IS NULL AND revoked_at IS NULL`, coOwner.ID); n == 0 {
		t.Fatal("the remaining Owner became sole and was not issued saved codes")
	}
}

// TestDeleteLoginHandler_RequiresConfirmation is CLAUDE.md's hard block
// with a deliberate override, at the boundary that can enforce it: the
// frontend's dialog is not an enforcement point, the header is.
func TestDeleteLoginHandler_RequiresConfirmation(t *testing.T) {
	e := newDeleter(t)

	e.deleteLogin(http.StatusBadRequest, e.SignedIn, false)

	requireNotErased(t, e)
}

// TestDeleteLoginHandler_RefusesWithoutAStaffMember proves the endpoint
// acts only on the caller's own identity: there is no staff id anywhere in
// the route or the body, so with no credential there is nobody to act on.
// A person who holds a live session minted before her staff row existed
// gets the same MsgNoMatchingStaffAccount shape RotateSavedCodesHandler
// answers with.
func TestDeleteLoginHandler_RefusesWithoutAStaffMember(t *testing.T) {
	cases := []struct {
		name    string
		want    int
		session func(e *env) reqtest.Option
	}{
		{"no session", http.StatusUnauthorized, func(*env) reqtest.Option { return reqtest.Session("") }},
		{noStaffRow, http.StatusNotFound, func(e *env) reqtest.Option {
			return reqtest.Session(authntest.SeedSession(e.T, e.DB.App, "a-uid-with-no-staff-row"))
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			e := newEnv(t)

			e.deleteLogin(tc.want, tc.session(e), true)
		})
	}
}

// TestDeleteLoginHandler_RefusesASecondDelete is the ticket's
// run-it-twice criterion. The second call presents a token that named the
// deleted row, and there is nothing left for it to name: her sessions are
// gone with the first act, so it never reaches the handler at all. That
// is the refusal, and it is stronger than a 409 -- there is no live
// credential in existence that can address a deleted login.
func TestDeleteLoginHandler_RefusesASecondDelete(t *testing.T) {
	e := newDeleter(t)

	e.erase()
	e.deleteLogin(http.StatusUnauthorized, e.SignedIn, true)

	if n := countRows(t, e.DB,
		`SELECT count(*) FROM staff_auth_events WHERE staff_id = $1 AND reason = 'login_deleted'`, e.Staff.ID); n != 1 {
		t.Fatalf("login_deleted events = %d, want the act recorded once, not twice", n)
	}
}

// TestDeleteLoginHandler_IdentityPlatformFailureRollsEverythingBack is why
// the Admin SDK call sits inside the transaction rather than after the
// commit: a person must never be left with a redacted row and a live
// account, nor with an account destroyed and a row that still works.
func TestDeleteLoginHandler_IdentityPlatformFailureRollsEverythingBack(t *testing.T) {
	e := newDeleter(t)
	e.accounts.DeleteAccountErr = errors.New("identity platform unreachable")

	e.deleteLogin(http.StatusInternalServerError, e.SignedIn, true)

	if _, _, _, deleted := readStaffRow(t, e.DB, e.Staff.ID); deleted {
		t.Fatal("her row was redacted even though the account delete failed")
	}
	if n := countRows(t, e.DB, `SELECT count(*) FROM practice_memberships WHERE staff_id = $1`, e.Staff.ID); n != 1 {
		t.Fatalf("memberships = %d, want the failed act to have ended none", n)
	}
	if got := authntest.CountFor(t, e.DB.App, e.Staff.UID); got != 1 {
		t.Fatalf("session rows = %d, want the failed act to have ended none", got)
	}
}

// TestDeleteLoginHandler_AbsentIdentityPlatformAccountIsSuccess is the
// idempotency the interface promises: an account Identity Platform
// already reports absent must not stop the rest of the act.
func TestDeleteLoginHandler_AbsentIdentityPlatformAccountIsSuccess(t *testing.T) {
	e := newDeleter(t)
	if err := e.accounts.DeleteAccount(t.Context(), e.Staff.UID); err != nil {
		t.Fatalf("remove the account: %v", err)
	}

	e.erase()

	if _, _, _, deleted := readStaffRow(t, e.DB, e.Staff.ID); !deleted {
		t.Fatal("an already-absent Identity Platform account stopped the redaction")
	}
}

// TestDeleteLoginHandler_KeepsHerAuthoredWork is ADR-0027's rule applied
// to a Staff person: redact the identity, keep the record. Every
// actor_staff_id still resolves -- to a row that no longer names her.
func TestDeleteLoginHandler_KeepsHerAuthoredWork(t *testing.T) {
	e := newDeleter(t)
	clientID, _ := testdb.SeedEngagement(t, e.DB, e.Practice.ID)
	testdb.SeedActivity(t, e.DB, e.Practice.ID, "client", clientID, "created", activity.StaffActor(e.Staff.ID))

	e.erase()

	if n := countRows(t, e.DB,
		`SELECT count(*) FROM activity a JOIN staff s ON s.id = a.actor_staff_id
		  WHERE a.subject_id = $1 AND a.action = 'created'`, clientID); n != 1 {
		t.Fatalf("resolvable authored rows = %d, want her work to still resolve to the redacted row", n)
	}
}

// TestDeleteLoginHandler_LeavesHerFeedbackInPlace is #1525's Login
// deletion AC, settled on #1501 from ADR-0033: her staff row is redacted
// where it sits, so every piece of Feedback she sent remains, with its
// text and its issue number, and names nobody -- the same as every other
// row she left. Nothing queues a close for its issue. One piece is from
// under her Practice and one is from /account, with no Practice at all.
func TestDeleteLoginHandler_LeavesHerFeedbackInPlace(t *testing.T) {
	e := newDeleter(t)
	e.Exec(`INSERT INTO feedback (id, kind, text, page_url, route_id, app_build, screen_width, browser,
		     staff_id, practice_id, roles, issue_number, sent_at)
		 VALUES (gen_random_uuid(), 'not_working', 'the schedule will not save', '/practices/x', '/practices/[practiceId]',
		         'abc1234', 390, 'Safari 18', $1, $2, '{doula}', 7, now()),
		        (gen_random_uuid(), 'idea_or_request', 'a dark theme', '/account', '/account',
		         'abc1234', 390, 'Safari 18', $1, NULL, NULL, 8, now())`,
		e.Staff.ID, e.Practice.ID)

	e.erase()

	if n := countRows(t, e.DB,
		`SELECT count(*) FROM feedback f JOIN staff s ON s.id = f.staff_id
		  WHERE f.staff_id = $1 AND f.issue_number IN (7, 8) AND f.text <> ''`, e.Staff.ID); n != 2 {
		t.Fatalf("her pieces of Feedback = %d, want both still there and still resolving to her redacted row", n)
	}
	if n := countRows(t, e.DB, `SELECT count(*) FROM feedback_issue_outbox WHERE act = 'close_erased'`); n != 0 {
		t.Fatalf("close jobs = %d, want 0 -- a Login deletion closes no issue", n)
	}
}

// TestDeleteLoginHandler_ResolvesQueuedMailAddressedToHer covers the four
// outbox tables keyed on her Identity Platform uid. Each is resolved at
// source, before the sentinel makes the row unfindable -- without it a
// verification link, an address-change notice, a sign-in notice and a
// recovery code all dead-letter on an account Identity Platform no longer
// holds.
func TestDeleteLoginHandler_ResolvesQueuedMailAddressedToHer(t *testing.T) {
	e := newDeleter(t)
	seedPendingMail(e)

	e.erase()

	for _, table := range []string{
		tokenMailOutbox,
		"session_notice_outbox",
		recoveryOutbox,
	} {
		//nolint:gosec // the table name is this test's own literal, from the slice above
		if n := countRows(t, e.DB,
			`SELECT count(*) FROM `+table+` WHERE status = 'pending'`); n != 0 {
			t.Fatalf("%s: pending rows = %d, want every one addressed to her resolved", table, n)
		}
	}

	// Each is resolved as skipped for her deletion, not marked sent, and
	// the two that carried a plaintext secret -- her own reset or
	// verification token, and a recovery code, which is a way back into
	// another person's account -- no longer hold it. 00061 promises the
	// token is gone the moment a row leaves pending; the deletion once
	// marked these rows sent by hand and left both behind.
	for _, table := range []string{
		tokenMailOutbox,
		"session_notice_outbox",
		recoveryOutbox,
	} {
		//nolint:gosec // the table name is this test's own literal, from the slice above
		if n := countRows(t, e.DB,
			`SELECT count(*) FROM `+table+` WHERE status = 'skipped' AND skip_reason = 'login_deleted' AND skipped_at IS NOT NULL`); n != 1 {
			t.Fatalf("%s: rows skipped for login_deleted = %d, want 1", table, n)
		}
	}
	for _, table := range []string{tokenMailOutbox, recoveryOutbox} {
		//nolint:gosec // the table name is this test's own literal, from the slice above
		if n := countRows(t, e.DB, `SELECT count(*) FROM `+table+` WHERE token IS NOT NULL`); n != 0 {
			t.Fatalf("%s: rows still holding a token = %d, want 0", table, n)
		}
	}

	// And the one deliberately left to send: somebody changed the address
	// on an account, and the person who used to own that mailbox is
	// exactly who needs to hear it. Suppressing it would hand anyone who
	// reached her session a way to move her address and then silence the
	// notice by deleting the login.
	if n := countRows(t, e.DB,
		`SELECT count(*) FROM staff_email_change_outbox WHERE status = 'pending'`); n != 1 {
		t.Fatalf("staff_email_change_outbox: pending rows = %d, want the address-change notice still to go out", n)
	}
}

// seedPendingMail queues one pending row addressed to her in each of the
// four outbox tables the deletion has to resolve.
// The two outboxes addressed to her whose rows hold a plaintext secret.
const (
	tokenMailOutbox = "staff_token_mail_outbox"
	recoveryOutbox  = "staff_mfa_recovery_outbox"
)

func seedPendingMail(e *env) {
	e.Exec(`INSERT INTO staff_token_mail_outbox (identity_uid, kind, token) VALUES ($1, 'email_verification', 'a-token')`, e.Staff.UID)
	e.Exec(`INSERT INTO staff_email_change_outbox (identity_uid, old_email) VALUES ($1, 'old@example.com')`, e.Staff.UID)
	e.Exec(`INSERT INTO session_notice_outbox (identity_uid, kind) VALUES ($1, 'new_signin')`, e.Staff.UID)
	e.Exec(`INSERT INTO staff_mfa_recovery_outbox (recipient_identity_uid, subject_staff_id, token)
	      VALUES ($1, $2, '12345678')`, e.Staff.UID, e.Staff.ID)
}
