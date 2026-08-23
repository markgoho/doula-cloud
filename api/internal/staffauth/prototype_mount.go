// PROTOTYPE -- throwaway. Answers #231: how is a read rule expressed in Go
// so a new read endpoint is closed until someone opens it?
//
// SHAPE A: a mount seam. GET routes are registered through GatedRouter
// instead of the raw http.ServeMux, and GatedRouter.Get panics at startup
// if no role declaration is given. The declaration is DATA (a []string),
// so a test can walk the registry -- this is the piece that answers the
// AC's "guardrail test in the shape of rlsguardrail" question for the
// whole-endpoint case.
//
// It does NOT solve the Contract "scope without money" case: Roles only
// says yes/no to the whole response. See prototype_reader.go for Shape B,
// which the Contract case forces.
package staffauth

import (
	"database/sql"
	"fmt"
	"net/http"
)

// GatedRoute is one GET route's read declaration, kept as data so a test
// can enumerate every mounted read endpoint without starting a server.
type GatedRoute struct {
	Pattern string   // e.g. "GET /api/practices/{practiceId}/billing"
	Roles   []string // roles allowed to reach this endpoint at all; empty means "any Staff member" and must be explicit (AnyStaff), never a bare nil
}

// AnyStaff is the explicit opt-out for an endpoint every Staff member may
// read (Templates, Engagements) -- a route can't be gateless by omission,
// only by naming this sentinel, so a table-driven test can tell "declared
// open on purpose" apart from "nobody declared anything."
var AnyStaff = []string{"*"}

// GatedRouter wraps an http.ServeMux so GET registration is only possible
// through Get, which requires a role declaration. The underlying mux is
// unexported: main.go cannot reach around this type to mount a bare GET,
// which is what makes "closed until opened" structural rather than a
// convention someone has to remember.
type GatedRouter struct {
	mux    *http.ServeMux
	db     *sql.DB
	routes []GatedRoute // registry a test walks; see prototype_mount_test.go
}

// NewGatedRouter wraps mux for GET registration. db is the low-privilege
// connection Middleware needs -- Get applies Middleware itself (rather
// than requiring the caller to, as main.go does today) so the role check
// always runs downstream of it, matching requireAnyRole's precondition.
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

// Routes returns the registry of every GET mounted through this router --
// the table a guardrail-shaped test walks.
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
			http.Error(w, MsgInternalError, http.StatusInternalServerError)
			return
		}
		staffID, _ := StaffID(r.Context())
		practiceID, _ := PracticeID(r.Context())

		callerRoles, err := Roles(r.Context(), tx, practiceID, staffID)
		if err != nil {
			http.Error(w, MsgInternalError, http.StatusInternalServerError)
			return
		}
		for _, want := range roles {
			for _, have := range callerRoles {
				if want == have {
					h.ServeHTTP(w, r)
					return
				}
			}
		}
		http.Error(w, "not permitted to read this", http.StatusForbidden)
	})
}
