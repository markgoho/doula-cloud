package activity

import "slices"

// SubjectEngagement is the subject_kind every Engagement-scoped write site
// in this ticket (#476) uses -- Contract, Visit, Plan Instance, Offer,
// Invoice/Payment and portal-invite actions all name subject_id =
// engagement_id rather than their own record's id. A per-child
// subject_kind ('contract', 'offer', ...) was considered and rejected:
// the read AC is "an Engagement's entries", and the only index
// (activity_subject) is a (subject_kind, subject_id) prefix, so a single
// subject keeps that read to one indexed range scan with one cursor.
// The record an event is really about (a Contract id, an Offer id, ...)
// still lives in Diff, for a reader that needs it.
const SubjectEngagement = "engagement"

// ClientAction is one of the fixed action strings a write site records
// against SubjectClient. Only #619's own action is named here so far --
// client/events.go's created/updated/erased are that package's own
// eventType, sealed diffs and all (ADR-0027), and are not in this
// vocabulary.
type ClientAction string

// ActionPortalSignInAddressChanged records a Client moving her Portal
// Account's sign-in address to a mailbox she has just proved she reads
// (#619, ADR-0026). Always a ClientActor: it is hers to change, and no
// Staff path reaches it.
//
// Its diff is deliberately empty. ADR-0015 makes the sign-in address her
// login, not the Practice's contact detail -- writing either the old or
// the new address into a ledger the Practice reads would put her private
// login in front of it for the first time. The row answers "who did it
// and when", which is what CLAUDE.md's audit-trail expectation asks of
// it; the address itself is on portal_accounts, where it belongs.
const ActionPortalSignInAddressChanged ClientAction = "portal_sign_in_address_changed"

// ActionPortalSessionsEnded records a Client signing herself out of
// every device from inside the portal (#618, ADR-0026). Always a
// ClientActor: it is her own "sign out everywhere", not Staff's
// administrative revoke (staffauth.EndSessionsHandler, which records
// against "membership" instead). No diff: like the address change
// above, there is nothing to say but who did it and when.
const ActionPortalSessionsEnded ClientAction = "portal_sessions_ended"

// SubjectClient is the subject_kind client/events.go's recordEvent writes
// every create/update/erase event against (subject_id = client_id).
// Exported for the same reason SubjectEngagement is: a caller building a
// read-side query or gate rule (activitygate's client Rule) imports this
// rather than repeating the literal, so the write side and any read side
// can't drift apart.
const SubjectClient = "client"

// SubjectPractice is the subject_kind a write site records against a
// Practice's own id: staffauth's PutMFARequiredHandler (the "require MFA
// for all staff" switch) and export.Handler (#288's whole-Practice
// export) both write against it. Exported for the same reason
// SubjectClient is -- a second write site is exactly the drift this
// prevents.
const SubjectPractice = "practice"

// SystemActorName is what ActorSystem renders as -- ADR-0022: "Doula
// Cloud", never "System". Every reader that resolves an activity row's
// actor to a display name falls back to this constant for actor_kind =
// 'system', rather than each caller inventing its own string.
const SystemActorName = "Doula Cloud"

// EngagementAction is one of the fixed action strings a write site
// records against SubjectEngagement. Named in one place so a write site
// can't typo a string the read side's money filter (see IsMoney) has to
// match exactly -- #476's punch list of every Engagement-scoped state
// change ADR-0022's ledger names.
type EngagementAction string

// The full action vocabulary #476 asks every Engagement-scoped write
// site to record.
const (
	ActionEngagementCreated   EngagementAction = "engagement_created"
	ActionEngagementCompleted EngagementAction = "engagement_completed"

	// ActionCarePhaseChanged records an Engagement's intake -> active
	// move (#253, ADR-0015), manual or automatic -- the only transition
	// on engagements.status besides reaching 'completed', which stays
	// its own ActionEngagementCompleted. Written by
	// engagement.TransitionHandler.
	ActionCarePhaseChanged EngagementAction = "care_phase_changed"

	ActionContractCreated EngagementAction = "contract_created"
	ActionContractSent    EngagementAction = "contract_sent"
	ActionContractSigned  EngagementAction = "contract_signed"
	ActionContractVoided  EngagementAction = "contract_voided"

	// ActionContractAmountOverridden records an Owner or an Admin
	// overriding a Contract's rate-card-derived amount (#967) -- Diff
	// carries amountCentsBefore and amountCentsAfter, mirroring
	// practicerate's own rateDiff shape. Never fired once a Contract is
	// signed or voided: PutContractAmountHandler refuses both (#967's
	// AC, "a signed Contract's amount never changes for any reason").
	ActionContractAmountOverridden EngagementAction = "contract_amount_overridden"

	// ActionContractAmountRepriced records a Contract's amount moving
	// because the Practice's rate card changed underneath it (#968) --
	// Diff carries amountCentsBefore and amountCentsAfter, the same
	// shape ActionContractAmountOverridden's diff already uses. Always a
	// SystemActor (ADR-0022): nobody performed this, practicerate's
	// PutRateHandler did, as a side effect of the rate change a Staff
	// member actually made (which gets its own practice_rate_changed
	// entry against the Practice, not this one). Never fired for a
	// Contract that is signed, voided, or has amount_overridden set --
	// PutRateHandler's reprice pass excludes all three.
	ActionContractAmountRepriced EngagementAction = "contract_amount_repriced"

	// ActionContractVoidRequested and ActionContractVoidDeclined record
	// #971's own two acts: a Doula (employed or, on a granted attachment,
	// a contractor) asking for a Signed Contract to be voided, and an
	// Owner or Admin refusing that ask with a reason of their own --
	// distinct from ActionContractVoided, which is the Owner/Admin acting
	// on the ask by actually voiding. Never fired by the same request:
	// PostVoidRequestHandler only ever writes the first, the decline
	// handler only ever the second.
	ActionContractVoidRequested EngagementAction = "contract_void_requested"
	ActionContractVoidDeclined  EngagementAction = "contract_void_declined"

	ActionVisitLogged     EngagementAction = "visit_logged"
	ActionVisitReassigned EngagementAction = "visit_reassigned"

	// DiffKeyAssignedStaffIDBefore and DiffKeyAssignedStaffIDAfter are
	// the two keys an ActionVisitReassigned diff carries, naming the
	// Staff member the Visit came off and the one it went to -- the
	// same before/after convention ActionVisitScheduled's
	// scheduledAtBefore/scheduledAtAfter already uses, not a third one.
	// Both hold ids; a reader resolves them to names on the read, the
	// way actorName is resolved (#887). They live here rather than in
	// visit/ because the write side and the Engagement ledger's read
	// both name them, and one constant is what keeps the two from
	// drifting apart.
	DiffKeyAssignedStaffIDBefore = "assignedStaffIdBefore"
	DiffKeyAssignedStaffIDAfter  = "assignedStaffIdAfter"

	// ActionVisitScheduled records a Visit's scheduled date/time being
	// set, changed or cleared (#250) -- its own action, distinct from
	// ActionVisitLogged (the row being typed) and ActionVisitReassigned
	// (which Doula covers it). Deliberately absent from staffingActions
	// below: unlike a reassignment, which names which Doula covers a
	// Visit (a Practice roster fact), when a Visit happens is a fact
	// about a Client's own care, the same footing ActionVisitLogged
	// already stands on -- neither is excluded from her own portal
	// ledger.
	ActionVisitScheduled EngagementAction = "visit_scheduled"

	ActionPlanInstanceEdited EngagementAction = "plan_instance_edited"

	// ActionVisitNotesEdited records a Visit's free-text notes being
	// written or re-written (#251) -- staff-only content (ADR-0006), but
	// the action itself is not: deliberately absent from staffingActions
	// below, the same footing ActionPlanInstanceEdited already stands on.
	// A Client may see that her Visit's notes were updated; she does not
	// see what they say, since Diff never carries the text (see notes.go).
	ActionVisitNotesEdited EngagementAction = "visit_notes_edited"

	// ActionBirthPlanAcknowledged records a Client confirming she has
	// read her Birth Plan (#301, v1: acknowledgement only -- she cannot
	// edit a field or suggest a change). Always a ClientActor: it is her
	// own act, never Staff's on her behalf. A Staff edit that changes the
	// Plan Instance's stored answers clears the acknowledgement itself
	// (plans.PutInstanceHandler) but records no activity row of its own --
	// that edit already fires ActionPlanInstanceEdited.
	ActionBirthPlanAcknowledged EngagementAction = "birth_plan_acknowledged"

	ActionOfferSent       EngagementAction = "offer_sent"
	ActionOfferAccepted   EngagementAction = "offer_accepted"
	ActionOfferDeclined   EngagementAction = "offer_declined"
	ActionOfferSuperseded EngagementAction = "offer_superseded"
	ActionOfferWithdrawn  EngagementAction = "offer_withdrawn"

	ActionInvoiceRaised EngagementAction = "invoice_raised"
	ActionInvoicePaid   EngagementAction = "invoice_paid"

	// ActionPaymentRecorded records a manually recorded Payment (#271) --
	// a check, a bank transfer, or cash, never money Stripe itself moved
	// (that stays ActionInvoicePaid, unchanged). Always a StaffActor: an
	// Owner or Admin typed it in, unlike a Stripe payment, which is the
	// Client's own act.
	ActionPaymentRecorded EngagementAction = "payment_recorded"
	// ActionInvoiceVoided and ActionInvoiceWrittenOff record the two
	// Staff-initiated ways a by-hand Invoice leaves 'open' without being
	// paid (#271) -- a mistyped amount, or one the Practice has given up
	// collecting. Neither reverses a Payment; both act on an Invoice that
	// was never paid.
	ActionInvoiceVoided     EngagementAction = "invoice_voided"
	ActionInvoiceWrittenOff EngagementAction = "invoice_written_off"

	ActionPortalInviteSent EngagementAction = "portal_invite_sent"

	// ActionPortalAccountProvisioned records the Portal Account
	// portalinvite.acceptInvite creates (#616) -- always a ClientActor,
	// since accepting her own invitation is the one path that reaches it.
	ActionPortalAccountProvisioned EngagementAction = "portal_account_provisioned"

	// ActionPortalAccountLinked records the other shape acceptInvite's
	// accept can take (#309, ADR-0015): the caller already holds a
	// Portal Account for this sign-in address, and accepting attaches
	// this Engagement's Client to it rather than minting a new one.
	// Distinct from ActionPortalAccountProvisioned because no Portal
	// Account came into being here -- an existing one gained reach into
	// a new Practice.
	ActionPortalAccountLinked EngagementAction = "portal_account_linked"

	// #303: a Client's own push-notification preference change, recorded
	// against her Engagement (notificationpref's PUT handler). Neither
	// action belongs in moneyActions or staffingActions below -- it is
	// neither the Practice's price nor its roster -- so it surfaces on both
	// the Client's own portal ledger and the Staff-side Engagement feed
	// unfiltered.
	ActionPushNotificationsEnabled  EngagementAction = "push_notifications_enabled"
	ActionPushNotificationsDisabled EngagementAction = "push_notifications_disabled"
)

// moneyActions is what ADR-0008's read table keeps off an employed
// Doula's ledger, and off a contractor's alongside it: the Practice's
// price (Contract) and its Invoice/payment history. A contractor's own
// agreed fee is a different fact -- it lives on the Offer she accepted,
// which is not in this set and stays on her ledger.
var moneyActions = map[EngagementAction]bool{
	ActionContractCreated:          true,
	ActionContractSent:             true,
	ActionContractSigned:           true,
	ActionContractVoided:           true,
	ActionContractAmountOverridden: true,
	ActionContractAmountRepriced:   true,
	// ActionContractVoidRequested and ActionContractVoidDeclined join the
	// rest of the Contract lifecycle here, not because either names a
	// dollar figure -- a request's reason and a decline's reason carry
	// none -- but because every entry above already groups by *entity*
	// (any ledger row about the Contract) rather than by whether that
	// specific row happens to show a price, and a contractor's own
	// ledger read hides the whole entity the same way (ADR-0008: "Money
	// entries in the activity ledger ... contractor: ✗"). Splitting these
	// two out as the one Contract action a contractor can read would be
	// the drift, not the consistency.
	ActionContractVoidRequested: true,
	ActionContractVoidDeclined:  true,
	ActionInvoiceRaised:         true,
	ActionInvoicePaid:           true,
	ActionPaymentRecorded:       true,
	ActionInvoiceVoided:         true,
	ActionInvoiceWrittenOff:     true,
}

// MoneyActions returns every action ADR-0008 keeps Owner/Admin-only,
// sorted for a deterministic query string. activitygate's engagement Rule
// (api/internal/activitygate) adapts this into its RestrictedActions,
// which engagement.ListActivityHandler builds its SQL exclusion clause
// from -- never a hand-copied literal list, so the write side (which
// names these actions) and the read side (which filters them) can never
// drift apart.
func MoneyActions() []EngagementAction {
	out := make([]EngagementAction, 0, len(moneyActions))
	for a := range moneyActions {
		out = append(out, a)
	}
	slices.Sort(out)
	return out
}

// staffingActions is what CONTEXT.md's Activity entry keeps off a
// Client's own portal ledger (#486): "she reads her own Activity ...
// never who inside the Practice did what." An Offer is Doula staffing --
// who was asked, who accepted, who was bumped -- and a Visit
// reassignment is which Doula covers it, not that a Visit happened; both
// are facts about the Practice's own roster, not about her. Money
// actions (moneyActions above) are a different, Staff-role-only cut and
// are deliberately absent here: CONTEXT.md also says "her money", so a
// Client keeps every Contract and Invoice entry on her own Engagement.
var staffingActions = map[EngagementAction]bool{
	ActionOfferSent:       true,
	ActionOfferAccepted:   true,
	ActionOfferDeclined:   true,
	ActionOfferSuperseded: true,
	ActionOfferWithdrawn:  true,
	ActionVisitReassigned: true,
}

// StaffingActions returns every action CONTEXT.md's Activity entry keeps
// off a Client's own portal ledger, sorted for a deterministic query
// string -- the same shape MoneyActions already gives
// engagement.ListActivityHandler, so a caller building a SQL exclusion
// clause for the Client-portal reader never hand-copies the literal list.
func StaffingActions() []EngagementAction {
	out := make([]EngagementAction, 0, len(staffingActions))
	for a := range staffingActions {
		out = append(out, a)
	}
	slices.Sort(out)
	return out
}
