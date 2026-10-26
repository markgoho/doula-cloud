package payments_test

import (
	"encoding/json"
	"testing"
	"time"

	"doula-cloud/api/internal/payments"
	"doula-cloud/api/internal/testdb"
)

// TestStaffInvoiceLists_RaisedOnIsThePracticesDay is #2146's: an Invoice
// raised at 10pm on Oct 25 in New York (02:00Z on Oct 26) reads as raised
// on Oct 25 on both Staff lists -- the Engagement's and the Practice's --
// the same day a contractor's row has carried since #2094, so every Staff
// reader of one Invoice reads one Raised day.
func TestStaffInvoiceLists_RaisedOnIsThePracticesDay(t *testing.T) {
	db := testdb.New(t)
	const uid = "invoice-raised-on-practice-day"
	practiceID, _ := testdb.SeedStaffAtNewPractice(t, db, uid, []string{ownerRole}, "employee")
	_, engagementID := testdb.SeedNamedEngagement(t, db, practiceID, "Ada Client", "ada@example.com")
	contractID := seedDraftContract(t, db, engagementID)
	seedInvoice(t, db, practiceID, contractID, "in_raised_on", invoiceStatusOpen, 15000,
		time.Date(2026, 10, 26, 2, 0, 0, 0, time.UTC))

	srv, session := newPracticeInvoiceServer(t, db, uid)
	defer srv.Close()

	book := readPracticeInvoices(t, srv, session, practiceID, "", false)
	if len(book.Items) != 1 || book.Items[0].RaisedOn != dayOct25 {
		t.Fatalf("Practice list items = %+v, want raisedOn %s", book.Items, dayOct25)
	}

	listResp := getInvoices(t, srv, session, practiceID, engagementID, "")
	defer listResp.Body.Close()
	var section payments.ListInvoicesResponse
	if err := json.NewDecoder(listResp.Body).Decode(&section); err != nil {
		t.Fatalf("decode engagement invoices: %v", err)
	}
	if len(section.Items) != 1 || section.Items[0].RaisedOn != dayOct25 {
		t.Fatalf("Engagement list items = %+v, want raisedOn %s", section.Items, dayOct25)
	}
}
