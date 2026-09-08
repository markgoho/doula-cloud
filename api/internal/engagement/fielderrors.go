package engagement

// The sentences this package puts in APIError.Details (#488), following
// the GOV.UK rules app/src/lib/formErrors.ts is already gated on and
// apierr's TestDetailsWording holds this side to.
//
// Each one is the sentence the client's own check already shows for that
// control, on the reasoning formErrors.ts's module comment gives: the
// same refusal has to read the same way wherever it happens.

// fieldPregnancyEndedOn is the pregnancyEndedOn json tag, which three
// of the birth-outcome refusals key their Details under.
const fieldPregnancyEndedOn = "pregnancyEndedOn"

const (
	// MsgStatusUnknown is the status field on the transition endpoint.
	// Nobody reaches it from the Engagement page, whose only two paths
	// are named buttons; it is here for a caller that sent its own body,
	// and it still names the field.
	MsgStatusUnknown = "Choose whether this Engagement is active or completed"
	// MsgEndingReasonNeeded is the endingReason field on the completion
	// form -- the same sentence that form's own check shows.
	MsgEndingReasonNeeded = "Select why this Engagement is ending"
	// MsgBirthOutcomeUnknown is the birthOutcome field on the birth
	// outcome form, matching its own client-side check.
	MsgBirthOutcomeUnknown = "Select what happened to the pregnancy"
	// MsgEndedOnNeeded answers a birth outcome of 'live_birth' or 'loss'
	// sent with no date: those two say the pregnancy ended, and when it
	// ended is the other half of that fact.
	MsgEndedOnNeeded = "Enter the date the pregnancy ended"
	// MsgEndedOnMalformed answers a date that will not parse.
	MsgEndedOnMalformed = "Enter the date the pregnancy ended as a real date, like 2027-04-23"
	// MsgEndedOnUnwanted answers a date sent with no outcome at all --
	// the one refusal here whose fix is to remove something rather than
	// supply it.
	MsgEndedOnUnwanted = "Remove the date, or say what happened to the pregnancy"
)
