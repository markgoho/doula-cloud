package main

import (
	"encoding/json"
	"log"
	"net/http"

	"doula-cloud/api/internal/idempotency"
	"doula-cloud/api/internal/session"
	"doula-cloud/api/internal/staffauth"
)

type helloResponse struct {
	Message string `json:"message"`
}

func helloHandler(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	// coverage:ignore reason: response encoding failure, not exercised by unit tests
	if err := json.NewEncoder(w).Encode(helloResponse{Message: "hello world"}); err != nil {
		log.Printf("helloHandler: encode response: %v", err)
	}
}

// The routes that belong to no Practice: the health probe and the Staff
// session itself. staffauth.Mount registers everything else that
// predates a Practice session -- sign-up, invitation acceptance, and the
// person-level facts (work state, email, MFA) that #437 and #613 keep
// off any one Membership -- alongside its own Practice-scoped routes, so
// it is called once, here, rather than split across two files the way
// its registrations used to be.
func registerSessionRoutes(g *staffauth.GatedRouter, ir *idempotency.Router, d Deps) {
	// Under /api like every other route: Firebase Hosting rewrites /api/** to
	// this service with the path unchanged, so a bare /hello would be
	// unreachable from the browser. CI's two smoke tests curl this same path
	// against the container and against the raw Cloud Run URL.
	g.OpenGet("/api/hello", "no auth at all -- a health probe", http.HandlerFunc(helloHandler))
	session.Mount(g, d.DB, d.Verifier, d.NudgeEnqueuer)
	staffauth.Mount(g, ir, d.DB, d.Verifier, d.AccountManager, d.NudgeEnqueuer)
}
