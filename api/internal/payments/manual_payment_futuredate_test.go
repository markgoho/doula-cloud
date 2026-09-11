package payments

import (
	"testing"
	"time"
)

// TestPaidOnAfterPracticeToday covers ADR-0036's rule directly, with no
// database or HTTP handler in the loop: PostManualPaymentHandler's own
// future-date guard delegates to this function, so pinning it here proves
// the guard's comparison at the exact instant it used to disagree with
// the Practice's own day (#1167). visit.TestDeriveType (internal/visit/
// type_test.go) proves the same shape of correction for #953's first
// reader; this is the second.
func TestPaidOnAfterPracticeToday(t *testing.T) {
	eastern, err := time.LoadLocation("America/New_York")
	if err != nil {
		t.Fatalf("load America/New_York: %v", err)
	}

	tests := []struct {
		name   string
		paidOn time.Time
		at     time.Time
		loc    *time.Location
		want   bool
	}{
		{
			name:   "same UTC day is not future",
			paidOn: time.Date(2026, 3, 15, 0, 0, 0, 0, time.UTC),
			at:     time.Date(2026, 3, 15, 12, 0, 0, 0, time.UTC),
			loc:    time.UTC,
			want:   false,
		},
		{
			name:   "next UTC day is future",
			paidOn: time.Date(2026, 3, 16, 0, 0, 0, 0, time.UTC),
			at:     time.Date(2026, 3, 15, 12, 0, 0, 0, time.UTC),
			loc:    time.UTC,
			want:   true,
		},
		{
			// #1167's own AC: 9pm Eastern on day N is already 1am the next
			// day in UTC. A comparison against UTC's day would call day
			// N+1 "today" for the last hours of every Eastern day -- the
			// bug this guard exists to end.
			name:   "9pm Eastern on day N: day N is not yet future",
			paidOn: time.Date(2026, 3, 15, 0, 0, 0, 0, time.UTC),
			at:     time.Date(2026, 3, 16, 1, 0, 0, 0, time.UTC), // 9pm EDT, day N
			loc:    eastern,
			want:   false,
		},
		{
			name:   "9pm Eastern on day N: day N+1 is future",
			paidOn: time.Date(2026, 3, 16, 0, 0, 0, 0, time.UTC),
			at:     time.Date(2026, 3, 16, 1, 0, 0, 0, time.UTC), // 9pm EDT, day N
			loc:    eastern,
			want:   true,
		},
		{
			// The far side of the same midnight: half an hour later in
			// Eastern is a new day, so day N+1 is no longer future.
			name:   "12:30am Eastern the day after day N: day N+1 is not future",
			paidOn: time.Date(2026, 3, 16, 0, 0, 0, 0, time.UTC),
			at:     time.Date(2026, 3, 16, 4, 30, 0, 0, time.UTC), // 12:30am EDT, day N+1
			loc:    eastern,
			want:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := paidOnAfterPracticeToday(tt.paidOn, tt.at, tt.loc); got != tt.want {
				t.Errorf("paidOnAfterPracticeToday(%v, %v, %v) = %v, want %v", tt.paidOn, tt.at, tt.loc, got, tt.want)
			}
		})
	}
}
