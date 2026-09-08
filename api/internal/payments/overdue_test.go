package payments_test

import (
	"testing"
	"time"

	"doula-cloud/api/internal/testdb"
)

// Overdue is derived, never stored (#768): `status = 'open' AND due_at <
// now()`, evaluated by Postgres at read. These tests move the comparison
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
	base := time.Now().Add(-time.Hour)

	lateID := seedInvoiceDue(t, db, practiceID, contractID, "in_overdue_late", invoiceStatusOpen, 15000,
		base, time.Now().AddDate(0, 0, -5))
	// Open, but on the right side of its own due date: the same status,
	// so status alone cannot produce this answer.
	seedInvoiceDue(t, db, practiceID, contractID, "in_overdue_current", invoiceStatusOpen, 22000,
		base.Add(time.Second), time.Now().AddDate(0, 0, 25))
	// Paid, with its due date long gone. Money that arrived is never
	// late, which is why the comparison carries the 'open' test with it.
	seedInvoiceDue(t, db, practiceID, contractID, "in_overdue_paid", "paid", 30000,
		base.Add(2*time.Second), time.Now().AddDate(0, 0, -40))

	srv, session := newPracticeInvoiceServer(t, db, uid)
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
	// The row carries the date, never a day count -- how late an Invoice
	// is has to stay true in a tab left open overnight.
	if narrowed.Items[0].DueAt.IsZero() || !narrowed.Items[0].DueAt.Before(time.Now()) {
		t.Fatalf("dueAt = %v, want a past instant", narrowed.Items[0].DueAt)
	}
}

// TestGetPracticeInvoicesHandler_OverdueBeatsUnpaid pins which narrowing
// wins when a caller sets both: the narrower one, so ?overdue=true means
// overdue whatever else rides along with it.
func TestGetPracticeInvoicesHandler_OverdueBeatsUnpaid(t *testing.T) {
	db := testdb.New(t)
	const uid = "practice-invoices-overdue-wins"
	practiceID, _ := testdb.SeedStaffAtNewPractice(t, db, uid, []string{ownerRole}, "employee")
	_, engagementID := testdb.SeedNamedEngagement(t, db, practiceID, "Ada Client", "ada@example.com")
	contractID := seedDraftContract(t, db, engagementID)
	base := time.Now().Add(-time.Hour)

	lateID := seedInvoiceDue(t, db, practiceID, contractID, "in_both_late", invoiceStatusOpen, 15000,
		base, time.Now().AddDate(0, 0, -1))
	seedInvoiceDue(t, db, practiceID, contractID, "in_both_current", invoiceStatusOpen, 22000,
		base.Add(time.Second), time.Now().AddDate(0, 0, 10))

	srv, session := newPracticeInvoiceServer(t, db, uid)
	defer srv.Close()

	out := readPracticeInvoicesNarrowed(t, srv, session, practiceID, "", "overdue=true"+"&"+"unpaid=true")
	if len(out.Items) != 1 || out.Items[0].ID != lateID {
		t.Fatalf("items = %+v, want only the overdue %q", out.Items, lateID)
	}
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
