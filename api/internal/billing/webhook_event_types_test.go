package billing_test

import (
	"slices"
	"testing"

	"doula-cloud/api/internal/billing"
)

func TestHandledEventTypes_IsTheCheckoutCompletion(t *testing.T) {
	if got, want := billing.HandledEventTypes(), []string{"checkout.session.completed"}; !slices.Equal(got, want) {
		t.Errorf("HandledEventTypes() = %v, want %v", got, want)
	}
}
