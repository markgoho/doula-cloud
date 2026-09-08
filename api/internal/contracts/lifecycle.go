package contracts

import "fmt"

// Status is a Contract's lifecycle stage -- exported so a route in
// another package (payments' Invoice-create handler) can name a
// required state without redeclaring the contract_status enum's
// literals for itself.
type Status string

// The four stages of a Contract's lifecycle, matching the contract_status
// enum column (00016_contracts.sql).
const (
	StatusDraft  Status = "draft"
	StatusSent   Status = "sent"
	StatusSigned Status = "signed"
	StatusVoided Status = "voided"
)

// Transition is one named Contract lifecycle move and the single status
// it requires beforehand. #275 found that the Invoice-create route in
// payments never consulted any of this at all -- the other four routes
// declared below (edit, send, sign, void) each carried their own
// hand-written `status != "..."` comparison, and Invoice had simply never
// been given one. This is the one declaration every route that changes
// or bills a Contract now consults, via Check, instead of restating its
// own literal comparison: a sixth route added later gets the question
// put to it by needing a Transition to check against, rather than being
// free to skip it the way Invoice was. See lifecycle_test.go's
// source-scan guardrail, which fails if any of the five route files
// below stops calling the Transition declared for it.
type Transition struct {
	// PastTense names the move for a refusal message ("sent", "voided",
	// "signed", "billed", "edited").
	PastTense string
	// Requires is the one status a Contract must be in for this move to
	// run; anything else is refused.
	Requires Status
}

// TransitionEdit is PutContractHandler's precondition: only a Draft
// Contract's merge field values may be replaced.
var TransitionEdit = Transition{PastTense: "edited", Requires: StatusDraft}

// TransitionSend is PostSendContractHandler's precondition: only a Draft
// Contract may be sent for signature.
var TransitionSend = Transition{PastTense: "sent", Requires: StatusDraft}

// TransitionSign is ClientPostSignContractHandler's precondition: only a
// Sent Contract may be signed.
var TransitionSign = Transition{PastTense: "signed", Requires: StatusSent}

// TransitionVoid is PostVoidContractHandler's precondition: only a Signed
// Contract may be voided.
var TransitionVoid = Transition{PastTense: "voided", Requires: StatusSigned}

// TransitionBill is payments.PostInvoiceHandler's precondition (#275):
// only a Signed, still-in-force Contract may be billed. A Draft, a Sent
// (unsigned) or a Voided Contract is refused before payments ever asks
// anything of Stripe.
var TransitionBill = Transition{PastTense: "billed", Requires: StatusSigned}

// TransitionOverrideAmount is PutContractAmountHandler's precondition
// (#967): only a Draft Contract's amount may be overridden, the same
// Requires TransitionEdit carries -- an Owner or Admin correction is one
// more kind of Draft-only edit, not a new lifecycle stage of its own.
// This is stricter than #967's AC literally requires ("a signed
// Contract's amount never changes"), which would also permit overriding
// a Sent Contract's amount; Draft-only is the narrower, safer reading,
// consistent with every other merge-field edit already being refused
// once a Contract leaves Draft (TransitionEdit).
var TransitionOverrideAmount = Transition{PastTense: "amount overridden", Requires: StatusDraft}

// Transitions is every declared Contract lifecycle precondition, walked
// by lifecycle_test.go's guardrail.
var Transitions = []Transition{TransitionEdit, TransitionSend, TransitionSign, TransitionVoid, TransitionBill, TransitionOverrideAmount}

// Check reports whether current satisfies t (ok=true, no refusal
// message), or, if not, the 409 message a caller should write -- naming
// the state the Contract is actually in, per #275's acceptance criteria,
// rather than only "not permitted".
func (t Transition) Check(current Status) (ok bool, refusal string) {
	if current == t.Requires {
		return true, ""
	}
	return false, fmt.Sprintf("contract cannot be %s: it is %s", t.PastTense, current)
}
