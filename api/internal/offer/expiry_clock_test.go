package offer_test

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"doula-cloud/api/internal/authntest"
	"doula-cloud/api/internal/clock"
	"doula-cloud/api/internal/idempotency"
	"doula-cloud/api/internal/offer"
	"doula-cloud/api/internal/staffauth"
	"doula-cloud/api/internal/testdb"
)

// atClock is f with a second server whose every request reads now from
// clock.Now(ctx) (#1992), and the Owner's and the Doula's sessions seeded
// at that instant so both are live there.
func atClock(t *testing.T, f fixture, now time.Time) fixture {
	t.Helper()
	mux := http.NewServeMux()
	g := staffauth.NewGatedRouter(mux, f.db.App)
	ir := idempotency.NewRouter(g, f.db.App)
	offer.Mount(g, ir, f.db.App, f.enq)
	fixed := clock.Middleware(func() time.Time { return now })
	srv := httptest.NewServer(fixed(mux))
	t.Cleanup(srv.Close)
	f.srv = srv.URL
	f.ownerSession = authntest.SeedSessionAt(t, f.db.App, "uid-owner", now)
	f.doulaSession = authntest.SeedSessionAt(t, f.db.App, "uid-doula", now)
	return f
}

// setExpiresAt pins an Offer's expires_at to an exact instant, so a test
// can put a fixed clock either side of it.
func setExpiresAt(t *testing.T, db *testdb.DB, offerID string, at time.Time) {
	t.Helper()
	if _, err := db.Admin.ExecContext(t.Context(),
		`UPDATE engagement_offers SET expires_at = $2 WHERE id = $1`, offerID, at,
	); err != nil {
		t.Fatalf("set expires_at: %v", err)
	}
}

// expiryEdge is one microsecond, Postgres's timestamptz resolution: the
// last instant an Offer is open and the first it is not.
const expiryEdge = time.Microsecond

// TestExpireOpen_LapsesAtExpiresAtOnTheRequestClock is #1992's edge for
// expireOpen: with the request clock one microsecond before expires_at
// the Offer is still open, and at expires_at it has lapsed, with
// decided_at stamped at that same instant. The instant is fixed by
// clock.Middleware, so only a rule read from clock.Now(ctx) can pass.
func TestExpireOpen_LapsesAtExpiresAtOnTheRequestClock(t *testing.T) {
	expiresAt := time.Now().Add(time.Hour).Truncate(time.Microsecond).UTC()
	cases := []struct {
		name      string
		now       time.Time
		wantState string
	}{
		{"just-before", expiresAt.Add(-expiryEdge), stateOffered},
		{"at-expiry", expiresAt, stateExpired},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			f := newFixture(t)
			offerID := f.makeOffer(t, offerBody(f.doulaID, 45000))
			setExpiresAt(t, f.db, offerID, expiresAt)
			at := atClock(t, f, c.now)

			var listed offer.ListResponse
			decode(t, do(t, http.MethodGet, at.srv+"/api/practices/"+at.practiceID+"/offers", at.doulaSession, nil),
				http.StatusOK, &listed)
			if len(listed.Items) != 1 || listed.Items[0].State != c.wantState {
				t.Fatalf("offers = %+v, want one in state %q", listed.Items, c.wantState)
			}

			var decidedAt *time.Time
			if err := f.db.Admin.QueryRowContext(t.Context(),
				`SELECT decided_at FROM engagement_offers WHERE id = $1`, offerID,
			).Scan(&decidedAt); err != nil {
				t.Fatalf("read decided_at: %v", err)
			}
			if c.wantState == stateExpired && (decidedAt == nil || !decidedAt.Equal(c.now)) {
				t.Fatalf("decided_at = %v, want the request clock %v", decidedAt, c.now)
			}
		})
	}
}

// TestReissueOpenOffers_ReissuesOnlyWhileOpenOnTheRequestClock is #1992's
// edge for reissueOpenOffers: a second Offer to the same address rotates
// the Invitation's token, and the first Offer is re-issued a fresh code
// one microsecond before its expires_at, but not at it.
func TestReissueOpenOffers_ReissuesOnlyWhileOpenOnTheRequestClock(t *testing.T) {
	expiresAt := time.Now().Add(time.Hour).Truncate(time.Microsecond).UTC()
	cases := []struct {
		name         string
		now          time.Time
		wantReissued bool
	}{
		{"just-before", expiresAt.Add(-expiryEdge), true},
		{"at-expiry", expiresAt, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			f := newFixture(t)
			firstOffer, _, firstCode := seedEmailOffer(t, f)
			setExpiresAt(t, f.db, firstOffer, expiresAt)
			at := atClock(t, f, c.now)

			_, secondEngagement := testdb.SeedEngagement(t, f.db, f.practiceID)
			fee := int64(52000)
			decode(t, do(t, http.MethodPost,
				at.srv+"/api/practices/"+at.practiceID+"/engagements/"+secondEngagement+"/offers",
				at.ownerSession, emailOfferBody(testAddress, &fee)), http.StatusCreated, nil)

			_, code := outboxCredentials(t, f.db, firstOffer)
			if reissued := code != firstCode; reissued != c.wantReissued {
				t.Fatalf("re-issued = %v, want %v", reissued, c.wantReissued)
			}
		})
	}
}
