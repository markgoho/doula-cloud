package main

import (
	"encoding/json"
	"log"
	"net/http"

	"doula-cloud/api/internal/offer"
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

// The routes that belong to no Practice: the health probe, the Staff
// session itself, sign-up and invitation acceptance, and #230's
// pre-account Offer read.
//
// What they have in common is that none of them can be scoped to a
// Practice, because at the moment they are called there may not be one
// yet -- or, for the work-state write, because where a person works is a
// fact about her rather than about a Membership (00043).
func registerSessionRoutes(mux *http.ServeMux, g *staffauth.GatedRouter, d Deps) {
	// Under /api like every other route: Firebase Hosting rewrites /api/** to
	// this service with the path unchanged, so a bare /hello would be
	// unreachable from the browser. CI's two smoke tests curl this same path
	// against the container and against the raw Cloud Run URL.
	mux.HandleFunc("GET /api/hello", helloHandler)
	mux.Handle("POST /api/session", session.CreateHandler(d.Verifier, d.DB, d.NudgeEnqueuer))
	mux.Handle("DELETE /api/session", session.EndHandler(d.DB))
	mux.Handle("POST /api/staff/signup", staffauth.SignupHandler(d.Verifier, d.DB))
	mux.Handle("GET /api/staff/session", staffauth.SessionHandler(d.DB))
	// Where she works is a fact about the person, not about a Membership
	// (00043), so its write sits beside the session probe rather than
	// under a Practice -- no {practiceId} in the path, and no staff id
	// either, which is what makes it self-edit-only by shape (#437).
	mux.Handle("PUT /api/staff/work-state", staffauth.UpdateWorkStateHandler(d.DB))
	mux.Handle("POST /api/staff/accept-invite", staffauth.AcceptInviteHandler(d.Verifier, d.DB))
	// #230's pre-account Offer read: no session of either population, so
	// it is mounted on the raw mux and authenticated by the Invitation's
	// token plus the emailed six-digit code. ADR-0008 requires the
	// exemption declared by name in GatedRouter's own registry, in the
	// same change that mounts the route -- g.Exempt is that declaration,
	// and the guardrail test walks it.
	g.Exempt("/api/offers/{offerId}", "pre-account Offer read (ADR-0008, #230): no session exists yet -- authenticated by the Invitation token and the emailed access code")
	mux.Handle("GET /api/offers/{offerId}", offer.ReadHandler(d.DB))
	mux.Handle("POST /api/offers/{offerId}/decline", offer.DeclineByTokenHandler(d.DB))
}
