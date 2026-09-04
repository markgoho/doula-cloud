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
