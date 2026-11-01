package main

import (
	"database/sql"
	"net/http"

	"doula-cloud/api/internal/apierr"
	"doula-cloud/api/internal/authn"
	"doula-cloud/api/internal/billing"
	"doula-cloud/api/internal/clock"
	"doula-cloud/api/internal/csrf"
	"doula-cloud/api/internal/legal"
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
	// Stripe moves a refund's money back (#2481).
	Stripe billing.StripeClient
	// Accounts is Identity Platform admin on the Staff pool, which a
	// second-factor reset clears (#2483).
	Accounts authn.AccountManager
	// FeedbackRepo is GITHUB_FEEDBACK_REPO, owner/name, which a Feedback
	// issue's link is built from. Empty leaves the link off.
	FeedbackRepo string
	// Legal is the version history legal publish reads. Nil is the
	// shipped history; the end-to-end stack appends a version
	// (stackswitch.Legal).
	Legal []legal.Document
}

// legalHistory is d.Legal, or the shipped history when unset.
func (d Deps) legalHistory() []legal.Document {
	if d.Legal != nil {
		return d.Legal
	}
	return shippedLegal
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
	r.handle(http.MethodPost, "/api/session/step-up", requireSession(d.DB, stepUp(d)))
	r.handle(http.MethodGet, "/api/practices", requireSession(d.DB, practiceList(d.DB, false)))
	r.handle(http.MethodGet, "/api/practices/deleted", requireSession(d.DB, practiceList(d.DB, true)))
	r.handle(http.MethodGet, "/api/practices/dormant", requireSession(d.DB, dormantList(d.DB)))
	r.handle(http.MethodGet, "/api/credits/monthly", requireSession(d.DB, monthlyCredits(d.DB)))
	r.handle(http.MethodGet, "/api/operator-log", requireSession(d.DB, operatorLog(d.DB)))
	r.handle(http.MethodGet, "/api/feedback", requireSession(d.DB, feedbackList(d.DB, d.FeedbackRepo)))
	r.handle(http.MethodGet, "/api/feedback/{feedbackId}", requireSession(d.DB, feedbackRead(d.DB, d.FeedbackRepo)))
	r.handle(http.MethodGet, "/api/practices/{practiceId}/refund", requireSession(d.DB, refundCheckRead(d.DB)))
	r.handle(http.MethodPost, "/api/practices/{practiceId}/refunds", requireStepUp(d.DB, actRefund, refundAct(d)))
	r.handle(http.MethodGet, "/api/practices/{practiceId}/founding-grant", requireSession(d.DB, foundingGrantCheckRead(d.DB)))
	r.handle(http.MethodPost, "/api/practices/{practiceId}/founding-grants", requireSession(d.DB, foundingGrantAct(d.DB)))
	r.handle(http.MethodPost, "/api/pilot-allowlist", requireSession(d.DB, pilotAddressAct(d.DB)))
	r.handle(http.MethodGet, "/api/pilot-gate", requireSession(d.DB, pilotGateRead(d.DB)))
	r.handle(http.MethodPost, "/api/pilot-gate/openings", requireStepUp(d.DB, actPilotGateOpened, pilotGateOpenAct(d.DB)))
	r.handle(http.MethodGet, "/api/legal-versions", requireSession(d.DB, legalVersions(d.DB, d.legalHistory())))
	r.handle(http.MethodGet, "/api/legal-versions/{document}/{version}", requireSession(d.DB, legalVersionCheckRead(d.DB, d.legalHistory())))
	r.handle(http.MethodPost, "/api/legal-versions/{document}/{version}/publications", requireStepUp(d.DB, actLegalVersionPublished, legalPublishAct(d)))
	r.handle(http.MethodGet, "/api/staff", requireSession(d.DB, staffLookup(d.DB)))
	r.handle(http.MethodGet, "/api/staff/{staffId}/second-factor", requireSession(d.DB, secondFactorCheckRead(d.DB)))
	r.handle(http.MethodPost, "/api/staff/{staffId}/second-factor-clears", requireStepUp(d.DB, actSecondFactorCleared, secondFactorClearAct(d)))
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
