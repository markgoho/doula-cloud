package main

import (
	"net/http"

	"doula-cloud/api/internal/billing"
	"doula-cloud/api/internal/outbox"
	"doula-cloud/api/internal/sitebuild"
	"doula-cloud/api/internal/staffauth"
)

// The endpoints nobody signs in to reach: ADR-0010's outboxes, #443's
// page verifier, and the operator endpoints.
//
// Cloud Scheduler and Cloud Tasks call these on a cadence, authenticated
// by X-Internal-Secret rather than by a session, so they sit outside
// staffauth.Middleware and GatedRouter entirely -- the same position the
// webhooks hold, and for the same reason.
func registerInternalRoutes(mux *http.ServeMux, d Deps) {
	// Every outbox, from the one list in outboxes.go (ADR-0010), each
	// authenticated by d.WorkerSecret rather than a session so it sits
	// outside staffauth.Middleware/GatedRouter like the Stripe webhooks.
	//
	// This used to be thirteen mux.Handle calls whose only differences
	// were the path and the worker, every one of them routed through a
	// pass-through ProcessOutboxHandler in the worker's own package.
	outbox.Register(mux, d.DB, d.WorkerSecret, outboxRegistrations(d))
	// #443's page verifier, on the same X-Internal-Secret shape and under
	// /api/internal/site alongside that ticket's rebuild outbox above.
	// Deliberately not an outbox itself and so not in the list: it
	// processes no queue, it probes every published page and records
	// whether it resolved. Two callers, and the same behavior for both --
	// Cloud Scheduler on a cadence, and the last step of the deploy
	// workflow. The cadence is what covers a build that fails and
	// produces no deploy and no callback at all.
	mux.Handle("POST /api/internal/site/verify-pages", sitebuild.VerifyHandler(d.DB, d.PageVerifier, d.WorkerSecret))
	// #420's two billing endpoints, on the same X-Internal-Secret guard.
	// Neither is a screen: /support tells a Practice to email us for a
	// refund, because a refund issued on her recorded request restarts
	// the dormancy clock that one we initiated would not (APL 1315), and
	// the dormancy list is an operator's yearly mailing, not a tenant's
	// view of anything.
	mux.Handle("POST /api/internal/billing/refunds", billing.RefundHandler(d.DB, d.StripeClient, d.WorkerSecret))
	mux.Handle("GET /api/internal/billing/dormant-practices", billing.DormantPracticesHandler(d.DB, d.WorkerSecret))
	// #449's founding grant, on the same guard again. A pilot Practice's
	// Credits are issued by a person, by hand, roughly a dozen times in
	// total -- so this is an operator endpoint that records who issued
	// them, not a screen and not an ad-hoc INSERT.
	mux.Handle("POST /api/internal/billing/founding-grants", billing.FoundingGrantHandler(d.DB, d.WorkerSecret))
	// #605's support path: an operator clears a sole Owner's enrolment
	// after a live video call and government-ID match against her
	// Practice's Stripe Connect identity (ADR-0007), per
	// docs/runbooks/mfa-recovery-support.md. Same guard, same shape as
	// founding-grants above -- an operator endpoint, deliberately not a
	// screen (#615's AC: "no product surface").
	mux.Handle("POST /api/internal/staffauth/mfa-recovery/support-clear", staffauth.SupportClearHandler(d.AccountManager, d.DB, d.WorkerSecret))
}
