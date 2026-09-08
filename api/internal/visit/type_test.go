package visit_test

import (
	"testing"
	"time"

	"doula-cloud/api/internal/visit"
)

// TestDeriveType covers ADR-0015's rule directly, with no database in the
// loop: prenatal before the pivot, birth on it, postpartum after, and
// prenatal for every Visit on an Engagement with no pivot at all.
// TestListHandler_ReturnsVisitType (handlers_test.go) is the "every
// surface agrees" half of #281's own AC -- it drives the same function
// through the real HTTP handler and compares.
func TestDeriveType(t *testing.T) {
	pivot := "2026-03-15"
	tests := []struct {
		name             string
		at               time.Time
		pregnancyEndedOn *string
		want             string
	}{
		{
			name:             "no pivot recorded yet",
			at:               time.Date(2099, 1, 1, 0, 0, 0, 0, time.UTC),
			pregnancyEndedOn: nil,
			want:             visit.TypePrenatal,
		},
		{
			name:             "empty pivot string treated the same as nil",
			at:               time.Date(2026, 3, 15, 12, 0, 0, 0, time.UTC),
			pregnancyEndedOn: new(string), // zero-value "": the empty pivot case
			want:             visit.TypePrenatal,
		},
		{
			name:             "before the pivot",
			at:               time.Date(2026, 3, 14, 23, 59, 0, 0, time.UTC),
			pregnancyEndedOn: &pivot,
			want:             visit.TypePrenatal,
		},
		{
			name:             "on the pivot",
			at:               time.Date(2026, 3, 15, 8, 0, 0, 0, time.UTC),
			pregnancyEndedOn: &pivot,
			want:             visit.TypeBirth,
		},
		{
			name:             "late in the day on the pivot is still birth",
			at:               time.Date(2026, 3, 15, 23, 59, 59, 0, time.UTC),
			pregnancyEndedOn: &pivot,
			want:             visit.TypeBirth,
		},
		{
			name:             "after the pivot",
			at:               time.Date(2026, 3, 16, 0, 0, 1, 0, time.UTC),
			pregnancyEndedOn: &pivot,
			want:             visit.TypePostpartum,
		},
		{
			name:             "weeks after the pivot",
			at:               time.Date(2026, 4, 5, 9, 0, 0, 0, time.UTC),
			pregnancyEndedOn: &pivot,
			want:             visit.TypePostpartum,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := visit.DeriveType(tt.at, tt.pregnancyEndedOn); got != tt.want {
				t.Errorf("DeriveType(%v, %v) = %q, want %q", tt.at, deref(tt.pregnancyEndedOn), got, tt.want)
			}
		})
	}
}

func deref(s *string) string {
	if s == nil {
		return "<nil>"
	}
	return *s
}
