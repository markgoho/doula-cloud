package staffauth

import (
	"database/sql"
	"fmt"
	"net/http"
	"slices"
)

// GatedRoute is one GET route's read declaration, kept as data so a test
// can enumerate every mounted read endpoint without starting a server.
type GatedRoute struct {
	Pattern string   // the path passed to Get, e.g. "/api/practices/{practiceId}/billing" -- no leading "GET "
	Roles   []string // roles allowed to reach this endpoint at all; empty means "any Staff member" and must be explicit (AnyStaff), never a bare nil
	// Exempt marks a GET that is deliberately mounted outside
	// staffauth.Middleware altogether, so Roles has nothing to say about
	// it. Reason records why, in one line. See Exempt below.
	Exempt bool
	Reason string
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
// unexported so main.go cannot reach around this type to mount a bare
// GET, which is what makes "closed until opened" structural rather than
// a convention someone has to remember.
type GatedRouter struct {
	mux    *http.ServeMux
	db     *sql.DB
	routes []GatedRoute // registry a test walks; see gate_test.go
}

// NewGatedRouter wraps mux for GET registration. db is the low-privilege
// connection Middleware needs -- Get applies Middleware itself (rather
// than requiring the caller to, as every other verb does) so the role
// check always runs downstream of it, matching requireAnyRole's
// precondition.
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
	g.routes = append(g.routes, GatedRoute{Pattern: pattern, Roles: roles})
	g.mux.Handle("GET "+pattern, Middleware(g.db)(requireAnyRole(roles, h)))
}

// Exempt records a GET that is mounted outside staffauth.Middleware
// entirely -- a bootstrap route with no session yet, a clientauth-scoped
// portal route, or ADR-0008's token-authenticated pre-account Offer read
// (#317), which is authenticated by an Invitation token and an emailed
// code rather than by a session.
//
// GatedRouter cannot mount such a route itself: Get applies Middleware,
// which is exactly what these routes have no session for. That leaves
// them unable to be ungated-by-omission the way a forgotten role
// declaration is caught, which is why ADR-0008 requires them declared
// exempt *by name, in this registry* -- so the guardrail test walking
// Routes() sees a deliberate entry with a reason rather than an absence.
// reason must be non-empty for the same purpose AnyStaff exists: a
// declaration nobody had to justify is not a declaration.
func (g *GatedRouter) Exempt(pattern, reason string) {
	if reason == "" {
		panic(fmt.Sprintf("staffauth: GatedRouter.Exempt(%q): no reason given -- an ungated GET must say why it is ungated", pattern))
	}
	g.routes = append(g.routes, GatedRoute{Pattern: pattern, Exempt: true, Reason: reason})
}

// Routes returns the registry of every GET this router knows about --
// those it mounted through Get, and those declared exempt through Exempt
// -- the table a guardrail-shaped test walks.
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
