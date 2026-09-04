package staffauth

import (
	"database/sql"
	"fmt"
	"net/http"
	"slices"
	"strings"
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
	// Write marks a route mounted under any verb but GET. ADR-0008's read
	// table is about reads, so a write carries no role declaration here --
	// what it carries is the fact of having been registered through this
	// router at all, which is what lets a test enumerate the write surface
	// rather than grep for it.
	Write bool
}

// AnyStaff is the explicit opt-out for an endpoint every Staff member may
// read (Templates, Engagements) -- a route can't be gateless by omission,
// only by naming this sentinel, so a table-driven test can tell "declared
// open on purpose" apart from "nobody declared anything".
var AnyStaff = []string{"*"}

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
	method, path, found := strings.Cut(pattern, " ")
	if !found {
		panic(fmt.Sprintf("staffauth: GatedRouter.Write(%q): pattern must name its method, e.g. \"POST /api/session\"", pattern))
	}
	if method == http.MethodGet {
		panic(fmt.Sprintf("staffauth: GatedRouter.Write(%q): a GET belongs at Get (with roles) or OpenGet (with a reason), so ADR-0008's read table can see it", pattern))
	}
	g.routes = append(g.routes, GatedRoute{Method: method, Pattern: path, Write: true})
	g.mux.Handle(pattern, h)
}

// Routes returns the registry of every route this router mounted -- gated
// GETs through Get, ungated ones through OpenGet, and every write through
// Write -- the table a guardrail-shaped test walks.
func (g *GatedRouter) Routes() []GatedRoute {
	return g.routes
}

// requireAnyRole 403s unless the caller holds at least one of roles (or
// roles is AnyStaff). Must run downstream of Middleware.
func requireAnyRole(roles []string, h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if len(roles) == 1 && roles[0] == "*" {
			h.ServeHTTP(w, r)
			return
		}
		tx, has := Tx(r.Context())
		if !has {
			// coverage:ignore reason: Middleware always sets a tx before this handler runs
			http.Error(w, MsgInternalError, http.StatusInternalServerError)
			return
		}
		staffID, _ := StaffID(r.Context())
		practiceID, _ := PracticeID(r.Context())

		callerRoles, err := Roles(r.Context(), tx, practiceID, staffID)
		if err != nil {
			// coverage:ignore reason: DB query failure, not exercised by unit tests
			http.Error(w, MsgInternalError, http.StatusInternalServerError)
			return
		}
		for _, want := range roles {
			if slices.Contains(callerRoles, want) {
				h.ServeHTTP(w, r)
				return
			}
		}
		http.Error(w, "not permitted to read this", http.StatusForbidden)
	})
}
