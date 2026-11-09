package portal_test

import (
	"testing"
	"time"

	"doula-cloud/api/internal/fixture"
)

// TestVisitsHandler_HasHappenedAtScheduledAtOnTheRequestClock is #1992's
// edge for listPortalVisits: a Visit one microsecond in the request
// clock's future has not happened, and one scheduled at that very instant
// has. The clock is fixed by clock.Middleware, so only a rule read from
// clock.Now(ctx) can pass.
func TestVisitsHandler_HasHappenedAtScheduledAtOnTheRequestClock(t *testing.T) {
	now := time.Now().Add(time.Hour).Truncate(time.Microsecond).UTC()
	e := newEnv(t, fixture.At(now))
	atNow := e.visit(now)
	justAfter := e.visit(now.Add(time.Microsecond))

	happened := map[string]bool{}
	for _, v := range e.visitsPage("").Items {
		happened[v.VisitID] = v.HasHappened
	}
	if len(happened) != 2 || happened[justAfter] || !happened[atNow] {
		t.Fatalf("hasHappened = %v, want %q false and %q true", happened, justAfter, atNow)
	}
}
