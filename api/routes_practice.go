package main

import (
	"encoding/json"
	"log"
	"net/http"

	"doula-cloud/api/internal/billing"
	"doula-cloud/api/internal/client"
	"doula-cloud/api/internal/clientfieldtemplate"
	"doula-cloud/api/internal/contracts"
	"doula-cloud/api/internal/engagement"
	"doula-cloud/api/internal/engagementrequest"
	"doula-cloud/api/internal/idempotency"
	"doula-cloud/api/internal/message"
	"doula-cloud/api/internal/offer"
	"doula-cloud/api/internal/payments"
	"doula-cloud/api/internal/plans"
	"doula-cloud/api/internal/portalinvite"
	"doula-cloud/api/internal/pushsub"
	"doula-cloud/api/internal/staffauth"
	"doula-cloud/api/internal/visit"
	"doula-cloud/api/internal/website"
)

// ownerAndAdmin is the role declaration for every GatedRouter route
// ADR-0008's read table admits to Owner and Admin only (Staff roster,
// Credit balance and ledger, Contract's money-bearing Signed PDF and
// Invoice history) -- named once so golangci-lint's package-wide goconst
// check doesn't see four independent literals to flag.
var ownerAndAdmin = []string{"owner", "admin"}

// practiceSessionResponse confirms to the frontend which Practice the
// caller landed on -- and, as a side effect of running through
// staffauth.Middleware, records it as the Staff member's last-used
// Practice for their next login.
type practiceSessionResponse struct {
	PracticeID   string   `json:"practiceId"`
	PracticeName string   `json:"practiceName"`
	Roles        []string `json:"roles"`
}

func practiceSessionHandler(w http.ResponseWriter, r *http.Request) {
	tx, _ := staffauth.Tx(r.Context())
	staffID, _ := staffauth.StaffID(r.Context())
	practiceID, _ := staffauth.PracticeID(r.Context())

	var name string
	if err := tx.QueryRowContext(r.Context(), `SELECT name FROM practices WHERE id = $1`, practiceID).Scan(&name); err != nil {
		// coverage:ignore reason: DB query failure, not exercised by unit tests
		http.Error(w, staffauth.MsgInternalError, http.StatusInternalServerError)
		return
	}

	roles, err := staffauth.Roles(r.Context(), tx, practiceID, staffID)
	if err != nil {
		// coverage:ignore reason: DB query failure, not exercised by unit tests
		http.Error(w, staffauth.MsgInternalError, http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	// coverage:ignore reason: response encoding failure, not exercised by unit tests
	if err := json.NewEncoder(w).Encode(practiceSessionResponse{PracticeID: practiceID, PracticeName: name, Roles: roles}); err != nil {
		log.Printf("practiceSessionHandler: encode response: %v", err)
	}
}

// Everything under /api/practices: the Staff roster, billing, Stripe
// Connect, the website answer, Clients, Engagements and their Visits,
// Messages, Plans, Contracts, Invoices and Offers.
//
// Two thirds of the BFF's surface, and the file most feature work
// touches. It is one file rather than several because the routes here
// share ADR-0008's role vocabulary -- ownerAndAdmin below, AnyStaff, and
// the attachment narrowing each handler does for itself -- and splitting
// them further would scatter that without separating anything.
func registerPracticeRoutes(g *staffauth.GatedRouter, ir *idempotency.Router, d Deps) {
	g.Get("/api/practices/{practiceId}/session", staffauth.AnyStaff, http.HandlerFunc(practiceSessionHandler))
	// Roles and employment type are edited together on one surface
	// (RA-G2, #261) -- ADR-0008 makes them the two halves of what a
	// person is at a Practice, so there is one endpoint, not two.
	ir.Exempt("PATCH /api/practices/{practiceId}/staff/{staffId}/membership",
		"PATCH replaces roles and employment type wholesale to the caller's given values, and records an audit event only for an axis that actually changed -- a repeated call with the same body is a no-op on both",
		staffauth.Middleware(d.DB)(staffauth.UpdateMembershipHandler()))
	// The route #291 found missing: without it a roster row nobody wants
	// can never be taken off.
	ir.Exempt("DELETE /api/practices/{practiceId}/staff/{staffId}/membership",
		"delete; a retry after the first succeeds finds no membership row left and 404s instead of removing or recording removal twice",
		staffauth.Middleware(d.DB)(staffauth.RemoveMembershipHandler()))
	ir.Replayable("POST /api/practices/{practiceId}/staff/invitations",
		staffauth.Middleware(d.DB)(idempotency.Wrap(staffauth.InviteHandler(d.NudgeEnqueuer))))
	ir.Exempt("POST /api/practices/{practiceId}/staff/invitations/{invitationId}/revoke",
		"state-guarded UPDATE ... WHERE status = 'pending'; a retry after the first commit affects zero rows and 404s instead of revoking twice",
		staffauth.Middleware(d.DB)(staffauth.RevokeInvitationHandler()))
	// Staff roster -- members and pending invitations both: Owner and
	// Admin only (ADR-0008's read table) -- a Doula has no reason to see
	// the full roster.
	g.Get("/api/practices/{practiceId}/staff", ownerAndAdmin, staffauth.ListStaffHandler())
	ir.Exempt("DELETE /api/practices/{practiceId}/staff/{staffId}/sessions",
		"EndAllSessions ends whatever remains and no-ops once already ended, and QueueSessionRevoked's own ON CONFLICT ... WHERE status = 'pending' DO NOTHING dedupes the notification; a retry can't double-notify",
		staffauth.Middleware(d.DB)(staffauth.EndSessionsHandler(d.NudgeEnqueuer)))
	// Credit balance and ledger: Owner and Admin only (ADR-0008).
	g.Get("/api/practices/{practiceId}/billing", ownerAndAdmin, billing.GetBalanceHandler())
	ir.Exempt("POST /api/practices/{practiceId}/billing/purchases",
		"creates a Stripe Checkout Session URL only; the ledger is credited by the purchase webhook against the actual completed payment, so a duplicate call yields an extra unused Checkout Session, never a double charge or double credit",
		staffauth.Middleware(d.DB)(billing.PostPurchaseHandler(d.StripeClient)))
	ir.Exempt("POST /api/practices/{practiceId}/payments/connect",
		"lazily creates the Stripe Connect account and reuses the stored account id on any retry, row-locked against a concurrent create; a duplicate call resumes the same account, not a second one",
		staffauth.Middleware(d.DB)(payments.PostConnectHandler(d.PaymentsClient)))
	// ADR-0008's read table has no row for Stripe Connect state; mirroring
	// the write side's Owner-only gate (PostConnectHandler,
	// staffauth.RequireOwner) is the narrowest defensible default until a
	// real rule lands (#267 stays open for that rule).
	g.Get("/api/practices/{practiceId}/payments/connect", []string{"owner"}, payments.GetConnectStatusHandler(d.PaymentsClient))
	// The website a Practice declares to Stripe (#440). Read by every
	// Staff member, because the payments screen has to tell a Doula who
	// opens it what is outstanding rather than show her an empty panel,
	// and nothing here is secret -- the whole point of the answer is that
	// it is published. Written by an Owner alone (website.PutHandler).
	g.Get("/api/practices/{practiceId}/website", staffauth.AnyStaff, website.GetHandler())
	ir.Exempt("PUT /api/practices/{practiceId}/website",
		"one declaration per Practice, replaced whole (PUT semantics) -- the handler's own doc comment already says re-sending the same body is safe -- and the rebuild nudge only fires when the page becomes newly stale",
		staffauth.Middleware(d.DB)(website.PutHandler(d.NudgeEnqueuer)))
	// Engagements, Visits, Messages, Plan Instances, and Contract scope
	// are open to every Staff role at the mount; the employee/contractor
	// split ADR-0008's read table draws inside that column is
	// attachment-narrowing the handler itself enforces via
	// staffauth.Reader.CanAccessEngagement, not a role declaration.
	// The Client write surface (#397): search, lookup-before-insert
	// create, the detail read, and edit. Saving or editing a Client is
	// free and creates no Engagement -- that split off into a separate
	// Engagement Request, built elsewhere. Role gating beyond "any Staff
	// member" (the contractor create/search refusal, the attached-Clients
	// narrowing on edit/detail) is enforced inside each handler via
	// staffauth.Reader, the same pattern engagement.DetailHandler already
	// uses for CanAccessEngagement.
	g.Get("/api/practices/{practiceId}/clients", staffauth.AnyStaff, client.ListHandler())
	g.Get("/api/practices/{practiceId}/clients/search", staffauth.AnyStaff, client.SearchHandler())
	ir.Replayable("POST /api/practices/{practiceId}/clients",
		staffauth.Middleware(d.DB)(idempotency.Wrap(client.CreateHandler())))
	g.Get("/api/practices/{practiceId}/clients/{clientId}", staffauth.AnyStaff, client.DetailHandler())
	ir.Exempt("PUT /api/practices/{practiceId}/clients/{clientId}",
		"PUT replaces the Client record wholesale; re-sending the same body is a no-op",
		staffauth.Middleware(d.DB)(client.EditHandler()))
	// ADR-0017's Engagement Request (#398): the ask for paid work with a
	// Client, and the act that creates an Engagement. Request is any
	// Staff member but a contractor Doula (enforced here and,
	// independently, by engagement_requests_insert's RLS policy);
	// approve/refuse are Owner/Admin; withdraw is the requester alone,
	// so it carries no role declaration.
	ir.Replayable("POST /api/practices/{practiceId}/clients/{clientId}/engagement-requests",
		staffauth.Middleware(d.DB)(idempotency.Wrap(engagementrequest.RequestHandler(d.DB, d.NudgeEnqueuer))))
	ir.Exempt("POST /api/practices/{practiceId}/engagement-requests/{requestId}/approve",
		"approve() locks the Request FOR UPDATE and checks state = pending inside the same transaction; a retry after the first commit finds it already decided and 409s instead of creating a second Engagement or spending a second Credit",
		staffauth.Middleware(d.DB)(engagementrequest.ApproveHandler(d.DB, d.NudgeEnqueuer)))
	ir.Exempt("POST /api/practices/{practiceId}/engagement-requests/{requestId}/refuse",
		"state-guarded UPDATE ... WHERE state = 'pending'; a retry after the first commit affects zero rows and 409s instead of refusing twice",
		staffauth.Middleware(d.DB)(engagementrequest.RefuseHandler()))
	ir.Exempt("POST /api/practices/{practiceId}/engagement-requests/{requestId}/withdraw",
		"state-guarded UPDATE ... WHERE requested_by = $1 AND state = 'pending'; a retry after the first commit affects zero rows and 409s instead of withdrawing twice",
		staffauth.Middleware(d.DB)(engagementrequest.WithdrawHandler()))
	g.Get("/api/practices/{practiceId}/engagements/{engagementId}", staffauth.AnyStaff, engagement.DetailHandler())
	// Completing an Engagement runs ADR-0008's cascade -- open Offers
	// withdrawn, open attachments ended -- so it is one endpoint, not a
	// generic status PATCH a caller could half-apply.
	ir.Exempt("POST /api/practices/{practiceId}/engagements/{engagementId}/complete",
		"documented idempotent by construction: re-running the completion cascade on an already-completed Engagement is a no-op that only closes anything a partial earlier run left behind",
		staffauth.Middleware(d.DB)(engagement.CompleteHandler()))
	g.Get("/api/practices/{practiceId}/engagements/{engagementId}/visits", staffauth.AnyStaff, visit.ListHandler())
	// staffauth.AttachingWrite is ADR-0008's write-side seam: every
	// Engagement-scoped write below attaches the acting Doula, accrued,
	// once it has succeeded. It is applied here rather than inside each
	// handler so a new Engagement write cannot quietly fall off the list
	// -- #231's whole argument against a second hand-maintained registry.
	// Newly wrapped (2026 idempotency-stance review): CreateHandler is an
	// unconditional INSERT with a fresh id each call and no uniqueness
	// guard -- the same unguarded "create" shape as the six routes
	// already wrapped below, so a double-click logged a Visit twice.
	ir.Replayable("POST /api/practices/{practiceId}/engagements/{engagementId}/visits",
		staffauth.Middleware(d.DB)(staffauth.AttachingWrite(idempotency.Wrap(visit.CreateHandler()))))
	ir.Exempt("PATCH /api/practices/{practiceId}/engagements/{engagementId}/visits/{visitId}",
		"plain UPDATE staff_id = $1 WHERE id = $2; sets the assignment to the given value, so re-sending the same body is a no-op",
		staffauth.Middleware(d.DB)(staffauth.AttachingWrite(visit.ReassignHandler())))
	g.Get("/api/practices/{practiceId}/engagements/{engagementId}/messages", staffauth.AnyStaff, message.ListHandler())
	ir.Replayable("POST /api/practices/{practiceId}/engagements/{engagementId}/messages",
		staffauth.Middleware(d.DB)(staffauth.AttachingWrite(idempotency.Wrap(message.CreateHandler(d.Store, d.Pusher)))))
	g.Get("/api/practices/{practiceId}/engagements/{engagementId}/messages/{messageId}/attachment", staffauth.AnyStaff, message.AttachmentHandler(d.Store))
	// Plan Template and Contract Template: every Staff role (ADR-0008),
	// no attachment narrowing -- a Template isn't Engagement-scoped.
	g.Get("/api/practices/{practiceId}/plan-templates/{planType}", staffauth.AnyStaff, plans.GetTemplateHandler())
	ir.Exempt("PUT /api/practices/{practiceId}/plan-templates/{planType}",
		"upsert (ON CONFLICT ... DO UPDATE); replaces the template wholesale, so re-sending the same body is a no-op",
		staffauth.Middleware(d.DB)(plans.PutTemplateHandler()))
	// ADR-0017's Client Field Template settings screen (#399): the field
	// list an Owner or Admin defines for a Client's Practice-defined
	// layer. Sibling of Plan Templates above -- read by any Staff member
	// (the definitions carry nothing secret), written by an Owner or
	// Admin alone (client_field_templates_insert/_update, 00050, enforce
	// the same rule in RLS).
	g.Get("/api/practices/{practiceId}/client-field-template", staffauth.AnyStaff, clientfieldtemplate.GetHandler())
	ir.Exempt("PUT /api/practices/{practiceId}/client-field-template",
		"upsert (ON CONFLICT ... DO UPDATE); replaces the template wholesale, so re-sending the same body is a no-op",
		staffauth.Middleware(d.DB)(clientfieldtemplate.PutHandler()))
	ir.Exempt("POST /api/practices/{practiceId}/engagements/{engagementId}/plans/{planType}",
		"guarded by plan_instances' unique constraint on (engagement_id, plan_type); a retry after the first succeeds hits the constraint and 409s rather than creating a duplicate Plan Instance",
		staffauth.Middleware(d.DB)(staffauth.AttachingWrite(plans.PostInstanceHandler())))
	g.Get("/api/practices/{practiceId}/engagements/{engagementId}/plans/{planType}", staffauth.AnyStaff, plans.GetInstanceHandler())
	ir.Exempt("PUT /api/practices/{practiceId}/engagements/{engagementId}/plans/{planType}",
		"full-replace UPDATE of the Plan Instance's answers; re-sending the same body is a no-op",
		staffauth.Middleware(d.DB)(staffauth.AttachingWrite(plans.PutInstanceHandler())))
	g.Get("/api/practices/{practiceId}/contract-template", staffauth.AnyStaff, contracts.GetTemplateHandler())
	ir.Exempt("PUT /api/practices/{practiceId}/contract-template",
		"upsert (ON CONFLICT ... DO UPDATE); replaces the template wholesale, so re-sending the same body is a no-op",
		staffauth.Middleware(d.DB)(contracts.PutTemplateHandler()))
	ir.Exempt("POST /api/practices/{practiceId}/engagements/{engagementId}/contract",
		"guarded by contracts' unique constraint on engagement_id; a retry after the first succeeds hits the constraint and 409s rather than creating a duplicate Contract",
		staffauth.Middleware(d.DB)(staffauth.AttachingWrite(contracts.PostContractHandler())))
	// Contract read is the sharpest #231 case: scope reaches every role
	// (narrowed by attachment for a contractor, same as above), but money
	// -- and Invoice history -- is Owner/Admin only, never a Doula's,
	// employee or contractor (ADR-0008: "her own agreed fee only ...
	// never the Practice's price"). GetContractHandler does the
	// scope-vs-money split itself via staffauth.Reader +
	// contracts.ContractScope/ContractFull; the mount stays AnyStaff so
	// scope-only Doulas still reach it.
	g.Get("/api/practices/{practiceId}/engagements/{engagementId}/contract", staffauth.AnyStaff, contracts.GetContractHandler())
	ir.Exempt("PUT /api/practices/{practiceId}/engagements/{engagementId}/contract",
		"full-replace UPDATE of the Contract's merge field values; re-sending the same body is a no-op",
		staffauth.Middleware(d.DB)(staffauth.AttachingWrite(contracts.PutContractHandler())))
	ir.Exempt("POST /api/practices/{practiceId}/engagements/{engagementId}/contract/send",
		"state-guarded (status != 'draft' -> 409); a retry after the first commit finds the Contract already sent and 409s instead of pushing the Client notification twice",
		staffauth.Middleware(d.DB)(staffauth.AttachingWrite(contracts.PostSendContractHandler(d.Pusher))))
	ir.Exempt("POST /api/practices/{practiceId}/engagements/{engagementId}/contract/void",
		"state-guarded (status != 'signed' -> 409); a retry after the first commit 409s instead of voiding twice",
		staffauth.Middleware(d.DB)(staffauth.AttachingWrite(contracts.PostVoidContractHandler())))
	// The Signed PDF is a rendered, unredactable document -- it can't be
	// split into scope/money views the way the JSON Contract read can, so
	// it follows the money row: Owner/Admin only.
	g.Get("/api/practices/{practiceId}/engagements/{engagementId}/contract/pdf", ownerAndAdmin, contracts.GetSignedContractPDFHandler(d.Store))
	// Newly wrapped (2026 idempotency-stance review): every call
	// unconditionally calls Stripe CreateInvoice + FinalizeInvoice and
	// inserts a new invoices row, with no dedup guard -- a double-click
	// billed the Client twice. Money-creating, same as the six routes
	// already wrapped below.
	ir.Replayable("POST /api/practices/{practiceId}/engagements/{engagementId}/contract/invoices",
		staffauth.Middleware(d.DB)(idempotency.Wrap(payments.PostInvoiceHandler(d.PaymentsClient))))
	// Invoice history rides the same money row as Contract money -- see
	// above. A contractor's own-fee narrowing (rather than an outright
	// no) is #317's to build once the Offer/Attachment flow exists.
	g.Get("/api/practices/{practiceId}/engagements/{engagementId}/contract/invoices", ownerAndAdmin, payments.GetInvoicesHandler())
	// ADR-0008's Offer flow (#317). The Practice side is Owner/Admin --
	// making an Offer, taking it back, and reading who has been asked,
	// which names people and so follows the Staff-roster row of the read
	// table. The Doula side is her own inbox and her own decisions, so it
	// is scoped to her staff_id in SQL rather than by a role declaration.
	ir.Replayable("POST /api/practices/{practiceId}/engagements/{engagementId}/offers",
		staffauth.Middleware(d.DB)(idempotency.Wrap(offer.CreateHandler(d.NudgeEnqueuer))))
	g.Get("/api/practices/{practiceId}/engagements/{engagementId}/offers", ownerAndAdmin, offer.EngagementListHandler())
	g.Get("/api/practices/{practiceId}/offers", staffauth.AnyStaff, offer.InboxHandler())
	ir.Exempt("POST /api/practices/{practiceId}/offers/{offerId}/accept",
		"state-guarded UPDATE ... WHERE state = 'offered'; a retry after the first commit affects zero rows and 409s instead of granting the attachment twice",
		staffauth.Middleware(d.DB)(offer.AcceptHandler()))
	ir.Exempt("POST /api/practices/{practiceId}/offers/{offerId}/decline",
		"documented idempotent by design (#229): declining an already-declined Offer succeeds again rather than erroring",
		staffauth.Middleware(d.DB)(offer.DeclineHandler()))
	ir.Exempt("POST /api/practices/{practiceId}/offers/{offerId}/withdraw",
		"state-guarded UPDATE ... WHERE state = 'offered'; a retry after the first commit affects zero rows and 409s instead of withdrawing twice",
		staffauth.Middleware(d.DB)(offer.WithdrawHandler()))
	ir.Exempt("POST /api/practices/{practiceId}/push-subscriptions",
		"upsert; registering the same endpoint again is a no-op update, not a duplicate row",
		staffauth.Middleware(d.DB)(pushsub.RegisterHandler()))
	ir.Exempt("DELETE /api/practices/{practiceId}/push-subscriptions",
		"delete; a retry after the first succeeds deletes nothing further",
		staffauth.Middleware(d.DB)(pushsub.UnregisterHandler()))
	ir.Replayable("POST /api/practices/{practiceId}/engagements/{engagementId}/portal-invite",
		staffauth.Middleware(d.DB)(idempotency.Wrap(portalinvite.InviteHandler(d.NudgeEnqueuer))))
}
