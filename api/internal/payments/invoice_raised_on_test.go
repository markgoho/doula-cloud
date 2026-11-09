package payments_test

import (
	"testing"
	"time"
)

// TestStaffInvoiceLists_RaisedOnIsThePracticesDay is #2146's: an Invoice
// raised at 10pm on Oct 25 in New York (02:00Z on Oct 26) reads as raised
// on Oct 25 on both Staff lists -- the Engagement's and the Practice's --
// the same day a contractor's row has carried since #2094, so every Staff
// reader of one Invoice reads one Raised day.
func TestStaffInvoiceLists_RaisedOnIsThePracticesDay(t *testing.T) {
	e := newEnv(t)
	engagementID, contractID := e.contractWithStatus("draft")
	seedInvoice(t, e.DB, e.Practice.ID, contractID, "in_raised_on", invoiceStatusOpen, 15000,
		time.Date(2026, 10, 26, 2, 0, 0, 0, time.UTC))

	book := e.practiceInvoices()
	if len(book.Items) != 1 || book.Items[0].RaisedOn != dayOct25 {
		t.Fatalf("Practice list items = %+v, want raisedOn %s", book.Items, dayOct25)
	}

	section := e.listInvoices(engagementID)
	if len(section.Items) != 1 || section.Items[0].RaisedOn != dayOct25 {
		t.Fatalf("Engagement list items = %+v, want raisedOn %s", section.Items, dayOct25)
	}
}
