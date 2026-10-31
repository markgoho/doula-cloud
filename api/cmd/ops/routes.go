package main

import (
	"database/sql"
	"net/http"

	"doula-cloud/api/internal/apierr"
	"doula-cloud/api/internal/clock"
)

// Deps is everything doula-ops's route table needs.
type Deps struct {
	DB  *sql.DB
	Now clock.Clock
}

// router mounts a route and records it, so routes() can hand back the
// list a guardrail test reads.
type router struct {
	mux    *http.ServeMux
	routes []string
}

func (r *router) handle(method, path string, h http.HandlerFunc) {
	pattern := method + " " + path
	r.mux.Handle(pattern, h)
	r.routes = append(r.routes, pattern)
}

// routes builds doula-ops's route table and returns every route it
// mounted as "METHOD /path".
func routes(d Deps) (http.Handler, []string) {
	r := &router{mux: http.NewServeMux()}
	r.handle(http.MethodGet, "/api/health", health(d.DB))
	return clock.Middleware(d.Now)(r.mux), r.routes
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
