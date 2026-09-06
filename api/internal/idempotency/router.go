package idempotency

import (
	"database/sql"
	"fmt"
	"net/http"

	"doula-cloud/api/internal/staffauth"
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
	// Attaching marks a route registered with attaching=true: Router
	// wrapped it in staffauth.AttachingWrite, ADR-0008's write-side seam
	// that attaches the acting Doula to the Engagement the route mutates.
	// Kept as data, the same reason Replayable is, so
	// write_gate_guardrail_test.go can walk the registry instead of
	// scanning source for staffauth.AttachingWrite(.
	Attaching bool
	// Reason records why a route registered through Router.Exempt
	// deliberately runs without Wrap. Empty when Replayable is true.
	Reason string
}

// Router closes mutating-route registration until each route declares its
// idempotency stance. A 2026 architecture review found six of
// routes_practice.go's 35 mutating routes wrapped in Wrap by deliberate,
// ticket-by-ticket choice (#126, #128, #129) and the other 29 left
// undeclared -- indistinguishable at the route table from a route nobody
// got round to. Router is the write-side mirror of staffauth.GatedRouter:
// Replayable and Exempt are the only two doors a mutating route can be
// registered through, and Exempt refuses to register without a reason --
// the same "a declaration nobody had to justify is not a declaration"
// argument staffauth.GatedRouter.Exempt makes for a GET mounted outside
// Middleware.
//
// #836 moved staffauth.Middleware and Wrap themselves inside Replayable
// and Exempt: a caller used to have to write
// staffauth.Middleware(db)(idempotency.Wrap(handler)) at every mutating
// route, in the right order, and 41 call sites in routes_practice.go did.
// Router already held every fact that ordering needs -- db, at
// construction, and whether the route is Replayable or Exempt -- so
// asking a caller to reassemble it by hand at every site was ritual, not
// a real choice. A feature Mount now names only Replayable-or-Exempt,
// attaching-or-not, and (at the GatedRouter.Get calls beside it) roles.
type Router struct {
	mounter Mounter
	db      *sql.DB
	routes  []Route
}

// Mounter is the one method this package needs from whatever actually
// holds the mux -- staffauth.GatedRouter, in the BFF. An interface rather
// than an *http.ServeMux because a mutating route has two declarations to
// make, not one: its idempotency stance here, and the fact of being
// registered at all in the router that owns every route. Taking the raw
// mux would let this package mount behind that router's back, which is
// the same hole this type exists to close on its own side.
type Mounter interface {
	Write(pattern string, h http.Handler)
}

// NewRouter mounts its routes through mounter. db is the low-privilege
// connection Replayable and Exempt apply staffauth.Middleware with --
// every route this Router mounts runs downstream of it, since
// routes_practice.go's whole surface is Staff-population, Practice-scoped.
func NewRouter(mounter Mounter, db *sql.DB) *Router {
	return &Router{mounter: mounter, db: db}
}

// Replayable declares pattern replayable and mounts h behind
// staffauth.Middleware and idempotency.Wrap, which Replayable applies
// itself -- a caller passes the bare handler. attaching wraps h in
// staffauth.AttachingWrite first (ADR-0008's write-side seam), for a
// mutating route under an Engagement.
func (rt *Router) Replayable(pattern string, attaching bool, h http.Handler) {
	wrapped := Wrap(h)
	if attaching {
		wrapped = staffauth.AttachingWrite(wrapped)
	}
	rt.routes = append(rt.routes, Route{Pattern: pattern, Replayable: true, Attaching: attaching})
	rt.mounter.Write(pattern, staffauth.Middleware(rt.db)(wrapped))
}

// Exempt declares pattern deliberately unwrapped, for reason, and mounts
// h behind staffauth.Middleware, which Exempt applies itself. A route
// earns this by being safe to repeat as-is -- a state-guarded transition,
// a full-replace PUT, a unique-constraint-guarded create -- or by moving
// no money and sending no notification worth deduplicating. reason must
// be non-empty, for the same purpose staffauth.GatedRouter.Exempt
// requires one. attaching wraps h in staffauth.AttachingWrite first, the
// same as Replayable.
func (rt *Router) Exempt(pattern, reason string, attaching bool, h http.Handler) {
	if reason == "" {
		panic(fmt.Sprintf("idempotency: Router.Exempt(%q): no reason given -- a mutating route left unwrapped must say why", pattern))
	}
	wrapped := h
	if attaching {
		wrapped = staffauth.AttachingWrite(wrapped)
	}
	rt.routes = append(rt.routes, Route{Pattern: pattern, Reason: reason, Attaching: attaching})
	rt.mounter.Write(pattern, staffauth.Middleware(rt.db)(wrapped))
}

// Routes returns the registry of every mutating route this router knows
// about -- the table a guardrail-shaped test walks.
func (rt *Router) Routes() []Route {
	return rt.routes
}
