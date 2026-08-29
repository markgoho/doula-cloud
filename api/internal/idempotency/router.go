package idempotency

import (
	"fmt"
	"net/http"
)

// Route is one mutating route's idempotency declaration, kept as data so a
// test can enumerate every mounted mutating endpoint without starting a
// server -- the write-side mirror of staffauth.GatedRoute.
type Route struct {
	Pattern string // e.g. "POST /api/practices/{practiceId}/clients"
	// Replayable marks a route registered through Router.Replayable: Wrap
	// sits somewhere in its handler chain, so a repeated Idempotency-Key
	// header replays the first response instead of re-running the
	// mutation.
	Replayable bool
	// Reason records why a route registered through Router.Exempt
	// deliberately runs without Wrap. Empty when Replayable is true.
	Reason string
}

// Router closes mutating-route registration in routes_practice.go until
// each route declares its idempotency stance. A 2026 architecture review
// found six of that file's 35 mutating routes wrapped in Wrap by
// deliberate, ticket-by-ticket choice (#126, #128, #129) and the other 29
// left undeclared -- indistinguishable at the route table from a route
// nobody got round to. Router is the write-side mirror of
// staffauth.GatedRouter: Replayable and Exempt are the only two doors a
// mutating route can be registered through, and Exempt refuses to
// register without a reason -- the same "a declaration nobody had to
// justify is not a declaration" argument staffauth.GatedRouter.Exempt
// makes for a GET mounted outside Middleware.
type Router struct {
	mux    *http.ServeMux
	routes []Route
}

// NewRouter wraps mux for mutating-route registration.
func NewRouter(mux *http.ServeMux) *Router {
	return &Router{mux: mux}
}

// Replayable declares pattern replayable and mounts h. h must already
// carry Wrap somewhere in its chain -- this package's own guardrail test
// (idempotency_guardrail_test.go in the main package) checks that against
// routes_practice.go's source, since an http.Handler value carries no way
// to ask "were you built with Wrap?" at runtime.
func (rt *Router) Replayable(pattern string, h http.Handler) {
	rt.routes = append(rt.routes, Route{Pattern: pattern, Replayable: true})
	rt.mux.Handle(pattern, h)
}

// Exempt declares pattern deliberately unwrapped, for reason, and mounts
// h. A route earns this by being safe to repeat as-is -- a state-guarded
// transition, a full-replace PUT, a unique-constraint-guarded create -- or
// by moving no money and sending no notification worth deduplicating.
// reason must be non-empty, for the same purpose staffauth.GatedRouter.Exempt
// requires one.
func (rt *Router) Exempt(pattern, reason string, h http.Handler) {
	if reason == "" {
		panic(fmt.Sprintf("idempotency: Router.Exempt(%q): no reason given -- a mutating route left unwrapped must say why", pattern))
	}
	rt.routes = append(rt.routes, Route{Pattern: pattern, Reason: reason})
	rt.mux.Handle(pattern, h)
}

// Routes returns the registry of every mutating route this router knows
// about -- the table a guardrail-shaped test walks.
func (rt *Router) Routes() []Route {
	return rt.routes
}
