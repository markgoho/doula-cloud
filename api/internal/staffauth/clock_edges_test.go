package staffauth_test

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"doula-cloud/api/internal/authntest"
	"doula-cloud/api/internal/clock"
	"doula-cloud/api/internal/fixture"
	"doula-cloud/api/internal/reqtest"
	"doula-cloud/api/internal/staffauth"
	"doula-cloud/api/internal/tasknudge"
	"doula-cloud/api/internal/testdb"
)

// clockEdge is one microsecond, Postgres's timestamptz resolution: the
// smallest step either side of a rule's edge.
const clockEdge = time.Microsecond

// newClockedStaffServer is the whole surface behind clock.Middleware fixed
// at now (#1992), with uid's session seeded at that instant so it is live
// there.
func newClockedStaffServer(t *testing.T, db *testdb.DB, uid string, now time.Time) (srv *httptest.Server, session string) {
	t.Helper()
	fixed := clock.Middleware(func() time.Time { return now })
	srv = httptest.NewServer(fixed(staffauthMux(db, authntest.Verifier{}, authntest.NewFakeAccountManager(), tasknudge.NoOpEnqueuer{})))
	t.Cleanup(srv.Close)
	return srv, authntest.SeedSessionAt(t, db.App, uid, now)
}

// TestListStaffHandler_InvitationExpiredAtExpiresAtOnTheRequestClock is
// #1992's edge for listPendingInvitations: an Invitation reads as live
// one microsecond before its expires_at and as expired at it, on the
// request's clock -- the rule acceptInvite applies to the same column.
func TestListStaffHandler_InvitationExpiredAtExpiresAtOnTheRequestClock(t *testing.T) {
	expiresAt := time.Now().Add(time.Hour).Truncate(time.Microsecond).UTC()
	cases := []struct {
		name        string
		now         time.Time
		wantExpired bool
	}{
		{"just-before", expiresAt.Add(-clockEdge), false},
		{"at-expiry", expiresAt, true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			e := newEnv(t, signedInAs(fixture.Owner()), viaScene(fixture.At(c.now)))
			invitationID := seedInvitation(t, e.DB, e.Practice.ID, e.Staff.ID, "edge@example.com", "{doula}", employeeType, expiresAt)

			roster := e.roster("")
			if len(roster.Invitations.Items) != 1 || roster.Invitations.Items[0].InvitationID != invitationID {
				t.Fatalf("invitations = %+v, want the one seeded", roster.Invitations.Items)
			}
			if got := roster.Invitations.Items[0].Expired; got != c.wantExpired {
				t.Fatalf("expired = %v, want %v", got, c.wantExpired)
			}
		})
	}
}

// TestInviteHandler_StampsCreatedAtAndExpiresAtFromOneClock is #1992's
// third box for the Invitation: MintInvitation writes created_at and
// expires_at from the same clock.Now(ctx), on the first send and on a
// re-send, so the row's start and its lapse are on the clock every read
// of them uses.
func TestInviteHandler_StampsCreatedAtAndExpiresAtFromOneClock(t *testing.T) {
	db := testdb.New(t)
	const ownerUID = "owner-invites-on-a-fixed-clock"
	_, practiceID := seedOwnerMembership(t, db, ownerUID)
	sentAt := time.Now().Add(-time.Hour).Truncate(time.Microsecond).UTC()
	resentAt := sentAt.Add(10 * time.Minute)

	for _, at := range []time.Time{sentAt, resentAt} {
		srv, session := newClockedStaffServer(t, db, ownerUID, at)
		resp := postInvite(t, srv, session, practiceID, staffauth.InviteRequest{
			Email: "clocked@example.com", Roles: []string{doulaRole}, EmploymentType: employeeType,
		})
		_ = resp.Body.Close()
		if resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusOK {
			t.Fatalf("invite at %v: status = %d", at, resp.StatusCode)
		}

		var createdAt, expiresAt time.Time
		if err := db.Admin.QueryRowContext(t.Context(),
			`SELECT created_at, expires_at FROM practice_invitations WHERE practice_id = $1 AND address = 'clocked@example.com'`,
			practiceID,
		).Scan(&createdAt, &expiresAt); err != nil {
			t.Fatalf("read invitation: %v", err)
		}
		if !createdAt.Equal(at) || !expiresAt.Equal(at.Add(staffauth.InvitationLifetime)) {
			t.Fatalf("created_at = %v, expires_at = %v; want %v and %v", createdAt, expiresAt, at, at.Add(staffauth.InvitationLifetime))
		}
	}
}

// TestMiddleware_ActivityStampStaleAfterADayOnTheRequestClock is #1992's
// edge for last_active_at: a stamp exactly a day old at the request's
// clock is still current and is left alone, and one a microsecond older
// is stale and is refreshed to that clock's instant. querySelf and
// stampActivity both read clock.Now(ctx), the instant dormancy judges
// last_active_at against.
func TestMiddleware_ActivityStampStaleAfterADayOnTheRequestClock(t *testing.T) {
	now := time.Now().Truncate(time.Microsecond).UTC()
	cases := []struct {
		name      string
		stampedAt time.Time
		want      time.Time
	}{
		{"a-day-old", now.Add(-24 * time.Hour), now.Add(-24 * time.Hour)},
		{"just-past-a-day", now.Add(-24*time.Hour - clockEdge), now},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			e := newPingEnv(t, fixture.At(now))
			e.Exec(`UPDATE staff SET last_practice_id = $1, last_active_at = $2 WHERE id = $3`, e.Practice.ID, c.stampedAt, e.Staff.ID)

			reqtest.ExpectStatus(t, http.MethodGet, e.pingURL(), http.StatusOK, e.SignedIn)

			lastActiveAt := testdb.Scalar[time.Time](t, e.DB.Admin, `SELECT last_active_at FROM staff WHERE id = $1`, e.Staff.ID)
			if !lastActiveAt.Equal(c.want) {
				t.Fatalf("last_active_at = %v, want %v", lastActiveAt, c.want)
			}
		})
	}
}
