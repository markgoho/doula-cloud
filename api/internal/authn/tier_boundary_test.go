package authn_test

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"doula-cloud/api/internal/authn"
	"doula-cloud/api/internal/authntest"
	"doula-cloud/api/internal/portalaccount"
	"doula-cloud/api/internal/testdb"
)

// beginAs runs authn.Begin for the population want names, against a
// request carrying token as its __session cookie. Its own helper rather
// than beginRequest's, which fixes the tier at TierStaff -- the whole
// subject here is what happens when the cookie's tier and the caller's
// disagree.
func beginAs(t *testing.T, db *testdb.DB, token string, want authn.Tier) (*httptest.ResponseRecorder, string, bool) {
	t.Helper()
	rec := httptest.NewRecorder()
	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/", nil)
	authntest.AddSessionCookie(req, token)

	tx, uid, _, ok := authn.Begin(rec, req, db.App, want)
	if ok {
		t.Cleanup(func() { _ = tx.Rollback() })
	}
	return rec, uid, ok
}

// TestBegin_RefusesTheOtherPopulationsSession is #1024's seam: a live
// session is a credential for the population it was issued in and for
// nothing else. Both directions, because both were open -- a Client
// portal session reached every /api/staff/* route, and a Staff session
// reached every /api/portal/* one; each was caught downstream only by
// that population's own table lookup finding no row.
//
// The refusal is byte-identical to the one a cookie naming no live
// session gets, which is the point rather than an accident: any
// distinguishable answer would tell the caller which namespace her
// cookie was issued in, and she proved only that she holds it.
func TestBegin_RefusesTheOtherPopulationsSession(t *testing.T) {
	tests := []struct {
		name string
		uid  string
		want authn.Tier
	}{
		{
			name: "portal session at a staff seam",
			uid:  portalaccount.NewIdentifier(),
			want: authn.TierStaff,
		},
		{
			name: "staff session at a portal seam",
			uid:  testUID,
			want: authn.TierPortal,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db := testdb.New(t)
			token := authntest.SeedSession(t, db.App, tt.uid)

			rec, _, ok := beginAs(t, db, token, tt.want)
			if ok {
				t.Fatal("expected ok=false, got true -- the other population's session was accepted")
			}
			assertUnauthorized(t, rec, authn.MsgInvalidSession)
		})
	}
}

// TestBegin_RefusedTierIsNotRenewed holds the ordering the check depends
// on: it runs above renewIfStale, not after Begin returns. The session
// is seeded past half its 30-day life, so a check placed after renewal
// would answer 401 while handing the browser a freshly extended cookie
// and pushing the row's expires_at out -- refusing a request and
// rewarding it in the same breath. assertUnauthorized covers the cookie;
// the store read below covers the row, which no response header would
// show.
func TestBegin_RefusedTierIsNotRenewed(t *testing.T) {
	db := testdb.New(t)
	portalUID := portalaccount.NewIdentifier()
	// Minted 20 days ago, so 10 of its 30 days remain -- past half life.
	token := authntest.SeedSessionAt(t, db.App, portalUID, time.Now().Add(-20*24*time.Hour))

	before := sessionExpiry(t, db, portalUID)
	rec, _, ok := beginAs(t, db, token, authn.TierStaff)
	if ok {
		t.Fatal("expected ok=false, got true")
	}
	assertUnauthorized(t, rec, authn.MsgInvalidSession)

	if after := sessionExpiry(t, db, portalUID); !after.Equal(before) {
		t.Fatalf("expires_at moved from %v to %v -- the refused request renewed the session it was refused for", before, after)
	}
}

// TestBegin_AcceptsItsOwnPopulationsSession is the control: the tier
// argument refuses the other population without refusing the one it
// serves, so the test above cannot pass by refusing everything.
func TestBegin_AcceptsItsOwnPopulationsSession(t *testing.T) {
	db := testdb.New(t)
	portalUID := portalaccount.NewIdentifier()
	token := authntest.SeedSession(t, db.App, portalUID)

	_, uid, ok := beginAs(t, db, token, authn.TierPortal)
	if !ok {
		t.Fatal("expected ok=true, got false")
	}
	if uid != portalUID {
		t.Fatalf("uid = %q, want %q", uid, portalUID)
	}
}

// sessionExpiry reads expires_at for identityUID's one session row.
func sessionExpiry(t *testing.T, db *testdb.DB, identityUID string) time.Time {
	t.Helper()
	var expiresAt time.Time
	if err := db.App.QueryRowContext(t.Context(),
		`SELECT expires_at FROM sessions WHERE identity_uid = $1`, identityUID,
	).Scan(&expiresAt); err != nil {
		t.Fatalf("read expires_at: %v", err)
	}
	return expiresAt
}
