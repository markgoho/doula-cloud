package engagement_test

import (
	"testing"

	"doula-cloud/api/internal/engagement"
)

// TestOffersBirthPlan proves ADR-0015's suppression rule's kind half --
// the only input #311 could build, since birth_outcome does not exist
// until #293/#294.
func TestOffersBirthPlan(t *testing.T) {
	tests := []struct {
		name string
		kind engagement.Kind
		want bool
	}{
		{"birth Engagement is offered one", engagement.KindBirth, true},
		{"postpartum Engagement is not", engagement.KindPostpartum, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := engagement.OffersBirthPlan(engagement.BirthPlanInputs{Kind: tt.kind})
			if got != tt.want {
				t.Fatalf("OffersBirthPlan(%q) = %v, want %v", tt.kind, got, tt.want)
			}
		})
	}
}
