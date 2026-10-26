package idempotency_test

import (
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"doula-cloud/api/internal/testdb"
)

// storedTTL is idempotency's unexported ttl, restated for the edges below.
const storedTTL = 48 * time.Hour

// ttlEdges are the ages either side of the TTL, and the TTL itself. live is
// what both the lookup and the sweep must agree on at sweepNow: a row
// exactly ttl old is still live, the first instant past it is not.
var ttlEdges = []struct {
	name string
	age  time.Duration
	live bool
}{
	{"just-inside", storedTTL - time.Second, true},
	{"at-ttl", storedTTL, true},
	{"just-outside", storedTTL + time.Second, false},
}

// TestTTL_LookupAndSweepAgreeOnAFixedClock is #1991's AC: with every
// request's clock fixed at sweepNow, a row is replayed just before the TTL
// and not just after it, and the sweep deletes exactly the rows the lookup
// treats as expired. Both read clock.Now(ctx), not the database's now(),
// so moving the clock moves them together.
//
// The lookup and the sweep each get their own copy of every edge: a POST
// against an expired row re-runs the handler and refreshes that row to
// sweepNow, after which it no longer says anything about the sweep.
func TestTTL_LookupAndSweepAgreeOnAFixedClock(t *testing.T) {
	db := testdb.New(t)
	practiceID, staffID := testdb.SeedStaffAtNewPractice(t, db, "ttl-edge-owner", []string{doulaRole}, "employee")
	for _, e := range ttlEdges {
		seedKeyAt(t, db, "lookup-"+e.name, practiceID, staffID, "", e.age)
		seedKeyAt(t, db, "sweep-"+e.name, practiceID, staffID, "", e.age)
	}

	var calls int
	srv, session := newIdempotencyServerAt(t, db, "ttl-edge-owner", &calls, http.StatusCreated, sweepNow)
	defer srv.Close()

	replayedByLookup := map[string]bool{}
	for _, e := range ttlEdges {
		before := calls
		resp := postWidget(t, srv, session, practiceID, "lookup-"+e.name)
		body, err := io.ReadAll(resp.Body)
		_ = resp.Body.Close()
		if err != nil {
			t.Fatalf("%s: read body: %v", e.name, err)
		}
		replayed := calls == before
		if replayed != strings.Contains(string(body), "her own words") {
			t.Fatalf("%s: handler calls %d -> %d but body %q", e.name, before, calls, body)
		}
		replayedByLookup[e.name] = replayed
	}

	// The sweep runs after the lookups, so the lookup above read every
	// seeded row and not a gap the sweep left behind.
	if err := runSweep(t, db, true); err != nil {
		t.Fatalf("sweep: %v", err)
	}
	kept := map[string]bool{}
	for _, k := range remainingKeys(t, db) {
		kept[k] = true
	}

	for _, e := range ttlEdges {
		if replayed := replayedByLookup[e.name]; replayed != e.live {
			t.Errorf("%s (%v old): replayed = %v, want %v", e.name, e.age, replayed, e.live)
		}
		if kept["sweep-"+e.name] != e.live {
			t.Errorf("%s (%v old): kept by the sweep = %v, want %v", e.name, e.age, kept["sweep-"+e.name], e.live)
		}
	}
}
