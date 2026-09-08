package engagement_test

import (
	"testing"

	"doula-cloud/api/internal/engagement"
)

// TestHasLivingOrExpectedBaby proves ADR-0015's standing question over
// its only input today. 'unknown' answering no is the deliberate part:
// presuming nothing is the safe direction for a Client whose Engagement
// ended without the Practice ever learning what happened.
func TestHasLivingOrExpectedBaby(t *testing.T) {
	tests := []struct {
		name         string
		birthOutcome *string
		want         bool
	}{
		{"no outcome recorded yet -- the pregnancy is still expected", nil, true},
		{"a live birth", new(engagement.OutcomeLiveBirth), true},
		{"a loss", new(engagement.OutcomeLoss), false},
		{"an unknown outcome presumes nothing", new(engagement.OutcomeUnknown), false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := engagement.HasLivingOrExpectedBaby(tt.birthOutcome); got != tt.want {
				t.Fatalf("HasLivingOrExpectedBaby(%v) = %v, want %v", tt.birthOutcome, got, tt.want)
			}
		})
	}
}

// TestOffersBirthPlan proves ADR-0015's suppression rule over both of
// its inputs: the kind half (#311) and the outcome half (#294). A
// postpartum Engagement is refused whatever became of the pregnancy, so
// neither half can be dropped without a row here failing.
func TestOffersBirthPlan(t *testing.T) {
	tests := []struct {
		name         string
		kind         engagement.Kind
		birthOutcome *string
		want         bool
	}{
		{"birth, no outcome recorded yet", engagement.KindBirth, nil, true},
		{"birth, live birth", engagement.KindBirth, new(engagement.OutcomeLiveBirth), true},
		{"birth, loss", engagement.KindBirth, new(engagement.OutcomeLoss), false},
		{"birth, unknown", engagement.KindBirth, new(engagement.OutcomeUnknown), false},
		{"postpartum, no outcome recorded yet", engagement.KindPostpartum, nil, false},
		{"postpartum, live birth", engagement.KindPostpartum, new(engagement.OutcomeLiveBirth), false},
		{"postpartum, loss", engagement.KindPostpartum, new(engagement.OutcomeLoss), false},
		{"postpartum, unknown", engagement.KindPostpartum, new(engagement.OutcomeUnknown), false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := engagement.OffersBirthPlan(engagement.BirthPlanInputs{
				Kind: tt.kind, BirthOutcome: tt.birthOutcome,
			})
			if got != tt.want {
				t.Fatalf("OffersBirthPlan(%q, %v) = %v, want %v", tt.kind, tt.birthOutcome, got, tt.want)
			}
		})
	}
}
