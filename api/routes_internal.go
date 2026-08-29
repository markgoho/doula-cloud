package main

import (
	"net/http"

	"doula-cloud/api/internal/billing"
	"doula-cloud/api/internal/engagementrequest"
	"doula-cloud/api/internal/offer"
	"doula-cloud/api/internal/payments"
	"doula-cloud/api/internal/portalinvite"
	"doula-cloud/api/internal/sessionnotice"
	"doula-cloud/api/internal/sitebuild"
	"doula-cloud/api/internal/staffinvite"
)

// The endpoints nobody signs in to reach: ADR-0010's outbox workers and
// #443's two site endpoints.
//
// Cloud Scheduler and Cloud Tasks call these on a cadence, authenticated
// by X-Internal-Secret rather than by a session, so they sit outside
// staffauth.Middleware and GatedRouter entirely -- the same position the
// webhooks hold, and for the same reason.
func registerInternalRoutes(mux *http.ServeMux, d Deps) {
	// Cloud-Scheduler-triggered, not Staff/Client facing (ADR-0010):
	// authenticated by d.WorkerSecret, not a session, so it sits
	// outside staffauth.Middleware/GatedRouter like the Stripe webhooks
	// above.
	mux.Handle("POST /api/internal/notifications/process-outbox", portalinvite.ProcessOutboxHandler(d.DB, d.PortalInviteWorker, d.WorkerSecret))
	// Same X-Internal-Secret guard, same Cloud Scheduler cadence, a
	// separate endpoint because the two workers process unrelated
	// outbox tables (ADR-0010, #342).
	mux.Handle("POST /api/internal/notifications/process-low-credit-outbox", billing.ProcessOutboxHandler(d.DB, d.LowCreditWorker, d.WorkerSecret))
	// Same shape again for #343's payout-incomplete outbox.
	mux.Handle("POST /api/internal/notifications/process-payout-outbox", payments.ProcessOutboxHandler(d.DB, d.PayoutWorker, d.WorkerSecret))
	// Same shape again for #344's payment-received outbox.
	mux.Handle("POST /api/internal/notifications/process-payment-outbox", payments.ProcessPaymentOutboxHandler(d.DB, d.PaymentReceivedWorker, d.WorkerSecret))
	// Same shape again for #345's session-notice outbox (new sign-in,
	// session revoked).
	mux.Handle("POST /api/internal/notifications/process-session-notice-outbox", sessionnotice.ProcessOutboxHandler(d.DB, d.SessionNoticeWorker, d.WorkerSecret))
	// Same shape again for #339's Staff invitation outbox (RA-G1), whose
	// write site is staffauth.InviteHandler above (#316).
	mux.Handle("POST /api/internal/notifications/process-staff-invite-outbox", staffinvite.ProcessOutboxHandler(d.DB, d.StaffInviteWorker, d.WorkerSecret))
	// Same shape again for #317's Offer outbox, whose write site is
	// offer.CreateHandler's email-target path above.
	mux.Handle("POST /api/internal/notifications/process-offer-outbox", offer.ProcessOutboxHandler(d.DB, d.OfferWorker, d.WorkerSecret))
	// Same shape again for #398's Engagement Request outbox, whose write
	// site is engagementrequest.RequestHandler above.
	mux.Handle("POST /api/internal/notifications/process-engagement-request-outbox", engagementrequest.ProcessOutboxHandler(d.DB, d.EngagementRequestWorker, d.WorkerSecret))
	// #443's two site endpoints, on the same X-Internal-Secret shape and
	// under /api/internal/site rather than /notifications, because
	// neither of them notifies anybody. process-build-outbox turns
	// queued rebuilds into one repository_dispatch; verify-pages probes
	// every published page and records whether it resolved. Both are
	// called by Cloud Scheduler on a cadence, and verify-pages is also
	// the last step of the deploy workflow itself.
	mux.Handle("POST /api/internal/site/process-build-outbox", sitebuild.ProcessOutboxHandler(d.DB, d.SiteBuildWorker, d.WorkerSecret))
	mux.Handle("POST /api/internal/site/verify-pages", sitebuild.VerifyHandler(d.DB, d.PageVerifier, d.WorkerSecret))
}
