package engagement

import (
	"net/http"

	"doula-cloud/api/internal/activity"
	"doula-cloud/api/internal/apierr"
)

// The three kinds the record's Activity filter names (#2755), as the
// ?kind= parameter spells them. No kind is every entry.
const (
	paramKind  = "kind"
	kindCare   = "care"
	kindMoney  = "money"
	kindDoulas = "doulas"
)

const (
	// MsgActivityKindUnknown is the kind parameter on the Activity read. The
	// screen offers the three kinds as named links; this is for a caller that
	// typed its own.
	MsgActivityKindUnknown = "Choose care, money or doulas"
	// MsgActivityKindNotShown is the sentence over it.
	MsgActivityKindNotShown = "Those entries could not be shown."
)

// activityKinds splits every action an Engagement's ledger holds into the
// three kinds the record filters by, each action in exactly one.
//
//   - care: Visits, notes, the Care Plan and Birth Plan, the birth outcome,
//     the care phase and the Engagement's own standing facts, the Client's
//     portal access and push preferences.
//   - money: the Contract's lifecycle and price, and every Invoice,
//     payment and refund.
//   - doulas: who is on the birth (attachments, on-call narrowing and rule,
//     gaps), Offers, and what the Practice pays a contractor Doula.
//
// TestActivityKinds_EveryEngagementActionIsInExactlyOne holds the split
// against activity/actions.go.
var activityKinds = map[string][]activity.EngagementAction{
	kindCare: {
		activity.ActionEngagementCreated,
		activity.ActionEngagementCompleted,
		activity.ActionEngagementReopened,
		activity.ActionCarePhaseChanged,
		activity.ActionKindChanged,
		activity.ActionDueDateChanged,
		activity.ActionBirthOutcomeRecorded,
		activity.ActionVisitLogged,
		activity.ActionVisitReassigned,
		activity.ActionVisitScheduled,
		activity.ActionVisitNotesEdited,
		activity.ActionCarePlanStarted,
		activity.ActionCarePlanEdited,
		activity.ActionBirthPlanStarted,
		activity.ActionBirthPlanEdited,
		activity.ActionBirthPlanAcknowledged,
		activity.ActionPortalInviteSent,
		activity.ActionPortalAccountProvisioned,
		activity.ActionPortalAccountLinked,
		activity.ActionPortalInviteRefused,
		activity.ActionPushNotificationsEnabled,
		activity.ActionPushNotificationsDisabled,
	},
	kindMoney: {
		activity.ActionContractCreated,
		activity.ActionContractEdited,
		activity.ActionContractSent,
		activity.ActionContractSigned,
		activity.ActionContractOpened,
		activity.ActionContractReminded,
		activity.ActionContractVoidedByErasure,
		activity.ActionContractVoided,
		activity.ActionContractVoidRequested,
		activity.ActionContractVoidDeclined,
		activity.ActionContractPriced,
		activity.ActionContractAmountOverridden,
		activity.ActionContractAmountRepriced,
		activity.ActionInvoiceRaised,
		activity.ActionInvoicePaid,
		activity.ActionInvoiceVoided,
		activity.ActionInvoiceWrittenOff,
		activity.ActionInvoiceNotSent,
		activity.ActionInvoicePaymentFailed,
		activity.ActionInvoicePaymentProcessing,
		activity.ActionPaymentRecorded,
		activity.ActionPaymentReversed,
		activity.ActionPaymentRefunded,
	},
	kindDoulas: {
		activity.ActionDoulaAttached,
		activity.ActionDoulaTakenOff,
		activity.ActionOnCallNarrowingChanged,
		activity.ActionOnCallRuleChanged,
		activity.ActionCoverageGapCreated,
		activity.ActionCoverageGapUpdated,
		activity.ActionCoverageGapCleared,
		activity.ActionOfferSent,
		activity.ActionOfferAccepted,
		activity.ActionOfferDeclined,
		activity.ActionOfferSuperseded,
		activity.ActionOfferWithdrawn,
		activity.ActionContractorFeePaid,
		activity.ActionContractorFeePaymentReversed,
	},
}

// activityKindActions is the action names of ?kind=kind: nil for no kind,
// ok false for one that is not a kind.
func activityKindActions(kind string) (actions []string, ok bool) {
	if kind == "" {
		return nil, true
	}
	list, ok := activityKinds[kind]
	if !ok {
		return nil, false
	}
	actions = make([]string, len(list))
	for i, a := range list {
		actions[i] = string(a)
	}
	return actions, true
}

// parseActivityKind reads ?kind=, writing the 400 itself for one that is
// not a kind.
func parseActivityKind(w http.ResponseWriter, r *http.Request) (actions []string, ok bool) {
	actions, ok = activityKindActions(r.URL.Query().Get(paramKind))
	if !ok {
		apierr.Write(w, http.StatusBadRequest, apierr.CodeInvalidArgument, MsgActivityKindNotShown,
			map[string]string{paramKind: MsgActivityKindUnknown})
	}
	return actions, ok
}
