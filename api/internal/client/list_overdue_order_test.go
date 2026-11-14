package client_test

import (
	"cmp"
	"fmt"
	"slices"
	"testing"
	"time"

	"doula-cloud/api/internal/testdb"
)

const (
	orderOverdue   = "overdue"
	finishedName   = "Finished"
	contractVoided = "voided"
	onItsWayName   = "On Its Way"
	notATime       = "soon"
	paidUpName     = "Paid Up"
)

// behindUTCNow is 7pm on Dec 9 in Los Angeles, which is already 3am on Dec 10
// in UTC: the Practice's today starts at 08:00Z on Dec 9, not at 00:00Z on
// Dec 10, so an Invoice due at noon UTC on Dec 9 is due today there and is
// not overdue, though a UTC reading would call it a day late.
var behindUTCNow = time.Date(2026, 12, 10, 3, 0, 0, 0, time.UTC)

func boolInt(b bool) int {
	if b {
		return 1
	}
	return 0
}

func namesOf(all []lateClient, ids []string) []string {
	byID := map[string]string{}
	for _, c := range all {
		byID[c.id] = c.name
	}
	names := make([]string, len(ids))
	for i, id := range ids {
		names[i] = byID[id]
	}
	return names
}

type lateClient struct {
	id    string
	name  string
	ended bool
	dueAt time.Time
	dueOn string
}

// TestListHandler_OrdersByTheOldestOverdueInvoiceAcrossPageBoundaries is the
// overdue order: the Client whose oldest owed Invoice has been overdue longest
// first, then every Client with nothing overdue, each group by the default
// key then id. It is read in a Practice whose zone is behind UTC, with the
// Practice's own midnight deciding what is overdue, through two pages by GET
// and by POST and through the attached read ?mine=true shares with a
// contractor's list.
func TestListHandler_OrdersByTheOldestOverdueInvoiceAcrossPageBoundaries(t *testing.T) {
	db := testdb.New(t)
	const ownerUID = "staff-owner-overdue-order"
	practiceID, _ := testdb.SeedStaffAtNewPractice(t, db, ownerUID, []string{ownerRole}, "employee")
	if _, err := db.Admin.ExecContext(t.Context(), `UPDATE practices SET timezone = 'America/Los_Angeles' WHERE id = $1`, practiceID); err != nil {
		t.Fatalf("set zone: %v", err)
	}
	const doulaUID = "employee-overdue-order"
	doula := testdb.SeedStaffAtPractice(t, db, practiceID, doulaUID, []string{doulaRole}, "employee")

	var all []lateClient
	day := func(offset int) time.Time { return behindUTCNow.AddDate(0, 0, offset).Truncate(24 * time.Hour) }
	owes := func(name, dueOn string, dueAts ...time.Time) lateClient {
		clientID, engagementID := testdb.SeedEngagementInStatus(t, db, practiceID, name, "", "active")
		setDueDate(t, db, engagementID, dueOn)
		testdb.SeedGrantedAttachment(t, db, engagementID, doula)
		contractID := seedClientContract(t, db, engagementID, contractSigned)
		for _, dueAt := range dueAts {
			seedClientInvoiceDue(t, db, practiceID, contractID, dueAt)
		}
		c := lateClient{id: clientID, name: name, dueOn: dueOn}
		for _, dueAt := range dueAts {
			if dueAt.Before(time.Date(2026, 12, 9, 8, 0, 0, 0, time.UTC)) && (c.dueAt.IsZero() || dueAt.Before(c.dueAt)) {
				c.dueAt = dueAt
			}
		}
		all = append(all, c)
		return c
	}

	oldest := owes("Oldest", "2027-05-09", day(-40), day(-3))
	owes("Same Day A", "2027-05-02", day(-20))
	owes("Same Day B", "2027-05-01", day(-20))
	owes("Yesterday", "2027-05-08", day(-1))
	owes("Late And Not Yet", "2027-05-07", day(-10), day(30))
	today := owes("Due Today In Her Zone", "2027-05-03", time.Date(2026, 12, 9, 12, 0, 0, 0, time.UTC))
	for i := range 30 {
		owes(fmt.Sprintf("Not Yet %02d", i), fmt.Sprintf("2027-04-%02d", 1+i%28), day(1+i))
	}

	for name, due := range map[string]string{paidUpName: "2027-03-01", onItsWayName: "2027-03-02", voidedName: "2027-03-03", finishedName: "2027-03-04", "No Invoice": "2027-03-05"} {
		status := "active"
		if name == finishedName {
			status = "completed"
		}
		clientID, engagementID := testdb.SeedEngagementInStatus(t, db, practiceID, name, "", status)
		setDueDate(t, db, engagementID, due)
		testdb.SeedGrantedAttachment(t, db, engagementID, doula)
		contractStatus := contractSigned
		if name == voidedName {
			contractStatus = contractVoided
		}
		contractID := seedClientContract(t, db, engagementID, contractStatus)
		switch name {
		case paidUpName:
			seedClientInvoice(t, db, practiceID, contractID, "paid", 99900)
			if _, err := db.Admin.ExecContext(t.Context(), `UPDATE invoices SET due_at = $2 WHERE contract_id = $1`, contractID, day(-50)); err != nil {
				t.Fatalf("backdate: %v", err)
			}
		case onItsWayName:
			seedClientInvoiceDue(t, db, practiceID, contractID, day(-45))
			if _, err := db.Admin.ExecContext(t.Context(), `UPDATE invoices SET payment_processing_since = now() WHERE contract_id = $1`, contractID); err != nil {
				t.Fatalf("mark payment processing: %v", err)
			}
		case voidedName:
			seedClientInvoiceDue(t, db, practiceID, contractID, day(-46))
		case finishedName:
			seedClientInvoiceDue(t, db, practiceID, contractID, day(-47))
		}
		all = append(all, lateClient{id: clientID, name: name, dueOn: due, ended: status == "completed"})
	}

	want := slices.Clone(all)
	slices.SortFunc(want, func(a, b lateClient) int {
		switch {
		case a.ended != b.ended:
			return cmp.Compare(boolInt(a.ended), boolInt(b.ended))
		case a.dueAt.IsZero() && !b.dueAt.IsZero():
			return 1
		case !a.dueAt.IsZero() && b.dueAt.IsZero():
			return -1
		}
		return cmp.Or(a.dueAt.Compare(b.dueAt), cmp.Compare(a.dueOn, b.dueOn), cmp.Compare(a.id, b.id))
	})
	wantIDs := make([]string, len(want))
	for i, c := range want {
		wantIDs[i] = c.id
	}
	if wantIDs[0] != oldest.id {
		t.Fatalf("test model: first = %s, want the oldest", wantIDs[0])
	}

	srv, session := newServerAt(t, db, ownerUID, behindUTCNow)
	defer srv.Close()
	for _, post := range []bool{false, true} {
		got, pages := walkOrdered(t, srv.URL, session, practiceID, "?all=true&order=overdue", orderOverdue, post)
		if pages != 2 {
			t.Errorf("post=%v: %d pages, want 2", post, pages)
		}
		if !slices.Equal(got, wantIDs) {
			t.Errorf("post=%v: order = %v, want %v", post, namesOf(all, got), namesOf(all, wantIDs))
		}
	}

	first := readClients(t, srv.URL, session, practiceID, "?all=true&order=overdue")
	second := readClients(t, srv.URL, session, practiceID, "?all=true&order=overdue&cursor="+*first.NextCursor)
	items := map[string]*int{}
	for _, item := range append(first.Items, second.Items...) {
		items[item.ClientID] = item.OldestOverdueDays
	}
	if got := items[oldest.id]; got == nil || *got != 40 {
		t.Errorf("oldest: oldestOverdueDays = %v, want 40", got)
	}
	if got := items[today.id]; got == nil || *got != 0 {
		t.Errorf("due today in the Practice's zone: oldestOverdueDays = %v, want 0", got)
	}

	doulaSrv, doulaSession := newServerAt(t, db, doulaUID, behindUTCNow)
	defer doulaSrv.Close()
	mine, _ := walkOrdered(t, doulaSrv.URL, doulaSession, practiceID, "?mine=true&all=true&order=overdue", orderOverdue, false)
	if !slices.Equal(mine, wantIDs) {
		t.Errorf("mine: order = %v, want %v", mine, wantIDs)
	}
}
