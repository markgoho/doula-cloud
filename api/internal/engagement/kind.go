package engagement

// Kind is an Engagement's kind -- birth or postpartum, what the Practice
// sold (CONTEXT.md's Engagement entry). Landed by #308
// (00042_client_intake_schema.sql), mutable in both directions per
// ADR-0015, and staff-only: CONTEXT.md gives it no Client word, so no
// value of this type may reach a Client-facing response.
type Kind string

// The two values Kind can hold, per CONTEXT.md's Engagement entry.
const (
	KindBirth      Kind = "birth"
	KindPostpartum Kind = "postpartum"
)

// BirthPlanInputs holds what OffersBirthPlan needs to know about an
// Engagement: what the Practice sold, and what became of the pregnancy.
// BirthOutcome is nil for an Engagement whose outcome has not been
// recorded -- the pregnancy is still expected -- and otherwise holds one
// of outcome.go's three members.
type BirthPlanInputs struct {
	Kind         Kind
	BirthOutcome *string
}

// HasLivingOrExpectedBaby answers ADR-0015's standing question -- does
// this Engagement have a living or expected baby? -- from the one input
// it has today, the recorded birth outcome (#294). No outcome recorded
// means the pregnancy is still expected, so the answer is yes; a live
// birth is yes; a loss is no; and 'unknown' is no, because presuming
// nothing is the safe direction for a Client whose Engagement ended
// without the Practice ever learning what happened.
//
// A question rather than a column check, because ADR-0015 states that a
// later event -- a baby born alive who then dies -- becomes a second
// input to this same question without the rule being rewritten, and
// because every surface that presumes a living or expected baby asks
// this, not only the Birth Plan (#296's greeting is the next caller).
func HasLivingOrExpectedBaby(birthOutcome *string) bool {
	if birthOutcome == nil {
		return true
	}
	return *birthOutcome == OutcomeLiveBirth
}

// OffersBirthPlan answers ADR-0015's suppression question -- does this
// Engagement call for a Birth Plan? -- once, so every surface that
// offers, links to or announces a Birth Plan agrees with every other.
// A Birth Plan is offered when the Practice sold a birth Engagement
// (#311) and that Engagement has a living or expected baby (#294).
//
// This is the Client-facing offer, which is why 'live_birth' still
// answers true: ADR-0015's table keeps an existing plan readable two
// days after the birth, and its "offer to create ... when the birth
// outcome is null" prose is about the Practice's authoring affordance,
// which this function does not gate.
//
// ADR-0015 calls the suppressed state "retired, not deleted", and
// retirement here is entirely derived: no flag is stored on the Plan
// Instance and nothing about it is changed, so correcting an outcome
// from 'loss' back to 'live_birth' gives the Birth Plan back with no
// separate act and nothing to reverse.
func OffersBirthPlan(in BirthPlanInputs) bool {
	return in.Kind == KindBirth && HasLivingOrExpectedBaby(in.BirthOutcome)
}
