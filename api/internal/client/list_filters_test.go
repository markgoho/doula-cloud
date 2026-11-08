package client_test

import (
	"encoding/json/v2"
	"net/http"
	"strings"
	"testing"

	"doula-cloud/api/internal/client"
	"doula-cloud/api/internal/testdb"
)

func listWith(t *testing.T, srvURL, session, practiceID, query string) []client.ListItem {
	t.Helper()
	resp := authedGet(t, session, srvURL+"/api/practices/"+practiceID+"/clients"+query)
	defer resp.Body.Close()
	var out client.ListResponse
	if err := json.UnmarshalRead(resp.Body, &out); err != nil || resp.StatusCode != http.StatusOK {
		t.Fatalf("%s: status = %d, err = %v", query, resp.StatusCode, err)
	}
	return out.Items
}

func idsOf(items []client.ListItem) map[string]bool {
	ids := map[string]bool{}
	for _, item := range items {
		ids[item.ClientID] = true
	}
	return ids
}

func wantExactly(t *testing.T, what string, got []client.ListItem, want ...string) {
	t.Helper()
	ids := idsOf(got)
	if len(got) != len(want) {
		t.Fatalf("%s: got %d Clients %v, want %d", what, len(got), ids, len(want))
	}
	for _, id := range want {
		if !ids[id] {
			t.Fatalf("%s: missing %s in %v", what, id, ids)
		}
	}
}

func TestListHandler_OwesNarrowsToClientsWithAnOpenInvoiceNotOnItsWay(t *testing.T) {
	db := testdb.New(t)
	practiceID := testdb.SeedPractice(t, db, "Owes Filter Practice")
	const ownerUID = "staff-owes-filter-owner"
	testdb.SeedStaffAtPractice(t, db, practiceID, ownerUID, []string{ownerRole}, "employee")

	open, openEngagement := testdb.SeedEngagementInStatus(t, db, practiceID, "Open Owes", "open-owes@example.com", "active")
	seedClientInvoice(t, db, practiceID, seedClientContract(t, db, openEngagement, "signed"), "open", 150000)

	overdue, overdueEngagement := testdb.SeedEngagementInStatus(t, db, practiceID, "Overdue Owes", "overdue-owes@example.com", "active")
	seedClientInvoiceDue(t, db, practiceID, seedClientContract(t, db, overdueEngagement, "signed"), lateDueAt)

	_, processingEngagement := testdb.SeedEngagementInStatus(t, db, practiceID, "Processing", "processing-owes@example.com", "active")
	processingContract := seedClientContract(t, db, processingEngagement, "signed")
	seedClientInvoice(t, db, practiceID, processingContract, "open", 150000)
	if _, err := db.Admin.ExecContext(t.Context(),
		`UPDATE invoices SET payment_processing_since = now() WHERE contract_id = $1`, processingContract,
	); err != nil {
		t.Fatalf("mark payment processing: %v", err)
	}

	_, paidEngagement := testdb.SeedEngagementInStatus(t, db, practiceID, "Paid Up", "paid-owes@example.com", "active")
	seedClientInvoice(t, db, practiceID, seedClientContract(t, db, paidEngagement, "signed"), "paid", 150000)

	_, completedEngagement := testdb.SeedEngagementInStatus(t, db, practiceID, "Completed Owes", "completed-owes@example.com", "completed")
	seedClientInvoice(t, db, practiceID, seedClientContract(t, db, completedEngagement, "signed"), "open", 150000)

	_, voidedEngagement := testdb.SeedEngagementInStatus(t, db, practiceID, "Voided Owes", "voided-owes@example.com", "active")
	seedClientInvoice(t, db, practiceID, seedClientContract(t, db, voidedEngagement, "voided"), "open", 150000)

	testdb.SeedEngagementInStatus(t, db, practiceID, "No Invoice", "no-invoice-owes@example.com", "active")

	srv, session := newServer(t, db, ownerUID)
	defer srv.Close()

	wantExactly(t, "owes", listWith(t, srv.URL, session, practiceID, "?owes=true"), open, overdue)
	wantExactly(t, "owes ignores all", listWith(t, srv.URL, session, practiceID, "?owes=true&all=true"), open, overdue)
}

func TestListHandler_NoContractNarrowsToEngagementsPastIntakeWithNoContractRow(t *testing.T) {
	db := testdb.New(t)
	practiceID := testdb.SeedPractice(t, db, "No Contract Filter Practice")
	const ownerUID = "staff-no-contract-filter-owner"
	testdb.SeedStaffAtPractice(t, db, practiceID, ownerUID, []string{ownerRole}, "employee")

	active, _ := testdb.SeedEngagementInStatus(t, db, practiceID, "Active No Contract", "active-none@example.com", "active")
	testdb.SeedEngagementInStatus(t, db, practiceID, "Intake No Contract", "intake-none@example.com", "intake")
	testdb.SeedEngagementInStatus(t, db, practiceID, "Completed No Contract", "completed-none@example.com", "completed")
	_, voidedEngagement := testdb.SeedEngagementInStatus(t, db, practiceID, "Voided Only", "voided-only@example.com", "active")
	seedClientContract(t, db, voidedEngagement, "voided")
	_, signedEngagement := testdb.SeedEngagementInStatus(t, db, practiceID, "Signed", "signed-none@example.com", "active")
	seedClientContract(t, db, signedEngagement, "signed")

	srv, session := newServer(t, db, ownerUID)
	defer srv.Close()

	wantExactly(t, "noContract", listWith(t, srv.URL, session, practiceID, "?noContract=true"), active)
	wantExactly(t, "noContract ignores all", listWith(t, srv.URL, session, practiceID, "?noContract=true&all=true"), active)
}

func TestListHandler_OwesAndNoContractCombineWithEachOtherAndMine(t *testing.T) {
	db := testdb.New(t)
	practiceID, _ := testdb.SeedStaffAtNewPractice(t, db, "staff-owner-filter-combo", []string{ownerRole}, "employee")
	const doulaUID = "employee-doula-filter-combo"
	doulaID := testdb.SeedStaffAtPractice(t, db, practiceID, doulaUID, []string{doulaRole}, "employee")

	mineOwes, mineOwesEngagement := testdb.SeedEngagementInStatus(t, db, practiceID, "Mine Owes", "mine-owes@example.com", "active")
	testdb.SeedGrantedAttachment(t, db, mineOwesEngagement, doulaID)
	seedClientInvoice(t, db, practiceID, seedClientContract(t, db, mineOwesEngagement, "signed"), "open", 150000)

	_, theirOwesEngagement := testdb.SeedEngagementInStatus(t, db, practiceID, "Their Owes", "their-owes@example.com", "active")
	seedClientInvoice(t, db, practiceID, seedClientContract(t, db, theirOwesEngagement, "signed"), "open", 150000)

	mineNone, mineNoneEngagement := testdb.SeedEngagementInStatus(t, db, practiceID, "Mine No Contract", "mine-none@example.com", "active")
	testdb.SeedGrantedAttachment(t, db, mineNoneEngagement, doulaID)

	both, bothOwesEngagement := testdb.SeedEngagementInStatus(t, db, practiceID, "Both", "both-filters@example.com", "active")
	seedClientInvoice(t, db, practiceID, seedClientContract(t, db, bothOwesEngagement, "signed"), "open", 150000)
	seedEngagementForClient(t, db, both, practiceID, "active", "postpartum")

	srv, session := newServer(t, db, doulaUID)
	defer srv.Close()

	wantExactly(t, "mine+owes", listWith(t, srv.URL, session, practiceID, "?mine=true&owes=true"), mineOwes)
	wantExactly(t, "mine+noContract", listWith(t, srv.URL, session, practiceID, "?mine=true&noContract=true"), mineNone)
	if got := listWith(t, srv.URL, session, practiceID, "?owes=true"); len(got) != 3 {
		t.Fatalf("owes without mine = %d Clients, want 3", len(got))
	}
	wantExactly(t, "owes+noContract", listWith(t, srv.URL, session, practiceID, "?owes=true&noContract=true"), both)
}

func TestListHandler_OwesRefusesAnAmbientContractorButNoContractIsAllowed(t *testing.T) {
	db := testdb.New(t)
	practiceID := testdb.SeedPractice(t, db, "Owes Contractor Practice")
	const contractorUID = "contractor-owes-filter"
	contractorID := testdb.SeedContractorAtPractice(t, db, practiceID, contractorUID)
	mine, engagementID := testdb.SeedEngagementInStatus(t, db, practiceID, "Mine", "mine-contractor@example.com", "active")
	testdb.SeedGrantedAttachment(t, db, engagementID, contractorID)

	srv, session := newServer(t, db, contractorUID)
	defer srv.Close()

	resp := authedGet(t, session, srv.URL+"/api/practices/"+practiceID+"/clients?owes=true")
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("owes status = %d, want 403", resp.StatusCode)
	}
	var body struct {
		Message string `json:"message"`
	}
	if err := json.UnmarshalRead(resp.Body, &body); err != nil || !strings.Contains(body.Message, "what they owe") {
		t.Fatalf("owes body = %+v, err = %v, want a plain-English refusal", body, err)
	}

	wantExactly(t, "noContract", listWith(t, srv.URL, session, practiceID, "?noContract=true"), mine)
}
