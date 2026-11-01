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

// Deps is what doula-ops's route table needs.
type Deps struct {
	DB       *sql.DB
	Now      clock.Clock
	Verifier Verifier
	// Set only while the emulator is the identity provider.
	AcceptEmulatorFactor bool
	Origins              []string
	Stripe               billing.StripeClient
	Accounts             authn.AccountManager
	FeedbackRepo         string
	Legal                []legal.Document
}

func (d Deps) legalHistory() []legal.Document {
	if d.Legal != nil {
		return d.Legal
	}
	return shippedLegal
}

type router struct {
	mux    *http.ServeMux
	routes []string
}

func (r *router) handle(method, path string, h http.Handler) {
	pattern := method + " " + path
	r.mux.Handle(pattern, h)
	r.routes = append(r.routes, pattern)
}

// There is no signup route: an operator login is made by hand in the tenant.
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

func health(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if err := db.PingContext(r.Context()); err != nil {
			apierr.WriteError(w, "The database cannot be reached.", http.StatusServiceUnavailable)
			return
		}
		apierr.WriteJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	}
}
