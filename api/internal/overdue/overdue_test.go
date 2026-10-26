package overdue_test

import (
	"testing"
	"time"

	"doula-cloud/api/internal/overdue"
)

// These pin #1965's lateness rule with no database or HTTP handler in the
// loop: an Invoice is overdue from the start of the day after its due day
// in the Practice's own zone. The payments and client handler tests prove
// the same rule through the wire.

func mustLoad(t *testing.T, name string) *time.Location {
	t.Helper()
	loc, err := time.LoadLocation(name)
	if err != nil {
		t.Fatalf("load %s: %v", name, err)
	}
	return loc
}

func TestFrom(t *testing.T) {
	eastern := mustLoad(t, "America/New_York")

	// 11:59pm Eastern on Nov 23 (EST, UTC-5): the Practice's day is still
	// the 23rd, so the boundary is midnight Eastern starting the 23rd.
	got := overdue.From(time.Date(2026, 11, 24, 4, 59, 0, 0, time.UTC), eastern)
	want := time.Date(2026, 11, 23, 5, 0, 0, 0, time.UTC)
	if !got.Equal(want) {
		t.Fatalf("From at 11:59pm Eastern = %v, want %v", got, want)
	}

	// One minute later it is Nov 24 in Eastern, and the boundary moves to
	// the start of that day.
	got = overdue.From(time.Date(2026, 11, 24, 5, 0, 0, 0, time.UTC), eastern)
	want = time.Date(2026, 11, 24, 5, 0, 0, 0, time.UTC)
	if !got.Equal(want) {
		t.Fatalf("From at midnight Eastern = %v, want %v", got, want)
	}
}

// open is the stored status of a billed, unpaid Invoice.
const open = "open"

func TestDays(t *testing.T) {
	eastern := mustLoad(t, "America/New_York")
	// Due Nov 23 in Eastern (#1958's walk).
	dueAt := time.Date(2026, 11, 24, 0, 0, 0, 0, time.UTC)

	tests := []struct {
		name   string
		status string
		now    time.Time
		want   int
	}{
		// Under the instant rule this Invoice was overdue from 7pm Eastern
		// on its own due day. A person reads that day as a whole day to
		// pay in, so it is not late until the day after.
		{"the due day itself is not late, even after the due instant", open, time.Date(2026, 11, 24, 4, 59, 0, 0, time.UTC), 0},
		{"the start of the next Practice day is one day late", open, time.Date(2026, 11, 24, 5, 0, 0, 0, time.UTC), 1},
		{"late on the next Practice day is still one day", open, time.Date(2026, 11, 25, 4, 59, 0, 0, time.UTC), 1},
		{"three Practice days on", open, time.Date(2026, 11, 26, 15, 0, 0, 0, time.UTC), 3},
		{"before the due day", open, time.Date(2026, 11, 20, 15, 0, 0, 0, time.UTC), 0},
		{"a paid Invoice is never late", "paid", time.Date(2026, 12, 30, 15, 0, 0, 0, time.UTC), 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := overdue.Days(tt.status, dueAt, tt.now, eastern); got != tt.want {
				t.Errorf("Days(%q, %v, %v) = %d, want %d", tt.status, dueAt, tt.now, got, tt.want)
			}
		})
	}
}
