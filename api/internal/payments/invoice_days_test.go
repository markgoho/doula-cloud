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
