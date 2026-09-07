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
// Engagement. Kind is the only field today; #294 adds the birth outcome
// as a second field here once #293 lands birth_outcome, extending the
// struct rather than OffersBirthPlan's signature or its callers.
type BirthPlanInputs struct {
	Kind Kind
}

// OffersBirthPlan answers ADR-0015's suppression question -- does this
// Engagement call for a Birth Plan? -- once, so every surface that
// offers, links to or announces a Birth Plan agrees with every other.
// Today: kind = birth, per ADR-0015's rule to offer a Birth Plan when
// kind = birth and the birth outcome is null.
func OffersBirthPlan(in BirthPlanInputs) bool {
	return in.Kind == KindBirth
}
