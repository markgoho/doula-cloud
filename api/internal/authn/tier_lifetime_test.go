package authn_test

import (
	"net/http"
	"testing"
	"time"

	"doula-cloud/api/internal/authn"
	"doula-cloud/api/internal/authntest"
	"doula-cloud/api/internal/portalaccount"
	"doula-cloud/api/internal/testdb"
)

// TestSessionLifetimeFor proves the branch #618/ADR-0026 asks for: a
// Portal Account identifier (portalaccount.Prefix) gets the 30-day
// lifetime, and anything else -- an Identity Platform uid, by
// construction -- keeps Staff's 12 hours.
func TestSessionLifetimeFor(t *testing.T) {
	if got := authn.SessionLifetimeFor("identity-platform-uid"); got != authn.SessionLifetime {
		t.Errorf("SessionLifetimeFor(staff uid) = %v, want %v", got, authn.SessionLifetime)
	}
	if got := authn.SessionLifetimeFor(portalaccount.NewIdentifier()); got != authn.PortalSessionLifetime {
		t.Errorf("SessionLifetimeFor(portal identifier) = %v, want %v", got, authn.PortalSessionLifetime)
	}
}

// TestMintSession_LifetimeByTier proves MintSession itself branches on
// the identity_uid it is given, both on the cookie it returns and on the
// row it writes -- not just on the pure function above.
func TestMintSession_LifetimeByTier(t *testing.T) {
	db := testdb.New(t)
	now := time.Now()
	portalUID := portalaccount.NewIdentifier()

	staffCookie, err := authn.MintSession(t.Context(), db.App, testUID, false, now)
	if err != nil {
		t.Fatalf("mint staff session: %v", err)
	}
	if staffCookie.MaxAge != int(authn.SessionLifetime.Seconds()) {
		t.Errorf("staff cookie MaxAge = %d, want %d", staffCookie.MaxAge, int(authn.SessionLifetime.Seconds()))
	}

	portalCookie, err := authn.MintSession(t.Context(), db.App, portalUID, false, now)
	if err != nil {
		t.Fatalf("mint portal session: %v", err)
	}
	if portalCookie.MaxAge != int(authn.PortalSessionLifetime.Seconds()) {
		t.Errorf("portal cookie MaxAge = %d, want %d", portalCookie.MaxAge, int(authn.PortalSessionLifetime.Seconds()))
	}

	assertExpiresAround(t, db, testUID, now.Add(authn.SessionLifetime))
	assertExpiresAround(t, db, portalUID, now.Add(authn.PortalSessionLifetime))
}

func assertExpiresAround(t *testing.T, db *testdb.DB, identityUID string, want time.Time) {
	t.Helper()
	var expiresAt time.Time
	if err := db.App.QueryRowContext(t.Context(),
		`SELECT expires_at FROM sessions WHERE identity_uid = $1`, identityUID,
	).Scan(&expiresAt); err != nil {
		t.Fatalf("read expires_at for %s: %v", identityUID, err)
	}
	if diff := expiresAt.Sub(want); diff < -time.Minute || diff > time.Minute {
		t.Errorf("expires_at for %s = %v, want ~%v", identityUID, expiresAt, want)
	}
}

// TestBegin_SessionCookie_RenewsPortalSessionAtItsOwnHalfLife proves
// renewal reads the portal identity's own 30-day lifetime rather than
// Staff's 12 hours: minted 20 days ago, it is past half of 30 days but
// would be laughably past half of 12 hours too, so the assertion that
// actually distinguishes the two is the renewed MaxAge itself.
func TestBegin_SessionCookie_RenewsPortalSessionAtItsOwnHalfLife(t *testing.T) {
	db := testdb.New(t)
	portalUID := portalaccount.NewIdentifier()
	token := authntest.SeedSessionAt(t, db.App, portalUID, time.Now().Add(-20*24*time.Hour))

	rec, uid, ok := beginRequest(t, db, func(req *http.Request) {
		authntest.AddSessionCookie(req, token)
	})
	if !ok {
		t.Fatal("expected ok=true, got false")
	}
	if uid != portalUID {
		t.Fatalf("uid = %q, want %q", uid, portalUID)
	}

	renewed := findSessionCookie(t, rec.Result().Cookies())
	if renewed.MaxAge != int(authn.PortalSessionLifetime.Seconds()) {
		t.Errorf("MaxAge = %d, want %d (30 days)", renewed.MaxAge, int(authn.PortalSessionLifetime.Seconds()))
	}
	assertExpiresAround(t, db, portalUID, time.Now().Add(authn.PortalSessionLifetime))
}

// TestBegin_SessionCookie_NoRenewalBeforePortalHalfLife proves a portal
// session well inside its first half -- 6 days into 30 -- is not
// renewed, even though 6 days would be well past half of Staff's own 12
// hours. If renewal read the wrong constant for this identity, this
// session would be renewed early.
func TestBegin_SessionCookie_NoRenewalBeforePortalHalfLife(t *testing.T) {
	db := testdb.New(t)
	portalUID := portalaccount.NewIdentifier()
	token := authntest.SeedSessionAt(t, db.App, portalUID, time.Now().Add(-6*24*time.Hour))

	rec, _, ok := beginRequest(t, db, func(req *http.Request) {
		authntest.AddSessionCookie(req, token)
	})
	if !ok {
		t.Fatal("expected ok=true, got false")
	}
	if got := rec.Result().Cookies(); len(got) != 0 {
		t.Fatalf("Set-Cookie = %v, want none", got)
	}
}
