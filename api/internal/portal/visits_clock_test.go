package portal_test

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"doula-cloud/api/internal/authntest"
	"doula-cloud/api/internal/clock"
	"doula-cloud/api/internal/portal"
	"doula-cloud/api/internal/staffauth"
	"doula-cloud/api/internal/testdb"
)

// TestVisitsHandler_HasHappenedAtScheduledAtOnTheRequestClock is #1992's
// edge for listPortalVisits: a Visit one microsecond in the request
// clock's future has not happened, and one scheduled at that very instant
// has. The clock is fixed by clock.Middleware, so only a rule read from
// clock.Now(ctx) can pass.
func TestVisitsHandler_HasHappenedAtScheduledAtOnTheRequestClock(t *testing.T) {
	db := testdb.New(t)
	const identityUID = "portal-visits-clock-edge"
	engagementID, doulaID := seedEngagementForVisits(t, db, identityUID, "Clock Edge Practice", "Priya Raman")
	now := time.Now().Add(time.Hour).Truncate(time.Microsecond).UTC()
	atNow := seedPortalVisit(t, db, engagementID, doulaID, now)
	justAfter := seedPortalVisit(t, db, engagementID, doulaID, now.Add(time.Microsecond))

	mux := http.NewServeMux()
	portal.Mount(staffauth.NewGatedRouter(mux, db.App), db.App)
	srv := httptest.NewServer(clock.Middleware(func() time.Time { return now })(mux))
	defer srv.Close()
	session := authntest.SeedSessionAt(t, db.App, testdb.PortalUID(identityUID), now)

	resp := authedActivityGet(t, session, srv.URL+"/api/portal/engagements/"+engagementID+"/visits")
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}

	happened := map[string]bool{}
	for _, v := range decodeVisits(t, resp).Items {
		happened[v.VisitID] = v.HasHappened
	}
	if len(happened) != 2 || happened[justAfter] || !happened[atNow] {
		t.Fatalf("hasHappened = %v, want %q false and %q true", happened, justAfter, atNow)
	}
}
