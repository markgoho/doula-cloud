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
	// The Practice's own zone (#953). Eastern is four hours behind UTC on
	// this date, so an evening Visit here is already tomorrow in UTC --
	// which is the bug this argument exists to end.
	eastern, err := time.LoadLocation("America/New_York")
	if err != nil {
		t.Fatalf("load America/New_York: %v", err)
	}
	// A zone ahead of UTC, so the correction is proved in both
	// directions rather than only the one the bug was reported in: an
	// early-morning Visit in Tokyo is still yesterday in UTC.
	tokyo, err := time.LoadLocation("Asia/Tokyo")
	if err != nil {
		t.Fatalf("load Asia/Tokyo: %v", err)
	}
	tests := []struct {
		name             string
		at               time.Time
		pregnancyEndedOn *string
		zone             *time.Location
		want             string
	}{
		{
			name:             "no pivot recorded yet",
			at:               time.Date(2099, 1, 1, 0, 0, 0, 0, time.UTC),
			pregnancyEndedOn: nil,
			zone:             time.UTC,
			want:             visit.TypePrenatal,
		},
		{
			name:             "empty pivot string treated the same as nil",
			at:               time.Date(2026, 3, 15, 12, 0, 0, 0, time.UTC),
			pregnancyEndedOn: new(string), // zero-value "": the empty pivot case
			zone:             time.UTC,
			want:             visit.TypePrenatal,
		},
		{
			name:             "before the pivot",
			at:               time.Date(2026, 3, 14, 23, 59, 0, 0, time.UTC),
			pregnancyEndedOn: &pivot,
			zone:             time.UTC,
			want:             visit.TypePrenatal,
		},
		{
			name:             "on the pivot",
			at:               time.Date(2026, 3, 15, 8, 0, 0, 0, time.UTC),
			pregnancyEndedOn: &pivot,
			zone:             time.UTC,
			want:             visit.TypeBirth,
		},
		{
			name:             "late in the day on the pivot is still birth",
			at:               time.Date(2026, 3, 15, 23, 59, 59, 0, time.UTC),
			pregnancyEndedOn: &pivot,
			zone:             time.UTC,
			want:             visit.TypeBirth,
		},
		{
			name:             "after the pivot",
			at:               time.Date(2026, 3, 16, 0, 0, 1, 0, time.UTC),
			pregnancyEndedOn: &pivot,
			zone:             time.UTC,
			want:             visit.TypePostpartum,
		},
		{
			name:             "weeks after the pivot",
			at:               time.Date(2026, 4, 5, 9, 0, 0, 0, time.UTC),
			pregnancyEndedOn: &pivot,
			zone:             time.UTC,
			want:             visit.TypePostpartum,
		},
		{
			// #953's own AC, and the reason this argument exists: 9pm
			// Eastern on the pregnancy-end date is 01:00 the next day in
			// UTC, so this typed postpartum before the Practice's zone
			// decided the day.
			name:             "9pm Eastern on the pivot is birth, not postpartum",
			at:               time.Date(2026, 3, 16, 1, 0, 0, 0, time.UTC),
			pregnancyEndedOn: &pivot,
			zone:             eastern,
			want:             visit.TypeBirth,
		},
		{
			name:             "11:30pm Eastern on the pivot is still birth",
			at:               time.Date(2026, 3, 16, 3, 30, 0, 0, time.UTC),
			pregnancyEndedOn: &pivot,
			zone:             eastern,
			want:             visit.TypeBirth,
		},
		{
			// The far side of the same midnight: half an hour later in
			// Eastern is a new day, and the type moves with it.
			name:             "12:30am Eastern the day after the pivot is postpartum",
			at:               time.Date(2026, 3, 16, 4, 30, 0, 0, time.UTC),
			pregnancyEndedOn: &pivot,
			zone:             eastern,
			want:             visit.TypePostpartum,
		},
		{
			// 6am Tokyo on the pivot is still the previous day in UTC,
			// so UTC would have called this prenatal.
			name:             "early morning in Tokyo on the pivot is birth, not prenatal",
			at:               time.Date(2026, 3, 14, 21, 0, 0, 0, time.UTC),
			pregnancyEndedOn: &pivot,
			zone:             tokyo,
			want:             visit.TypeBirth,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := visit.DeriveType(tt.at, tt.pregnancyEndedOn, tt.zone); got != tt.want {
				t.Errorf("DeriveType(%v, %v, %v) = %q, want %q", tt.at, deref(tt.pregnancyEndedOn), tt.zone, got, tt.want)
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
