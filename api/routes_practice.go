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
func registerPracticeRoutes(mux *http.ServeMux, g *staffauth.GatedRouter, d Deps) {
	g.Get("/api/practices/{practiceId}/session", staffauth.AnyStaff, http.HandlerFunc(practiceSessionHandler))
	// Roles and employment type are edited together on one surface
	// (RA-G2, #261) -- ADR-0008 makes them the two halves of what a
	// person is at a Practice, so there is one endpoint, not two.
	mux.Handle("PATCH /api/practices/{practiceId}/staff/{staffId}/membership",
		staffauth.Middleware(d.DB)(staffauth.UpdateMembershipHandler()))
	// The route #291 found missing: without it a roster row nobody wants
	// can never be taken off.
	mux.Handle("DELETE /api/practices/{practiceId}/staff/{staffId}/membership",
		staffauth.Middleware(d.DB)(staffauth.RemoveMembershipHandler()))
	mux.Handle("POST /api/practices/{practiceId}/staff/invitations",
		staffauth.Middleware(d.DB)(idempotency.Wrap(staffauth.InviteHandler(d.NudgeEnqueuer))))
	mux.Handle("POST /api/practices/{practiceId}/staff/invitations/{invitationId}/revoke",
		staffauth.Middleware(d.DB)(staffauth.RevokeInvitationHandler()))
	// Staff roster -- members and pending invitations both: Owner and
	// Admin only (ADR-0008's read table) -- a Doula has no reason to see
	// the full roster.
	g.Get("/api/practices/{practiceId}/staff", ownerAndAdmin, staffauth.ListStaffHandler())
	mux.Handle("DELETE /api/practices/{practiceId}/staff/{staffId}/sessions",
		staffauth.Middleware(d.DB)(staffauth.EndSessionsHandler(d.NudgeEnqueuer)))
	// Credit balance and ledger: Owner and Admin only (ADR-0008).
	g.Get("/api/practices/{practiceId}/billing", ownerAndAdmin, billing.GetBalanceHandler())
	mux.Handle("POST /api/practices/{practiceId}/billing/purchases",
		staffauth.Middleware(d.DB)(billing.PostPurchaseHandler(d.StripeClient)))
	mux.Handle("POST /api/practices/{practiceId}/payments/connect",
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
	mux.Handle("PUT /api/practices/{practiceId}/website",
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
	mux.Handle("POST /api/practices/{practiceId}/clients",
		staffauth.Middleware(d.DB)(idempotency.Wrap(client.CreateHandler())))
	g.Get("/api/practices/{practiceId}/clients/{clientId}", staffauth.AnyStaff, client.DetailHandler())
	mux.Handle("PUT /api/practices/{practiceId}/clients/{clientId}",
		staffauth.Middleware(d.DB)(client.EditHandler()))
	// ADR-0017's Engagement Request (#398): the ask for paid work with a
	// Client, and the act that creates an Engagement. Request is any
	// Staff member but a contractor Doula (enforced here and,
	// independently, by engagement_requests_insert's RLS policy);
	// approve/refuse are Owner/Admin; withdraw is the requester alone,
	// so it carries no role declaration.
	mux.Handle("POST /api/practices/{practiceId}/clients/{clientId}/engagement-requests",
		staffauth.Middleware(d.DB)(idempotency.Wrap(engagementrequest.RequestHandler(d.DB, d.NudgeEnqueuer))))
	mux.Handle("POST /api/practices/{practiceId}/engagement-requests/{requestId}/approve",
		staffauth.Middleware(d.DB)(engagementrequest.ApproveHandler(d.DB, d.NudgeEnqueuer)))
	mux.Handle("POST /api/practices/{practiceId}/engagement-requests/{requestId}/refuse",
		staffauth.Middleware(d.DB)(engagementrequest.RefuseHandler()))
	mux.Handle("POST /api/practices/{practiceId}/engagement-requests/{requestId}/withdraw",
		staffauth.Middleware(d.DB)(engagementrequest.WithdrawHandler()))
	g.Get("/api/practices/{practiceId}/engagements/{engagementId}", staffauth.AnyStaff, engagement.DetailHandler())
	// Completing an Engagement runs ADR-0008's cascade -- open Offers
	// withdrawn, open attachments ended -- so it is one endpoint, not a
	// generic status PATCH a caller could half-apply.
	mux.Handle("POST /api/practices/{practiceId}/engagements/{engagementId}/complete",
		staffauth.Middleware(d.DB)(engagement.CompleteHandler()))
	g.Get("/api/practices/{practiceId}/engagements/{engagementId}/visits", staffauth.AnyStaff, visit.ListHandler())
	// staffauth.AttachingWrite is ADR-0008's write-side seam: every
	// Engagement-scoped write below attaches the acting Doula, accrued,
	// once it has succeeded. It is applied here rather than inside each
	// handler so a new Engagement write cannot quietly fall off the list
	// -- #231's whole argument against a second hand-maintained registry.
	mux.Handle("POST /api/practices/{practiceId}/engagements/{engagementId}/visits",
		staffauth.Middleware(d.DB)(staffauth.AttachingWrite(visit.CreateHandler())))
	mux.Handle("PATCH /api/practices/{practiceId}/engagements/{engagementId}/visits/{visitId}",
		staffauth.Middleware(d.DB)(staffauth.AttachingWrite(visit.ReassignHandler())))
	g.Get("/api/practices/{practiceId}/engagements/{engagementId}/messages", staffauth.AnyStaff, message.ListHandler())
	mux.Handle("POST /api/practices/{practiceId}/engagements/{engagementId}/messages",
		staffauth.Middleware(d.DB)(staffauth.AttachingWrite(idempotency.Wrap(message.CreateHandler(d.Store, d.Pusher)))))
	g.Get("/api/practices/{practiceId}/engagements/{engagementId}/messages/{messageId}/attachment", staffauth.AnyStaff, message.AttachmentHandler(d.Store))
	// Plan Template and Contract Template: every Staff role (ADR-0008),
	// no attachment narrowing -- a Template isn't Engagement-scoped.
	g.Get("/api/practices/{practiceId}/plan-templates/{planType}", staffauth.AnyStaff, plans.GetTemplateHandler())
	mux.Handle("PUT /api/practices/{practiceId}/plan-templates/{planType}",
		staffauth.Middleware(d.DB)(plans.PutTemplateHandler()))
	// ADR-0017's Client Field Template settings screen (#399): the field
	// list an Owner or Admin defines for a Client's Practice-defined
	// layer. Sibling of Plan Templates above -- read by any Staff member
	// (the definitions carry nothing secret), written by an Owner or
	// Admin alone (client_field_templates_insert/_update, 00050, enforce
	// the same rule in RLS).
	g.Get("/api/practices/{practiceId}/client-field-template", staffauth.AnyStaff, clientfieldtemplate.GetHandler())
	mux.Handle("PUT /api/practices/{practiceId}/client-field-template",
		staffauth.Middleware(d.DB)(clientfieldtemplate.PutHandler()))
	mux.Handle("POST /api/practices/{practiceId}/engagements/{engagementId}/plans/{planType}",
		staffauth.Middleware(d.DB)(staffauth.AttachingWrite(plans.PostInstanceHandler())))
	g.Get("/api/practices/{practiceId}/engagements/{engagementId}/plans/{planType}", staffauth.AnyStaff, plans.GetInstanceHandler())
	mux.Handle("PUT /api/practices/{practiceId}/engagements/{engagementId}/plans/{planType}",
		staffauth.Middleware(d.DB)(staffauth.AttachingWrite(plans.PutInstanceHandler())))
	g.Get("/api/practices/{practiceId}/contract-template", staffauth.AnyStaff, contracts.GetTemplateHandler())
	mux.Handle("PUT /api/practices/{practiceId}/contract-template",
		staffauth.Middleware(d.DB)(contracts.PutTemplateHandler()))
	mux.Handle("POST /api/practices/{practiceId}/engagements/{engagementId}/contract",
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
	mux.Handle("PUT /api/practices/{practiceId}/engagements/{engagementId}/contract",
		staffauth.Middleware(d.DB)(staffauth.AttachingWrite(contracts.PutContractHandler())))
	mux.Handle("POST /api/practices/{practiceId}/engagements/{engagementId}/contract/send",
		staffauth.Middleware(d.DB)(staffauth.AttachingWrite(contracts.PostSendContractHandler(d.Pusher))))
	mux.Handle("POST /api/practices/{practiceId}/engagements/{engagementId}/contract/void",
		staffauth.Middleware(d.DB)(staffauth.AttachingWrite(contracts.PostVoidContractHandler())))
	// The Signed PDF is a rendered, unredactable document -- it can't be
	// split into scope/money views the way the JSON Contract read can, so
	// it follows the money row: Owner/Admin only.
	g.Get("/api/practices/{practiceId}/engagements/{engagementId}/contract/pdf", ownerAndAdmin, contracts.GetSignedContractPDFHandler(d.Store))
	mux.Handle("POST /api/practices/{practiceId}/engagements/{engagementId}/contract/invoices",
		staffauth.Middleware(d.DB)(payments.PostInvoiceHandler(d.PaymentsClient)))
	// Invoice history rides the same money row as Contract money -- see
	// above. A contractor's own-fee narrowing (rather than an outright
	// no) is #317's to build once the Offer/Attachment flow exists.
	g.Get("/api/practices/{practiceId}/engagements/{engagementId}/contract/invoices", ownerAndAdmin, payments.GetInvoicesHandler())
	// ADR-0008's Offer flow (#317). The Practice side is Owner/Admin --
	// making an Offer, taking it back, and reading who has been asked,
	// which names people and so follows the Staff-roster row of the read
	// table. The Doula side is her own inbox and her own decisions, so it
	// is scoped to her staff_id in SQL rather than by a role declaration.
	mux.Handle("POST /api/practices/{practiceId}/engagements/{engagementId}/offers",
		staffauth.Middleware(d.DB)(idempotency.Wrap(offer.CreateHandler(d.NudgeEnqueuer))))
	g.Get("/api/practices/{practiceId}/engagements/{engagementId}/offers", ownerAndAdmin, offer.EngagementListHandler())
	g.Get("/api/practices/{practiceId}/offers", staffauth.AnyStaff, offer.InboxHandler())
	mux.Handle("POST /api/practices/{practiceId}/offers/{offerId}/accept",
		staffauth.Middleware(d.DB)(offer.AcceptHandler()))
	mux.Handle("POST /api/practices/{practiceId}/offers/{offerId}/decline",
		staffauth.Middleware(d.DB)(offer.DeclineHandler()))
	mux.Handle("POST /api/practices/{practiceId}/offers/{offerId}/withdraw",
		staffauth.Middleware(d.DB)(offer.WithdrawHandler()))
	mux.Handle("POST /api/practices/{practiceId}/push-subscriptions",
		staffauth.Middleware(d.DB)(pushsub.RegisterHandler()))
	mux.Handle("DELETE /api/practices/{practiceId}/push-subscriptions",
		staffauth.Middleware(d.DB)(pushsub.UnregisterHandler()))
	mux.Handle("POST /api/practices/{practiceId}/engagements/{engagementId}/portal-invite",
		staffauth.Middleware(d.DB)(idempotency.Wrap(portalinvite.InviteHandler(d.NudgeEnqueuer))))
}
