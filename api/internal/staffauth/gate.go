package staffauth

import (
	"database/sql"
	"fmt"
	"net/http"
	"slices"
	"strings"

	"doula-cloud/api/internal/apierr"
)

// GatedRoute is one route's declaration, kept as data so a test can
// enumerate every mounted endpoint without starting a server.
type GatedRoute struct {
	Method  string   // "GET", "POST", ... -- the verb the route is mounted under
	Pattern string   // the path passed to Get, e.g. "/api/practices/{practiceId}/billing" -- no leading method
	Roles   []string // roles allowed to reach this endpoint at all; empty means "any Staff member" and must be explicit (AnyStaff), never a bare nil
	// Exempt marks a GET that is deliberately mounted outside
	// staffauth.Middleware altogether, so Roles has nothing to say about
	// it. Reason records why, in one line. See OpenGet below.
	Exempt bool
	Reason string
	// Write marks a route mounted under any verb but GET, through either
	// Write or GatedWrite. An ordinary Write route carries no role
	// declaration, by design: ADR-0008's read table is about reads, and
	// most of the write surface is gated on reach alone (AttachingWrite),
	// not role. A GatedWrite route is the exception -- one whose rule is
	// not fully described by reach (#970's Contract void, refused to every
	// Doula regardless of attachment) -- and carries its own non-empty
	// Roles, checked the same way a GET's is.
	Write bool
}

// AnyStaff is the explicit opt-out for an endpoint every Staff member may
// read (Templates, Engagements) -- a route can't be gateless by omission,
// only by naming this sentinel, so a table-driven test can tell "declared
// open on purpose" apart from "nobody declared anything".
var AnyStaff = []string{"*"}

// roleOwner and roleAdmin are this package's own two role literals, named
// once so golangci-lint's package-wide goconst check sees one definition
// rather than the repeats scattered across gate.go, reader.go, roles.go
// and membership.go.
const (
	roleOwner = "owner"
	roleAdmin = "admin"
)

// OwnerAndAdmin is the role declaration for every GatedRouter route
// ADR-0008's read table admits to Owner and Admin only (Staff roster,
// Credit balance and ledger, Contract's money-bearing Signed PDF and
// Invoice history, and Stripe Connect state -- #267 put the last of
// those here, beside the Invoice history it is the payment rail for).
// Moved here from routes_practice.go by #836: a
// feature Mount declares its own role table, and this vocabulary belongs
// to ADR-0008, not to main.
var OwnerAndAdmin = []string{roleOwner, roleAdmin}

// OwnerOnly is the role declaration for a GatedRouter route only a
// Practice Owner may read (#606's MFA impact count). Stripe Connect
// status used to be the other member and no longer is: #267 gave it to
// the Admin as well, so it reads through OwnerAndAdmin above.
var OwnerOnly = []string{roleOwner}

// GatedRouter closes GET registration until a route explicitly declares
// who may read it. #231 chose this mount-seam mechanism (proved on branch
// prototype/231-read-gate) to answer ADR-0008's read table: every GET
// behind Middleware panics at startup if it isn't registered here with a
// non-empty role declaration. It wraps an http.ServeMux so GET
// registration is only possible through Get; the underlying mux is
// unexported so nothing can reach around this type to mount a bare GET,
// which is what makes "closed until opened" structural rather than a
// convention someone has to remember.
//
// That claim used to hold only for what came *through* this type. The raw
// *http.ServeMux travelled beside it into the route files, so a route
// could skip the seam entirely by calling mux.Handle -- and the only thing
// standing in the way was a test that regexed this repository's own Go
// source for direct registrations. The router now carries a verb for every
// shape a route can take (Get, OpenGet, Write), the route files are handed
// this and never the mux, and the bypass is a compile error rather than
// something a test has to go looking for.
type GatedRouter struct {
	mux    *http.ServeMux
	db     *sql.DB
	routes []GatedRoute // registry a test walks; see gate_test.go
}

// NewGatedRouter wraps mux for route registration. db is the
// low-privilege connection Middleware needs -- Get applies Middleware
// itself (rather than requiring the caller to, as every other verb does)
// so the role check always runs downstream of it, matching
// requireAnyRole's precondition.
//
// The caller must not keep its own reference to mux afterwards: handing it
// over is what makes this the only door.
func NewGatedRouter(mux *http.ServeMux, db *sql.DB) *GatedRouter {
	return &GatedRouter{mux: mux, db: db}
}

// Get mounts a GET handler behind Middleware and a role check. roles must
// be non-empty -- pass AnyStaff to declare the endpoint open on purpose.
// Panics at startup (main() calling this directly, not per-request) if
// roles is empty, so a forgotten declaration fails the moment the binary
// starts rather than silently serving every Staff member.
func (g *GatedRouter) Get(pattern string, roles []string, h http.Handler) {
	if len(roles) == 0 {
		panic(fmt.Sprintf("staffauth: GatedRouter.Get(%q): no roles declared -- pass staffauth.AnyStaff to open this endpoint on purpose", pattern))
	}
	g.routes = append(g.routes, GatedRoute{Method: http.MethodGet, Pattern: pattern, Roles: roles})
	g.mux.Handle("GET "+pattern, Middleware(g.db)(requireAnyRole(roles, h)))
}

// OpenGet mounts a GET that sits outside Middleware entirely, for reason.
//
// It replaces the pair this used to take: an Exempt call recording the
// declaration and a separate mux.Handle making the mount, which could
// disagree with each other -- a route declared exempt and never mounted,
// or mounted and never declared. One call does both, so the registry
// describes what is actually served.
func (g *GatedRouter) OpenGet(pattern, reason string, h http.Handler) {
	if reason == "" {
		panic(fmt.Sprintf("staffauth: GatedRouter.OpenGet(%q): no reason given -- an ungated GET must say why it is ungated", pattern))
	}
	g.routes = append(g.routes, GatedRoute{Method: http.MethodGet, Pattern: pattern, Exempt: true, Reason: reason})
	g.mux.Handle("GET "+pattern, h)
}

// Write mounts a route under any verb but GET. pattern carries its own
// method, the way http.ServeMux spells it: "POST /api/session".
//
// No role declaration and no reason, deliberately. ADR-0008's read table
// is about reads, and the write side has its own seams -- idempotency
// stance through idempotency.Router, and an Engagement write's attachment
// through AttachingWrite. What this verb adds is that a write is
// registered *somewhere* a test can enumerate, rather than being visible
// only to a regex over this repository's source.
//
// Panics at startup on a GET, which belongs at Get or OpenGet where the
// read table can see it.
func (g *GatedRouter) Write(pattern string, h http.Handler) {
	method, path := g.cutWritePattern("Write", pattern)
	g.routes = append(g.routes, GatedRoute{Method: method, Pattern: path, Write: true})
	g.mux.Handle(pattern, h)
}

// GatedWrite mounts a route under any verb but GET, behind Middleware and
// a role check, the way Get gates a read -- for the rare write whose rule
// is not reach alone. #970 is the first write to declare its role at the
// mount seam this way: a Contract void must refuse every Doula, employee
// or contractor, no matter what she is attached to, which AttachingWrite's
// reach test cannot express (it asks "can she reach this Engagement",
// never "is this act hers to do"). A role-gated write is not new by
// itself -- payments.PutBillingModeHandler and its by-hand Invoice
// void/write-off already check staffauth.RequireOwner/RequireOwnerOrAdmin
// in-handler, invisible to any startup guardrail the same way Contract
// void was -- what is new here is the mount declaring it, the way GET
// already does. roles
// must be non-empty -- pass AnyStaff to declare the write open to any
// Staff member who reaches it, the same opt-out Get uses. Panics at
// startup if roles is empty, so a forgotten declaration fails the binary
// before it serves a single request, exactly as Get's own panic does.
//
// Most of the write surface stays on the plain Write above: ADR-0008's
// ordinary write table is a reach question (which Engagements), not a
// role one, and declaring roles for every write that needs none would
// only invite a role list that repeats what AttachingWrite already
// checks. Reach a GatedWrite through idempotency.Router.ExemptGated, the
// role-declaring mirror of Exempt (Write's own door).
//
// The role check runs before h, so a route registered attaching=true
// through ExemptGated checks role first, then AttachingWrite's reach.
// That order is right for #970's only case today (an Owner or Admin
// reaches every Engagement, so refusing her by role never hides an
// Engagement from her that the reach test would have shown) -- but it is
// an order: a
// future GatedWrite whose role list is narrower than its reach population
// should think about which refusal a caller meets first, a 403 that
// confirms the Engagement exists versus the 404 AttachingWrite gives an
// unattached contractor.
func (g *GatedRouter) GatedWrite(pattern string, roles []string, h http.Handler) {
	method, path := g.cutWritePattern("GatedWrite", pattern)
	if len(roles) == 0 {
		panic(fmt.Sprintf("staffauth: GatedRouter.GatedWrite(%q): no roles declared -- pass staffauth.AnyStaff to open this write to any Staff member on purpose", pattern))
	}
	g.routes = append(g.routes, GatedRoute{Method: method, Pattern: path, Write: true, Roles: roles})
	g.mux.Handle(pattern, Middleware(g.db)(requireAnyRole(roles, h)))
}

// cutWritePattern is Write and GatedWrite's shared pattern parse: split
// "METHOD /path" the way http.ServeMux spells it, and refuse a GET, which
// belongs at Get or OpenGet where ADR-0008's read table can see it.
func (g *GatedRouter) cutWritePattern(caller, pattern string) (method, path string) {
	method, path, found := strings.Cut(pattern, " ")
	if !found {
		panic(fmt.Sprintf("staffauth: GatedRouter.%s(%q): pattern must name its method, e.g. \"POST /api/session\"", caller, pattern))
	}
	if method == http.MethodGet {
		panic(fmt.Sprintf("staffauth: GatedRouter.%s(%q): a GET belongs at Get (with roles) or OpenGet (with a reason), so ADR-0008's read table can see it", caller, pattern))
	}
	return method, path
}

// Routes returns the registry of every route this router mounted -- gated
// GETs through Get, ungated ones through OpenGet, and every write through
// Write -- the table a guardrail-shaped test walks.
func (g *GatedRouter) Routes() []GatedRoute {
	return g.routes
}

// requireAnyRole 403s unless the caller holds at least one of roles (or
// roles is AnyStaff). Must run downstream of Middleware. Zero-query: it
// reads the Reader Middleware already resolved rather than querying
// practice_memberships again. Shared by Get and GatedWrite, so its
// refusal names no verb -- "read" would be wrong the half of the time
// this guards a write.
func requireAnyRole(roles []string, h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if len(roles) == 1 && roles[0] == "*" {
			h.ServeHTTP(w, r)
			return
		}
		reader, has := ReaderFrom(r.Context())
		if !has {
			// coverage:ignore reason: Middleware always places a Reader on context before this handler runs
			apierr.WriteError(w, apierr.MsgInternalError, http.StatusInternalServerError)
			return
		}
		if slices.ContainsFunc(roles, reader.Has) {
			h.ServeHTTP(w, r)
			return
		}
		apierr.WriteError(w, "not permitted to do this", http.StatusForbidden)
	})
}
