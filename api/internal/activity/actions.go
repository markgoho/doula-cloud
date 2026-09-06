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

// SubjectClient is the subject_kind client/events.go's recordEvent writes
// every create/update/erase event against (subject_id = client_id).
// Exported for the same reason SubjectEngagement is: a caller building a
// read-side query or gate rule (activitygate's client Rule) imports this
// rather than repeating the literal, so the write side and any read side
// can't drift apart.
const SubjectClient = "client"

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

	// ActionCarePhaseChanged has no writer yet: #253 ("An Engagement's
	// status never changes") is the still-open ticket that will add the
	// only other transition on engagements.status (intake -> active);
	// completion is the sole transition that exists today and is its
	// own ActionEngagementCompleted, not this one. #253's build must
	// call activity.Record with this action when it lands.
	ActionCarePhaseChanged EngagementAction = "care_phase_changed"

	ActionContractCreated EngagementAction = "contract_created"
	ActionContractSent    EngagementAction = "contract_sent"
	ActionContractSigned  EngagementAction = "contract_signed"
	ActionContractVoided  EngagementAction = "contract_voided"

	ActionVisitLogged     EngagementAction = "visit_logged"
	ActionVisitReassigned EngagementAction = "visit_reassigned"

	ActionPlanInstanceEdited EngagementAction = "plan_instance_edited"

	ActionOfferSent       EngagementAction = "offer_sent"
	ActionOfferAccepted   EngagementAction = "offer_accepted"
	ActionOfferDeclined   EngagementAction = "offer_declined"
	ActionOfferSuperseded EngagementAction = "offer_superseded"
	ActionOfferWithdrawn  EngagementAction = "offer_withdrawn"

	ActionInvoiceRaised EngagementAction = "invoice_raised"
	ActionInvoicePaid   EngagementAction = "invoice_paid"

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
	ActionContractCreated: true,
	ActionContractSent:    true,
	ActionContractSigned:  true,
	ActionContractVoided:  true,
	ActionInvoiceRaised:   true,
	ActionInvoicePaid:     true,
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
