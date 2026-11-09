package portalinvite_test

import (
	"database/sql"
	"net/http"
	"testing"
	"time"

	"doula-cloud/api/internal/fixture"
	"doula-cloud/api/internal/idempotency"
	"doula-cloud/api/internal/portalinvite"
	"doula-cloud/api/internal/reqtest"
	"doula-cloud/api/internal/staffauth"
	"doula-cloud/api/internal/tasknudge"
	"doula-cloud/api/internal/testdb"
)

// doulaRole is named once so golangci-lint's goconst check doesn't see
// repeated "doula" literals across this package's test surface.
const doulaRole = "doula"

// invitedAddress is the address every fixture invitation in this package
// goes to. Named rather than repeated as a literal because two fixtures
// have to agree on it for a reuse accept to reuse anything: the Portal
// Account's sign-in address is the invited Client's own contact address
// (ADR-0026), so a test seeding both sides is silently testing nothing
// if the two drift apart.
const invitedAddress = "invited@example.com"

const (
	notAUUID    = "not-a-uuid"
	unknownUUID = "00000000-0000-0000-0000-000000000000"
)

func mount(g *staffauth.GatedRouter, ir *idempotency.Router, app *sql.DB) {
	portalinvite.Mount(g, ir, app, &tasknudge.FakeEnqueuer{})
}

// newInviteScene mounts this package's whole surface through
// portalinvite.Mount, the same call main.go makes on the real GatedRouter
// and idempotency.Router, with a Doula at the screen. The Client-portal
// invitation-acceptance route is one of the routes: public and pre-account
// (#617), the invitation token itself is the whole credential, so a
// request to it reads no session.
func newInviteScene(t *testing.T, opts ...fixture.SceneOption) *fixture.Scene {
	t.Helper()
	return fixture.NewScene(t, mount, opts...)
}

// newAcceptScene is a server with nobody signed in, for the accept route.
func newAcceptScene(t *testing.T) *fixture.Scene {
	t.Helper()
	return newInviteScene(t, fixture.NobodySignedIn())
}

// invitedClient is the Client a staff-side test invites: named, and
// reachable at email, which can be empty for a Client with no address on
// file.
func invitedClient(name, email string) fixture.SceneOption {
	if email == "" {
		return fixture.WithClient(fixture.ClientName(name), fixture.NoEmail())
	}
	return fixture.WithClient(fixture.ClientName(name), fixture.ClientEmail(email))
}

// invitePath is the portal-invite route of one Engagement.
func invitePath(engagementID string) string {
	return "/engagements/" + engagementID + "/portal-invite"
}

// acceptInvite posts an accept for token and requires the status want.
func acceptInvite(t *testing.T, s *fixture.Scene, token string, want int) {
	t.Helper()
	reqtest.ExpectStatus(t, http.MethodPost, s.URL("/api/portal/accept-invite"), want,
		reqtest.JSON(portalinvite.AcceptInviteRequest{InviteToken: token}))
}

// seedPendingPortalInvite seeds a Practice, a Staff member, a Client with
// an Engagement at that Practice, and a pending (unclaimed) portal invite
// for the Client -- the state InviteHandler leaves behind for
// AcceptInviteHandler to pick up.
func seedPendingPortalInvite(t *testing.T, db *testdb.DB) (clientID, inviteToken string) {
	t.Helper()
	return seedPendingPortalInviteExpiringAt(t, db, time.Now().Add(7*24*time.Hour))
}

// seedPendingPortalInviteExpiringAt is seedPendingPortalInvite with an
// explicit invite_token_expires_at, for a test that drives the #616
// expiry check directly (a time in the past for an already-expired
// invite; NULL is what a fixture predating this migration would carry --
// no other caller needs that case, so no third helper exists for it).
func seedPendingPortalInviteExpiringAt(t *testing.T, db *testdb.DB, expiresAt time.Time) (clientID, inviteToken string) {
	t.Helper()
	practiceID, _ := testdb.SeedStaffAtNewPractice(t, db, "portal-invite-owner", []string{doulaRole}, "employee")
	clientID, _ = testdb.SeedNamedEngagement(t, db, practiceID, "Invited Client", invitedAddress)
	return clientID, insertPendingPortalInvite(t, db, clientID, expiresAt)
}

// insertPendingPortalInvite writes the unclaimed client_portal_users row
// itself -- the one row AcceptInviteHandler's token names -- and hands
// back the token. Split out from the fixture above so a test that needs a
// differently shaped Practice around it (see the two-Practice walk in
// accept_second_practice_test.go) reuses this row's exact shape rather
// than writing a second INSERT that could drift from it.
func insertPendingPortalInvite(t *testing.T, db *testdb.DB, clientID string, expiresAt time.Time) (inviteToken string) {
	t.Helper()
	if err := db.Admin.QueryRowContext(t.Context(),
		`INSERT INTO client_portal_users (client_id, invite_token, invite_token_expires_at) VALUES ($1, gen_random_uuid(), $2) RETURNING invite_token::text`,
		clientID, expiresAt,
	).Scan(&inviteToken); err != nil {
		t.Fatalf("seed pending portal invite: %v", err)
	}
	return inviteToken
}
