package main

import (
	"context"
	"database/sql"
	"net/http"

	"doula-cloud/api/internal/apierr"
	"doula-cloud/api/internal/idempotency"
	"doula-cloud/api/internal/mailsuppress"
	"doula-cloud/api/internal/session"
	"doula-cloud/api/internal/staffauth"
)

type helloResponse struct {
	Message string `json:"message"`
}

func helloHandler(w http.ResponseWriter, _ *http.Request) {
	apierr.WriteJSON(w, http.StatusOK, helloResponse{Message: "hello world"})
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
	staffauth.Mount(g, ir, d.DB, d.Verifier, d.AccountManager, d.NudgeEnqueuer,
		func(ctx context.Context, tx *sql.Tx, address string) (bool, error) {
			return mailsuppress.Active(ctx, tx, address)
		})
}
