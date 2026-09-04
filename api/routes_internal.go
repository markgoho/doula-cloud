package main

import (
	"net/http"

	"doula-cloud/api/internal/authmail"
	"doula-cloud/api/internal/billing"
	"doula-cloud/api/internal/client"
	"doula-cloud/api/internal/engagementrequest"
	"doula-cloud/api/internal/mfarecoverymail"
	"doula-cloud/api/internal/offer"
	"doula-cloud/api/internal/payments"
	"doula-cloud/api/internal/portalinvite"
	"doula-cloud/api/internal/sessionnotice"
	"doula-cloud/api/internal/sitebuild"
	"doula-cloud/api/internal/staffauth"
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
	// Same shape again for #613's two Staff auth mail outboxes
	// (verification/reset, and the email-change notice), whose write
	// sites are staffauth's signup/verify/reset/emailchange handlers
	// above.
	mux.Handle("POST /api/internal/notifications/process-staff-token-mail-outbox", authmail.ProcessTokenMailOutboxHandler(d.DB, d.StaffTokenMailWorker, d.WorkerSecret))
	mux.Handle("POST /api/internal/notifications/process-staff-email-change-outbox", authmail.ProcessEmailChangeOutboxHandler(d.DB, d.StaffEmailChangeWorker, d.WorkerSecret))
	// #615's Owner-vouched recovery code outbox, on the same X-Internal-
	// Secret guard and Cloud Scheduler cadence, also nudged by Cloud
	// Tasks (tasknudge.MFARecoveryCode) since a person is waiting on the
	// phone for this one, unlike #613's two token mails.
	mux.Handle("POST /api/internal/notifications/process-mfa-recovery-outbox", mfarecoverymail.ProcessOutboxHandler(d.DB, d.MFARecoveryMailWorker, d.WorkerSecret))
	// #394's Client-erasure outbox. Same X-Internal-Secret guard and
	// Cloud Scheduler cadence, under /api/internal/clients rather than
	// /notifications because it notifies nobody -- it deletes a Stripe
	// Customer, runs its Redaction Job once Stripe's 90-day floor has
	// passed, and deletes an Identity Platform account (ADR-0027). Also
	// nudged by Cloud Tasks (tasknudge.ClientErasure), so the two
	// immediate acts happen while the Owner is still on the screen.
	mux.Handle("POST /api/internal/clients/process-erasure-outbox", client.ProcessErasureOutboxHandler(d.DB, d.ClientErasureWorker, d.WorkerSecret))
	// #443's two site endpoints, on the same X-Internal-Secret shape and
	// under /api/internal/site rather than /notifications, because
	// neither of them notifies anybody. process-build-outbox turns
	// queued rebuilds into one repository_dispatch; verify-pages probes
	// every published page and records whether it resolved. Both are
	// called by Cloud Scheduler on a cadence, and verify-pages is also
	// the last step of the deploy workflow itself.
	mux.Handle("POST /api/internal/site/process-build-outbox", sitebuild.ProcessOutboxHandler(d.DB, d.SiteBuildWorker, d.WorkerSecret))
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
