package main

import (
	"net/http"

	"doula-cloud/api/internal/clientauth"
	"doula-cloud/api/internal/contracts"
	"doula-cloud/api/internal/message"
	"doula-cloud/api/internal/plans"
	"doula-cloud/api/internal/portal"
	"doula-cloud/api/internal/portalinvite"
	"doula-cloud/api/internal/pushsub"
)

// The Client's own surface, behind clientauth rather than staffauth.
//
// A different population with a different session, which is why none of
// these go through GatedRouter: ADR-0008's role table describes Staff at
// a Practice, and a Client holds no Membership to check against. Their
// GETs are declared in exemptGETRoutes in the guardrail test instead.
func registerPortalRoutes(mux *http.ServeMux, d Deps) {
	mux.Handle("POST /api/portal/accept-invite", portalinvite.AcceptInviteHandler(d.Verifier, d.DB))
	mux.Handle("GET /api/portal/session", clientauth.SessionHandler(d.DB))
	mux.Handle("GET /api/portal/engagements/{engagementId}",
		clientauth.Middleware(d.DB)(portal.DetailHandler()))
	mux.Handle("GET /api/portal/engagements/{engagementId}/birth-plan",
		clientauth.Middleware(d.DB)(plans.ClientGetBirthPlanHandler()))
	mux.Handle("GET /api/portal/engagements/{engagementId}/contract",
		clientauth.Middleware(d.DB)(contracts.ClientGetContractHandler()))
	mux.Handle("POST /api/portal/engagements/{engagementId}/contract/sign",
		clientauth.Middleware(d.DB)(contracts.ClientPostSignContractHandler(d.Store)))
	mux.Handle("GET /api/portal/engagements/{engagementId}/contract/pdf",
		clientauth.Middleware(d.DB)(contracts.ClientGetSignedContractPDFHandler(d.Store)))
	mux.Handle("GET /api/portal/engagements/{engagementId}/messages",
		clientauth.Middleware(d.DB)(message.ClientListHandler()))
	mux.Handle("POST /api/portal/engagements/{engagementId}/messages",
		clientauth.Middleware(d.DB)(message.ClientCreateHandler(d.Store, d.Pusher)))
	mux.Handle("GET /api/portal/engagements/{engagementId}/messages/{messageId}/attachment",
		clientauth.Middleware(d.DB)(message.ClientAttachmentHandler(d.Store)))
	mux.Handle("POST /api/portal/engagements/{engagementId}/push-subscriptions",
		clientauth.Middleware(d.DB)(pushsub.ClientRegisterHandler()))
	mux.Handle("DELETE /api/portal/engagements/{engagementId}/push-subscriptions",
		clientauth.Middleware(d.DB)(pushsub.ClientUnregisterHandler()))
}
