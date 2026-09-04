package idempotency_test

import (
	"net/http"
	"testing"

	"doula-cloud/api/internal/idempotency"
)

// discardMounter stands in for staffauth.GatedRouter, which is what
// actually holds the mux in the BFF. These tests are about the registry,
// not about routing, so it keeps nothing.
type discardMounter struct{}

func (discardMounter) Write(string, http.Handler) {}

// TestRouter_ExemptPanicsWithoutAReason mirrors
// staffauth.TestGatedRouter_ExemptPanicsWithoutAReason: an exemption
// nobody had to justify is not a declaration, so Exempt refuses one.
func TestRouter_ExemptPanicsWithoutAReason(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Fatal("expected Exempt to panic on an empty reason, it did not")
		}
	}()
	rt := idempotency.NewRouter(discardMounter{})
	rt.Exempt("/api/practices/{practiceId}/website", "", http.NotFoundHandler())
}

// TestRouter_RegistryIsWalkable proves every mutating route registered
// through Router carries a declaration -- either Replayable or a non-empty
// Reason -- the table a guardrail-shaped test walks against the real route
// table routes() builds.
func TestRouter_RegistryIsWalkable(t *testing.T) {
	rt := idempotency.NewRouter(discardMounter{})
	rt.Replayable("POST /api/practices/{practiceId}/clients", http.NotFoundHandler())
	rt.Exempt("PUT /api/practices/{practiceId}/website", "PUT replaces the declaration wholesale", http.NotFoundHandler())

	routes := rt.Routes()
	if len(routes) != 2 {
		t.Fatalf("Routes() = %d entries, want 2", len(routes))
	}
	for _, route := range routes {
		if !route.Replayable && route.Reason == "" {
			t.Fatalf("route %q is neither Replayable nor carries an exemption reason", route.Pattern)
		}
		if route.Replayable && route.Reason != "" {
			t.Fatalf("route %q is Replayable but also carries a Reason, want it empty", route.Pattern)
		}
	}
}

// TestRouter_ExemptWithoutReplayableAreDistinguishable confirms Replayable
// and Exempt entries are told apart by the Replayable flag, not by
// inspecting Reason -- the same shape staffauth.GatedRoute's Exempt field
// gives GatedRoute.
func TestRouter_ExemptWithoutReplayableAreDistinguishable(t *testing.T) {
	rt := idempotency.NewRouter(discardMounter{})
	rt.Exempt("DELETE /api/practices/{practiceId}/push-subscriptions", "delete; a retry deletes nothing further", http.NotFoundHandler())

	routes := rt.Routes()
	if len(routes) != 1 {
		t.Fatalf("Routes() = %d entries, want 1", len(routes))
	}
	if routes[0].Replayable {
		t.Fatal("Exempt entry reports Replayable = true, want false")
	}
	if routes[0].Reason == "" {
		t.Fatal("Exempt entry carries no reason")
	}
}
