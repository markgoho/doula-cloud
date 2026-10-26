package payments_test

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"
	"time"

	"doula-cloud/api/internal/authntest"
	"doula-cloud/api/internal/payments"
	"doula-cloud/api/internal/testdb"
)

// TestAnOverdueInvoiceReadsOverdueInEveryView proves #2122: one overdue
// Invoice is late by the same count on every Staff read of it -- the
// Engagement's Invoice list as an Owner, an Admin, an employee Doula and
// a contractor Doula, and the Practice-wide list -- because each one is
// counted by daysOverdue at the one request instant. The Client portal
// reads the same stored status, open, and no lateness at all: ADR-0038
// keeps Overdue off the portal, so no daysOverdue key reaches her.
func TestAnOverdueInvoiceReadsOverdueInEveryView(t *testing.T) {
	db := testdb.New(t)
	const clientUID = "one-status-client"
	me := seedClientMoney(t, db, clientUID, "Imani Client", "imani@example.com")

	// Due 2026-11-24T00:00Z is Nov 23 in New York; at 5pm UTC on Dec 10
	// it is 17 Practice days late.
	now := time.Date(2026, 12, 10, 17, 0, 0, 0, time.UTC)
	invoiceID := seedInvoiceDue(t, db, me.practiceID, me.contractID, "in_one_status", invoiceStatusOpen, 15000,
		time.Date(2026, 10, 25, 15, 0, 0, 0, time.UTC), time.Date(2026, 11, 24, 0, 0, 0, 0, time.UTC))

	const ownerUID = "one-status-owner"
	staff := []struct {
		name, uid, role string
	}{
		{"Owner", ownerUID, ownerRole},
		{"Admin", "one-status-admin", adminRole},
		{"employee Doula", "one-status-doula", doulaRole},
	}
	for _, s := range staff {
		testdb.SeedStaffAtPractice(t, db, me.practiceID, s.uid, []string{s.role}, "employee")
	}
	srv, ownerSession := newPracticeInvoiceServerAt(t, db, ownerUID, now)
	defer srv.Close()

	counts := map[string]int{}

	book := readPracticeInvoices(t, srv, ownerSession, me.practiceID, "", false)
	if len(book.Items) != 1 || book.Items[0].ID != invoiceID || book.Items[0].Status != invoiceStatusOpen {
		t.Fatalf("Practice-wide items = %+v, want the one open Invoice", book.Items)
	}
	counts["Practice-wide list"] = book.Items[0].DaysOverdue

	for _, s := range staff {
		session := ownerSession
		if s.uid != ownerUID {
			session = authntest.SeedSessionAt(t, db.App, s.uid, now)
		}
		resp := getInvoices(t, srv, session, me.practiceID, me.engagementID, "")
		var section payments.ListInvoicesResponse
		err := json.NewDecoder(resp.Body).Decode(&section)
		_ = resp.Body.Close()
		if resp.StatusCode != http.StatusOK || err != nil {
			t.Fatalf("%s: status %d, decode %v", s.name, resp.StatusCode, err)
		}
		if len(section.Items) != 1 || section.Items[0].Status != invoiceStatusOpen {
			t.Fatalf("%s: items = %+v, want the one open Invoice", s.name, section.Items)
		}
		counts["Engagement list as "+s.name] = section.Items[0].DaysOverdue
	}

	const contractorUID = "one-status-contractor"
	contractorID := testdb.SeedStaffAtPractice(t, db, me.practiceID, contractorUID, []string{doulaRole}, "contractor")
	testdb.SeedGrantedAttachment(t, db, me.engagementID, contractorID)
	resp := getInvoices(t, srv, authntest.SeedSessionAt(t, db.App, contractorUID, now), me.practiceID, me.engagementID, "")
	var rows payments.ContractorInvoicesResponse
	err := json.NewDecoder(resp.Body).Decode(&rows)
	_ = resp.Body.Close()
	if resp.StatusCode != http.StatusOK || err != nil {
		t.Fatalf("contractor: status %d, decode %v", resp.StatusCode, err)
	}
	if len(rows.Items) != 1 || rows.Items[0].Status != invoiceStatusOpen {
		t.Fatalf("contractor: items = %+v, want the one open Invoice", rows.Items)
	}
	counts["Engagement list as contractor Doula"] = rows.Items[0].DaysOverdue

	for view, got := range counts {
		if got != 17 {
			t.Errorf("%s: daysOverdue = %d, want 17", view, got)
		}
	}

	portal := newClientMoneyServer(t, db)
	status, body := getPortal(t, portal, portalSession(t, db, clientUID), clientInvoicesPath(me.engagementID, ""))
	if status != http.StatusOK {
		t.Fatalf("portal status = %d, want 200; body %s", status, body)
	}
	var portalList payments.ClientInvoicesResponse
	if err := json.Unmarshal([]byte(body), &portalList); err != nil {
		t.Fatalf("decode portal: %v", err)
	}
	if len(portalList.Items) != 1 || portalList.Items[0].ID != invoiceID || portalList.Items[0].Status != invoiceStatusOpen {
		t.Fatalf("portal items = %+v, want the one open Invoice", portalList.Items)
	}
	if strings.Contains(body, "daysOverdue") || strings.Contains(strings.ToLower(body), "overdue") {
		t.Fatalf("the portal carries lateness, which ADR-0038 keeps off it: %s", body)
	}
}
