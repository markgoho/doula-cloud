package payments_test

import (
	"testing"
	"time"

	"doula-cloud/api/internal/testdb"
)

// Overdue is derived, never stored (#768): open, and due before the start
// of the Practice's own today (#1965), evaluated by Postgres at read.
// invoice_days_wire_test.go pins that boundary at fixed instants; these
// tests move the comparison
// rather than waiting for one -- an Invoice seeded due five days ago is
// the same fact as an Invoice raised five days ago falling due today, and
// neither needs a webhook (Stripe emits none when a due date passes) nor
// a scheduled sweep.

// TestGetPracticeInvoicesHandler_OverdueTotalsAndNarrowing is #768's
// central proof: the Practice-wide list reports which Invoices are past
// their own due date and how much money that is, and can be narrowed to
// exactly those, with the whole-book totals unchanged by the narrowing.
func TestGetPracticeInvoicesHandler_OverdueTotalsAndNarrowing(t *testing.T) {
	db := testdb.New(t)
	const uid = "practice-invoices-overdue"
	practiceID, _ := testdb.SeedStaffAtNewPractice(t, db, uid, []string{ownerRole}, "employee")
	_, engagementID := testdb.SeedNamedEngagement(t, db, practiceID, "Ada Client", "ada@example.com")
	contractID := seedDraftContract(t, db, engagementID)
	now := overdueInstants[0]
	base := now.Add(-time.Hour)

	lateID := seedInvoiceDue(t, db, practiceID, contractID, "in_overdue_late", invoiceStatusOpen, 15000,
		base, dueOnPracticeDay(t, now, -5))
	// Open, but on the right side of its own due date: the same status,
	// so status alone cannot produce this answer.
	seedInvoiceDue(t, db, practiceID, contractID, "in_overdue_current", invoiceStatusOpen, 22000,
		base.Add(time.Second), dueOnPracticeDay(t, now, 25))
	// Paid, with its due date long gone. Money that arrived is never
	// late, which is why the comparison carries the 'open' test with it.
	seedInvoiceDue(t, db, practiceID, contractID, "in_overdue_paid", "paid", 30000,
		base.Add(2*time.Second), dueOnPracticeDay(t, now, -40))

	srv, session := newPracticeInvoiceServerAt(t, db, uid, now)
	defer srv.Close()

	whole := readPracticeInvoices(t, srv, session, practiceID, "", false)
	if whole.OverdueCents != 15000 || whole.OverdueCount != 1 {
		t.Fatalf("overdue totals = (%d, %d), want (15000, 1)", whole.OverdueCents, whole.OverdueCount)
	}
	// Overdue is a slice of outstanding, not a second book beside it: the
	// late Invoice is counted in both pairs.
	if whole.OutstandingCents != 37000 || whole.OutstandingCount != 2 {
		t.Fatalf("outstanding totals = (%d, %d), want (37000, 2)", whole.OutstandingCents, whole.OutstandingCount)
	}
	if len(whole.Items) != 3 {
		t.Fatalf("unnarrowed items = %d, want 3", len(whole.Items))
	}

	narrowed := readPracticeInvoicesNarrowed(t, srv, session, practiceID, "", "overdue=true")
	if len(narrowed.Items) != 1 || narrowed.Items[0].ID != lateID {
		t.Fatalf("overdue items = %+v, want only %q", narrowed.Items, lateID)
	}
	if narrowed.OverdueCents != 15000 || narrowed.OutstandingCents != 37000 {
		t.Fatalf("narrowed totals = (%d overdue, %d outstanding), want whole-book (15000, 37000)",
			narrowed.OverdueCents, narrowed.OutstandingCents)
	}
	// The row carries its due day and how many Practice days late it is,
	// counted by the same rule as the figure above it (#1965). The exact
	// count is TestDaysOverdue's; here it is enough that a late row says so.
	if narrowed.Items[0].DaysOverdue < 1 {
		t.Fatalf("daysOverdue = %d, want a positive count", narrowed.Items[0].DaysOverdue)
	}
	if narrowed.Items[0].DueOn == "" {
		t.Fatal("dueOn is empty, want the due day")
	}
}

// TestGetPracticeInvoicesHandler_OverdueBeatsUnpaid pins which narrowing
// wins when a caller sets both: the narrower one, so ?overdue=true means
// overdue whatever else rides along with it.
//
// It runs at each of overdueInstants, so it gives the same answer at every
// hour (#2639).
func TestGetPracticeInvoicesHandler_OverdueBeatsUnpaid(t *testing.T) {
	for _, now := range overdueInstants {
		t.Run(now.Format(time.RFC3339), func(t *testing.T) {
			db := testdb.New(t)
			const uid = "practice-invoices-overdue-wins"
			practiceID, _ := testdb.SeedStaffAtNewPractice(t, db, uid, []string{ownerRole}, "employee")
			_, engagementID := testdb.SeedNamedEngagement(t, db, practiceID, "Ada Client", "ada@example.com")
			contractID := seedDraftContract(t, db, engagementID)
			base := now.Add(-time.Hour)

			// Due on the Practice day before its today: the first day it is late.
			lateID := seedInvoiceDue(t, db, practiceID, contractID, "in_both_late", invoiceStatusOpen, 15000,
				base, dueOnPracticeDay(t, now, -1))
			seedInvoiceDue(t, db, practiceID, contractID, "in_both_current", invoiceStatusOpen, 22000,
				base.Add(time.Second), dueOnPracticeDay(t, now, 10))

			srv, session := newPracticeInvoiceServerAt(t, db, uid, now)
			defer srv.Close()

			out := readPracticeInvoicesNarrowed(t, srv, session, practiceID, "", "overdue=true"+"&"+"unpaid=true")
			if len(out.Items) != 1 || out.Items[0].ID != lateID {
				t.Fatalf("items = %+v, want only the overdue %q", out.Items, lateID)
			}
		})
	}
}

// overdueInstants are the request instants the overdue tests run at, read
// through clock.Now(ctx) (newPracticeInvoiceServerAt), never the host's
// time.Now(). The first is #2639's failing hour: 04:22 UTC on Nov 2 2026 is
// 11:22pm on Nov 1 in New York, the 25-hour day the clocks went back, so an
// instant 24 hours before it is still Nov 1 there and an invoice seeded due
// "now minus a day" was due on the Practice's own today, not before it. The
// others are an ordinary hour on that day, the same hour on an ordinary day
// and the 23-hour day the clocks went forward.
var overdueInstants = []time.Time{
	time.Date(2026, 11, 2, 4, 22, 0, 0, time.UTC),
	time.Date(2026, 11, 2, 15, 22, 0, 0, time.UTC),
	time.Date(2026, 10, 20, 4, 22, 0, 0, time.UTC),
	time.Date(2027, 3, 15, 4, 22, 0, 0, time.UTC),
}

// dueOnPracticeDay is noon on the Practice's own day, days after (or
// before, when negative) the Practice day now falls on -- the day the
// handler counts lateness in (#1965) -- so a due date is built from the
// same reckoning the read uses and not from 24-hour offsets that a clock
// change makes disagree with it.
func dueOnPracticeDay(t *testing.T, now time.Time, days int) time.Time {
	t.Helper()
	zone, err := time.LoadLocation(testdb.SeededPracticeZoneName)
	if err != nil {
		t.Fatalf("load %s: %v", testdb.SeededPracticeZoneName, err)
	}
	year, month, day := now.In(zone).Date()
	return time.Date(year, month, day+days, 12, 0, 0, 0, zone)
}

// TestGetPracticeInvoicesHandler_EmptyBookHasNoOverdue covers the
// COALESCE side of the overdue aggregate: a Practice that has billed
// nothing reports zero, not a null that fails to scan.
func TestGetPracticeInvoicesHandler_EmptyBookHasNoOverdue(t *testing.T) {
	db := testdb.New(t)
	const uid = "practice-invoices-overdue-empty"
	practiceID, _ := testdb.SeedStaffAtNewPractice(t, db, uid, []string{ownerRole}, "employee")

	srv, session := newPracticeInvoiceServer(t, db, uid)
	defer srv.Close()

	out := readPracticeInvoicesNarrowed(t, srv, session, practiceID, "", "overdue=true")
	if out.OverdueCents != 0 || out.OverdueCount != 0 || len(out.Items) != 0 {
		t.Fatalf("empty book = (%d, %d, %d items), want all zero", out.OverdueCents, out.OverdueCount, len(out.Items))
	}
}
