package payments

import (
	"database/sql"
	"testing"
	"time"
)

// These pin #1965's two calendar days with no database or HTTP handler in
// the loop: the day a Payment came in and the day an Invoice falls due
// are days in the Practice's own zone (ADR-0036), and an Invoice is
// overdue from the start of the day after its due day there. The handler
// tests in practice_invoices_days_test.go prove the same rules through
// the wire.

// dayOct25 is #1965's own paid day: 8pm on Oct 25 in New York.
const dayOct25 = "2026-10-25"

func mustLoad(t *testing.T, name string) *time.Location {
	t.Helper()
	loc, err := time.LoadLocation(name)
	if err != nil {
		t.Fatalf("load %s: %v", name, err)
	}
	return loc
}

func TestPracticeDay(t *testing.T) {
	eastern := mustLoad(t, "America/New_York")
	tokyo := mustLoad(t, "Asia/Tokyo")

	tests := []struct {
		name string
		at   time.Time
		loc  *time.Location
		want string
	}{
		// #1965's own case: 8pm on Oct 25 in New York is already Oct 26
		// in UTC. The Practice's day is the 25th.
		{"8pm Eastern is still that day", time.Date(2026, 10, 26, 0, 0, 0, 0, time.UTC), eastern, dayOct25},
		// #1958's walk: an Invoice due at UTC midnight on Nov 24 is due on
		// Nov 23 in New York.
		{"UTC midnight is the evening before in Eastern", time.Date(2026, 11, 24, 0, 0, 0, 0, time.UTC), eastern, "2026-11-23"},
		{"a zone ahead of UTC reads the next day", time.Date(2026, 10, 25, 20, 0, 0, 0, time.UTC), tokyo, "2026-10-26"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := practiceDay(tt.at, tt.loc); got != tt.want {
				t.Errorf("practiceDay(%v, %v) = %q, want %q", tt.at, tt.loc, got, tt.want)
			}
		})
	}
}

func TestInvoicePaidOn(t *testing.T) {
	eastern := mustLoad(t, "America/New_York")

	if got := invoicePaidOn(sql.NullTime{}, false, eastern); got != nil {
		t.Fatalf("unpaid invoice paidOn = %q, want nil", *got)
	}

	// A card Payment Stripe collected at 8pm Eastern on Oct 25: the
	// webhook stores the real instant, 2026-10-26T00:00Z, and the day is
	// read in the Practice's zone.
	stripePaid := sql.NullTime{Time: time.Date(2026, 10, 26, 0, 0, 0, 0, time.UTC), Valid: true}
	if got := invoicePaidOn(stripePaid, false, eastern); got == nil || *got != dayOct25 {
		t.Fatalf("stripe paidOn = %v, want 2026-10-25", got)
	}

	// A by-hand Payment recorded for Oct 25: manual_payment.go stores the
	// typed day as UTC midnight, which is Oct 24 in Eastern. The day is
	// the one the recorder typed, in any zone.
	byHand := sql.NullTime{Time: time.Date(2026, 10, 25, 0, 0, 0, 0, time.UTC), Valid: true}
	if got := invoicePaidOn(byHand, true, eastern); got == nil || *got != dayOct25 {
		t.Fatalf("by-hand paidOn = %v, want 2026-10-25", got)
	}
}

func TestOverdueFrom(t *testing.T) {
	eastern := mustLoad(t, "America/New_York")

	// 11:59pm Eastern on Nov 23 (EST, UTC-5): the Practice's day is still
	// the 23rd, so the boundary is midnight Eastern starting the 23rd.
	got := overdueFrom(time.Date(2026, 11, 24, 4, 59, 0, 0, time.UTC), eastern)
	want := time.Date(2026, 11, 23, 5, 0, 0, 0, time.UTC)
	if !got.Equal(want) {
		t.Fatalf("overdueFrom at 11:59pm Eastern = %v, want %v", got, want)
	}

	// One minute later it is Nov 24 in Eastern, and the boundary moves to
	// the start of that day.
	got = overdueFrom(time.Date(2026, 11, 24, 5, 0, 0, 0, time.UTC), eastern)
	want = time.Date(2026, 11, 24, 5, 0, 0, 0, time.UTC)
	if !got.Equal(want) {
		t.Fatalf("overdueFrom at midnight Eastern = %v, want %v", got, want)
	}
}

func TestDaysOverdue(t *testing.T) {
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
		{"the due day itself is not late, even after the due instant", invoiceStatusOpen, time.Date(2026, 11, 24, 4, 59, 0, 0, time.UTC), 0},
		{"the start of the next Practice day is one day late", invoiceStatusOpen, time.Date(2026, 11, 24, 5, 0, 0, 0, time.UTC), 1},
		{"late on the next Practice day is still one day", invoiceStatusOpen, time.Date(2026, 11, 25, 4, 59, 0, 0, time.UTC), 1},
		{"three Practice days on", invoiceStatusOpen, time.Date(2026, 11, 26, 15, 0, 0, 0, time.UTC), 3},
		{"before the due day", invoiceStatusOpen, time.Date(2026, 11, 20, 15, 0, 0, 0, time.UTC), 0},
		{"a paid Invoice is never late", invoiceStatusPaid, time.Date(2026, 12, 30, 15, 0, 0, 0, time.UTC), 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := daysOverdue(tt.status, dueAt, tt.now, eastern); got != tt.want {
				t.Errorf("daysOverdue(%q, %v, %v) = %d, want %d", tt.status, dueAt, tt.now, got, tt.want)
			}
		})
	}
}
