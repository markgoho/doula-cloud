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
	// Roles is set for a route registered through ExemptGated or
	// ReplayableGated: the role declaration staffauth.GatedRouter.GatedWrite
	// enforces at the mount, ADR-0008's write-side mirror of a GET's Roles
	// (#970, #990). Empty for a route registered through Replayable or
	// Exempt -- their write carries no role declaration, by design.
	Roles []string
}

// Router closes mutating-route registration until each route declares its
// idempotency stance. A 2026 architecture review found six of
// routes_practice.go's 35 mutating routes wrapped in Wrap by deliberate,
// ticket-by-ticket choice (#126, #128, #129) and the other 29 left
// undeclared -- indistinguishable at the route table from a route nobody
// got round to. Router is the write-side mirror of staffauth.GatedRouter:
// Replayable and Exempt are the two base doors a mutating route can be
// registered through, each with a role-gated variant (ExemptGated, #970;
// ReplayableGated, #990) for the rare write whose rule is not reach
// alone. Exempt (and ExemptGated) refuse to register without a reason --
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
	// GatedWrite is staffauth.GatedRouter's role-checked write door,
	// reached through Router.ExemptGated (#970's Contract void, refused
	// to every Doula regardless of reach).
	GatedWrite(pattern string, roles []string, h http.Handler)
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

// ReplayableGated is Replayable, plus a role declaration ADR-0008's write
// side can enforce at the mount, the same pairing ExemptGated makes for
// Exempt (#990: payments.PostManualPaymentHandler is money-creating, so
// it stays Replayable, but its Owner-or-Admin rule is not reach alone --
// the same gap #970 closed for Contract void). roles is checked by
// staffauth.GatedRouter itself, panicking on an empty list, the same
// guarantee ExemptGated's role list already carries.
//
// wrapped is built Wrap-then-AttachingWrite, the same order Replayable
// itself uses, so a route registered attaching=true here still checks
// reach before replay -- but GatedWrite's own role check runs outside
// both, the same "role first" order ExemptGated documents, since GatedWrite
// applies staffauth.Middleware and requireAnyRole around whatever this
// passes it.
func (rt *Router) ReplayableGated(pattern string, attaching bool, roles []string, h http.Handler) {
	wrapped := Wrap(h)
	if attaching {
		wrapped = staffauth.AttachingWrite(wrapped)
	}
	rt.routes = append(rt.routes, Route{Pattern: pattern, Replayable: true, Attaching: attaching, Roles: roles})
	rt.mounter.GatedWrite(pattern, roles, wrapped)
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

// ExemptGated is Exempt, plus a role declaration ADR-0008's write side
// can enforce at the mount (#970): roles is checked by
// staffauth.GatedRouter itself, panicking on an empty list rather than
// admitting a route with none, the same guarantee Get's role list already
// carries. Reserved for
// a write whose rule is not reach alone -- a Contract void, refused to
// every Doula no matter what she is attached to -- since an ordinary
// write's role-free Exempt already covers "may this caller reach this
// Engagement at all" through attaching.
//
// GatedWrite applies staffauth.Middleware itself (the same reason Get
// does), so, unlike Exempt, this must not wrap wrapped in Middleware
// again -- passing it bare to mounter.GatedWrite is deliberate, not an
// oversight.
func (rt *Router) ExemptGated(pattern, reason string, attaching bool, roles []string, h http.Handler) {
	if reason == "" {
		panic(fmt.Sprintf("idempotency: Router.ExemptGated(%q): no reason given -- a mutating route left unwrapped must say why", pattern))
	}
	wrapped := h
	if attaching {
		wrapped = staffauth.AttachingWrite(wrapped)
	}
	rt.routes = append(rt.routes, Route{Pattern: pattern, Reason: reason, Attaching: attaching, Roles: roles})
	rt.mounter.GatedWrite(pattern, roles, wrapped)
}

// Routes returns the registry of every mutating route this router knows
// about -- the table a guardrail-shaped test walks.
func (rt *Router) Routes() []Route {
	return rt.routes
}
