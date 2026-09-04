package main

import (
	"doula-cloud/api/internal/outbox"
	"doula-cloud/api/internal/tasknudge"
)

// The three paths named more than once in this package: #613's two
// un-nudged Staff auth mail outboxes and #443's doorless site rebuild.
// Constants only because each is a fact two tests assert about, not
// because a path is more special than the other eleven -- those stay
// written out at their registration, where they read as the address they
// are.
const (
	staffTokenMailOutboxPath   = "/api/internal/notifications/process-staff-token-mail-outbox"
	staffEmailChangeOutboxPath = "/api/internal/notifications/process-staff-email-change-outbox"
	siteBuildOutboxPath        = "/api/internal/site/process-build-outbox"
)

// outboxRegistrations is every outbox the BFF serves (ADR-0010), in one
// list.
//
// This is the whole of what used to be spread across a pass-through
// handler in each worker's own package, a field per worker on Deps, a
// mux.Handle call in registerInternalRoutes, and a second copy of every
// path in tasknudge. A new outbox is now one entry here plus its Worker;
// nothing else in the route table has to learn its name.
//
// Order is not significant -- Register sorts the paths it returns -- but
// the list is kept in the order the outboxes were built, so a reader can
// follow it against the tickets.
func outboxRegistrations(d Deps) []outbox.Registration {
	return []outbox.Registration{
		{
			Path:   "/api/internal/notifications/process-outbox",
			Door:   outbox.NotificationDoor,
			Nudge:  tasknudge.PortalInvite,
			Worker: d.PortalInviteWorker,
		},
		{
			// #342. Its own endpoint rather than a shared one because it
			// processes an unrelated outbox table, which is ADR-0010's
			// accepted per-Notification-type cost.
			Path:   "/api/internal/notifications/process-low-credit-outbox",
			Door:   outbox.NotificationDoor,
			Nudge:  tasknudge.LowCredit,
			Worker: d.LowCreditWorker,
		},
		{
			// #343's payout-incomplete outbox.
			Path:   "/api/internal/notifications/process-payout-outbox",
			Door:   outbox.NotificationDoor,
			Nudge:  tasknudge.Payout,
			Worker: d.PayoutWorker,
		},
		{
			// #344's payment-received outbox.
			Path:   "/api/internal/notifications/process-payment-outbox",
			Door:   outbox.NotificationDoor,
			Nudge:  tasknudge.PaymentReceived,
			Worker: d.PaymentReceivedWorker,
		},
		{
			// #345's session-notice outbox (new sign-in, session revoked).
			Path:   "/api/internal/notifications/process-session-notice-outbox",
			Door:   outbox.NotificationDoor,
			Nudge:  tasknudge.SessionNotice,
			Worker: d.SessionNoticeWorker,
		},
		{
			// #339's Staff invitation outbox (RA-G1), whose write site is
			// staffauth.InviteHandler (#316).
			Path:   "/api/internal/notifications/process-staff-invite-outbox",
			Door:   outbox.NotificationDoor,
			Nudge:  tasknudge.StaffInvite,
			Worker: d.StaffInviteWorker,
		},
		{
			// #317's Offer outbox, whose write site is offer.CreateHandler's
			// email-target path.
			Path:   "/api/internal/notifications/process-offer-outbox",
			Door:   outbox.NotificationDoor,
			Nudge:  tasknudge.EngagementOffer,
			Worker: d.OfferWorker,
		},
		{
			// #398's Engagement Request outbox, whose write site is
			// engagementrequest.RequestHandler.
			Path:   "/api/internal/notifications/process-engagement-request-outbox",
			Door:   outbox.NotificationDoor,
			Nudge:  tasknudge.EngagementRequest,
			Worker: d.EngagementRequestWorker,
		},
		{
			// #613's Staff auth mail outbox (verification and reset links),
			// whose write sites are staffauth's signup/verify/reset
			// handlers. No Nudge on purpose: that ticket accepted ADR-0010's
			// plain delay, so Cloud Scheduler's cadence alone carries it.
			Path:   staffTokenMailOutboxPath,
			Door:   outbox.NotificationDoor,
			Worker: d.StaffTokenMailWorker,
		},
		{
			// #613's second outbox, the email-change notice. Not nudged, for
			// the same reason as the one above.
			Path:   staffEmailChangeOutboxPath,
			Door:   outbox.NotificationDoor,
			Worker: d.StaffEmailChangeWorker,
		},
		{
			// #615's Owner-vouched recovery code outbox. Nudged, unlike
			// #613's two, because a person is waiting on the phone for it.
			Path:   "/api/internal/notifications/process-mfa-recovery-outbox",
			Door:   outbox.NotificationDoor,
			Nudge:  tasknudge.MFARecoveryCode,
			Worker: d.MFARecoveryMailWorker,
		},
		{
			// #394's Client-erasure outbox, under /api/internal/clients
			// rather than /notifications because it notifies nobody -- it
			// deletes a Stripe Customer, runs its Redaction Job once
			// Stripe's 90-day floor has passed, and deletes an Identity
			// Platform account (ADR-0027). Nudged so the two immediate acts
			// happen while the Owner is still on the screen.
			Path:   "/api/internal/clients/process-erasure-outbox",
			Door:   outbox.NotificationDoor,
			Nudge:  tasknudge.ClientErasure,
			Worker: d.ClientErasureWorker,
		},
		{
			// #443's site rebuild, under /api/internal/site for the same
			// reason: it notifies nobody, it turns queued rebuilds into one
			// repository_dispatch.
			//
			// The one outbox with no Door. Its table is not under RLS at
			// all, so the notification door would license nothing -- and a
			// worker should not be handed a door it has no use for. This is
			// the second door shape, and the reason Door is a field rather
			// than a constant inside ProcessHandler.
			Path:   siteBuildOutboxPath,
			Nudge:  tasknudge.SiteBuild,
			Worker: d.SiteBuildWorker,
		},
	}
}
