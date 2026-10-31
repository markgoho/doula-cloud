package main

import (
	"database/sql"
	"net/http"

	"doula-cloud/api/internal/apierr"
	"doula-cloud/api/internal/clock"
	"doula-cloud/api/internal/csrf"
)

// Deps is everything doula-ops's route table needs.
type Deps struct {
	DB       *sql.DB
	Now      clock.Clock
	Verifier Verifier
	// AcceptEmulatorFactor admits the Auth emulator's phone factor in
	// place of TOTP. config sets it only while the emulator is the
	// identity provider.
	AcceptEmulatorFactor bool
	// Origins are the front-end origins csrf.Wrap treats as same-site.
	Origins []string
}

// router mounts a route and records it, so routes() can hand back the
// list a guardrail test reads.
type router struct {
	mux    *http.ServeMux
	routes []string
}

func (r *router) handle(method, path string, h http.Handler) {
	pattern := method + " " + path
	r.mux.Handle(pattern, h)
	r.routes = append(r.routes, pattern)
}

// routes builds doula-ops's route table and returns every route it
// mounted as "METHOD /path". There is no signup route: an operator login
// is made by hand in the tenant (ADR-0062).
func routes(d Deps) (http.Handler, []string) {
	r := &router{mux: http.NewServeMux()}
	r.handle(http.MethodGet, "/api/health", health(d.DB))
	r.handle(http.MethodPost, "/api/session", signIn(d))
	r.handle(http.MethodGet, "/api/session", requireSession(d.DB, session()))
	r.handle(http.MethodDelete, "/api/session", requireSession(d.DB, signOut(d.DB)))
	return clock.Middleware(d.Now)(csrf.Wrap(d.Origins, r.mux)), r.routes
}

// health answers once doula-ops can reach Postgres as its own login.
func health(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if err := db.PingContext(r.Context()); err != nil {
			apierr.WriteError(w, "The database cannot be reached.", http.StatusServiceUnavailable)
			return
		}
		apierr.WriteJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	}
}
