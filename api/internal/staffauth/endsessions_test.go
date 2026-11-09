package staffauth_test

import (
	"net/http"
	"testing"

	"doula-cloud/api/internal/authntest"
	"doula-cloud/api/internal/fixture"
	"doula-cloud/api/internal/reqtest"
	"doula-cloud/api/internal/testdb"
)

// endSessions ends every session staffID holds at practiceID and requires
// want. confirm controls the X-Confirmed backstop.
func (e *env) endSessions(practiceID, staffID string, want int, confirm bool) {
	e.T.Helper()
	url := e.URL("/api/practices/" + practiceID + "/staff/" + staffID + "/sessions")
	reqtest.ExpectStatus(e.T, http.MethodDelete, url, want, e.SignedIn, reqtest.HeaderIf(confirm, "X-Confirmed", "true"))
}

func TestEndSessionsHandler_RequiresConfirmation(t *testing.T) {
	e := newEnv(t, signedInAs(fixture.Owner()))
	target := e.Colleague()
	authntest.SeedSession(t, e.DB.App, target.UID)

	e.endSessions(e.Practice.ID, target.ID, http.StatusBadRequest, false)

	if got := authntest.CountFor(t, e.DB.App, target.UID); got != 1 {
		t.Fatalf("target session rows = %d, want the unconfirmed request to have ended nothing", got)
	}
}

// TestEndSessionsHandler_RecordsActivity is #473's fix: ending a Staff
// member's sessions used to leave no record of who did it or when.
func TestEndSessionsHandler_RecordsActivity(t *testing.T) {
	e := newEnv(t, signedInAs(fixture.Owner()))
	target := e.Colleague()
	authntest.SeedSession(t, e.DB.App, target.UID)

	e.endSessions(e.Practice.ID, target.ID, http.StatusNoContent, true)

	var action, actor string
	if err := e.DB.Admin.QueryRowContext(t.Context(),
		`SELECT action, actor_staff_id FROM activity WHERE subject_kind = 'membership' AND subject_id = $1 AND action = 'sessions_ended'`,
		target.ID,
	).Scan(&action, &actor); err != nil {
		t.Fatalf("read activity row: %v", err)
	}
	if actor != e.Staff.ID {
		t.Fatalf("actor = %q, want the Owner %q", actor, e.Staff.ID)
	}
}

// TestEndSessionsHandler_Refuses covers who and what is turned away: a
// non-Owner; an Owner elsewhere, refused by staffauth.Middleware itself
// before RequireOwner ever runs, since she holds no membership at the
// Practice named in the URL; and a Staff member nobody has.
func TestEndSessionsHandler_Refuses(t *testing.T) {
	cases := []struct {
		name  string
		owner bool
		// target returns the Practice and Staff ids the request names.
		target func(e *env) (practiceID, staffID string)
		want   int
	}{
		{"a non-Owner", false, func(e *env) (string, string) { return e.Practice.ID, e.Staff.ID }, http.StatusForbidden},
		{"an Owner at a different Practice", true, func(e *env) (string, string) {
			other := fixture.Practice(e.T, e.DB)
			return other.ID, fixture.Staff(e.T, e.DB, other).ID
		}, http.StatusForbidden},
		{"no such membership", true, func(e *env) (string, string) { return e.Practice.ID, emptyUUID }, http.StatusNotFound},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var opts []envOption
			if tc.owner {
				opts = append(opts, signedInAs(fixture.Owner()))
			}
			e := newEnv(t, opts...)
			practiceID, staffID := tc.target(e)

			e.endSessions(practiceID, staffID, tc.want, true)
		})
	}
}

// TestEndSessionsHandler_Success is the ticket's core behavior: every
// session the target holds, across devices, is gone, while the acting
// Owner's own session is untouched.
func TestEndSessionsHandler_Success(t *testing.T) {
	e := newEnv(t, signedInAs(fixture.Owner()))
	target := e.Colleague()
	authntest.SeedSession(t, e.DB.App, target.UID) // laptop
	authntest.SeedSession(t, e.DB.App, target.UID) // phone

	e.endSessions(e.Practice.ID, target.ID, http.StatusNoContent, true)

	if got := authntest.CountFor(t, e.DB.App, target.UID); got != 0 {
		t.Fatalf("target session rows = %d, want 0", got)
	}
	if got := authntest.CountFor(t, e.DB.App, e.Staff.UID); got != 1 {
		t.Fatalf("acting owner's own session rows = %d, want 1 (unaffected)", got)
	}
}

// TestEndSessionsHandler_QueuesSessionRevokedNotice covers #345: ending a
// Staff member's sessions everywhere queues a session-revoked notice for
// that Staff member, not the acting Owner.
func TestEndSessionsHandler_QueuesSessionRevokedNotice(t *testing.T) {
	e := newEnv(t, signedInAs(fixture.Owner()))
	target := e.Colleague()

	e.endSessions(e.Practice.ID, target.ID, http.StatusNoContent, true)

	kind := testdb.Scalar[string](t, e.DB.Admin, `SELECT kind FROM session_notice_outbox WHERE identity_uid = $1`, target.UID)
	if kind != "session_revoked" {
		t.Fatalf("kind = %q, want session_revoked", kind)
	}
}
