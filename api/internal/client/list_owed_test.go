package client_test

import (
	"slices"
	"testing"

	"doula-cloud/api/internal/client"
	"doula-cloud/api/internal/testdb"
)

func owedByName(t *testing.T, page client.ListResponse) map[string]client.ListItem {
	t.Helper()
	byName := map[string]client.ListItem{}
	for _, item := range page.Items {
		byName[item.Name] = item
	}
	return byName
}

func wantOwed(t *testing.T, item client.ListItem, cents int64, count int) {
	t.Helper()
	if item.OwedCents == nil || item.OwedInvoiceCount == nil {
		t.Fatalf("%s: owedCents or owedInvoiceCount absent, want %d and %d", item.Name, cents, count)
	}
	if *item.OwedCents != cents || *item.OwedInvoiceCount != count {
		t.Fatalf("%s: owed = %d over %d Invoices, want %d over %d", item.Name, *item.OwedCents, *item.OwedInvoiceCount, cents, count)
	}
}

// #2874: every row says what the Client owes in all and over how many Invoices,
// in whatever order the list is read.
func TestListHandler_EachRowCarriesWhatTheClientOwesInAll(t *testing.T) {
	db := testdb.New(t)
	practiceID, _ := testdb.SeedStaffAtNewPractice(t, db, "staff-owner-owed-total", []string{ownerRole}, "employee")

	_, depositAndBalance := testdb.SeedEngagementInStatus(t, db, practiceID, "Deposit Balance", "", "active")
	contractID := seedClientContract(t, db, depositAndBalance, contractSigned)
	seedClientInvoice(t, db, practiceID, contractID, "open", 10000)
	seedClientInvoice(t, db, practiceID, contractID, "open", 40000)
	seedClientInvoice(t, db, practiceID, contractID, "paid", 99900)

	twoID, twoFirst := testdb.SeedEngagementInStatus(t, db, practiceID, "Two Engagements", "", "active")
	seedClientInvoice(t, db, practiceID, seedClientContract(t, db, twoFirst, contractSigned), "open", 20000)
	twoSecond := seedEngagementForClient(t, db, twoID, practiceID, "active", "postpartum")
	seedClientInvoice(t, db, practiceID, seedClientContract(t, db, twoSecond, contractSigned), "open", 30000)
	finished := seedEngagementForClient(t, db, twoID, practiceID, "completed", "birth")
	seedClientInvoice(t, db, practiceID, seedClientContract(t, db, finished, contractSigned), "open", 70000)

	_, onItsWay := testdb.SeedEngagementInStatus(t, db, practiceID, "On Its Way", "", "active")
	wayContract := seedClientContract(t, db, onItsWay, contractSigned)
	seedClientInvoice(t, db, practiceID, wayContract, "open", 25000)
	seedClientInvoice(t, db, practiceID, wayContract, "open", 5000)
	if _, err := db.Admin.ExecContext(t.Context(),
		`UPDATE invoices SET payment_processing_since = now() WHERE contract_id = $1 AND amount_cents = 25000`, wayContract,
	); err != nil {
		t.Fatalf("mark payment processing: %v", err)
	}

	_, voided := testdb.SeedEngagementInStatus(t, db, practiceID, "Voided Contract", "", "active")
	seedClientInvoice(t, db, practiceID, seedClientContract(t, db, voided, "voided"), "open", 60000)

	testdb.SeedEngagementInStatus(t, db, practiceID, "Owes Nothing", "", "active")

	srv, session := newServer(t, db, "staff-owner-owed-total")
	defer srv.Close()

	for _, query := range []string{"?all=true", "?all=true&order=name", "?all=true&order=owed"} {
		rows := owedByName(t, readClients(t, srv.URL, session, practiceID, query))
		wantOwed(t, rows["Deposit Balance"], 50000, 2)
		wantOwed(t, rows["Two Engagements"], 50000, 2)
		wantOwed(t, rows["On Its Way"], 5000, 1)
		wantOwed(t, rows["Voided Contract"], 0, 0)
		wantOwed(t, rows["Owes Nothing"], 0, 0)
	}
}

// #2874: a Client owing two smaller Invoices is above one owing a single larger
// one when the sum is bigger, and below it when the sum is smaller.
func TestListHandler_MostOwedFirstOrdersBySumNotBySingleInvoice(t *testing.T) {
	db := testdb.New(t)
	practiceID, _ := testdb.SeedStaffAtNewPractice(t, db, "staff-owner-owed-sum", []string{ownerRole}, "employee")
	owing := func(name string, amounts ...int64) string {
		clientID, engagementID := testdb.SeedEngagementInStatus(t, db, practiceID, name, "", "active")
		contractID := seedClientContract(t, db, engagementID, contractSigned)
		for _, amount := range amounts {
			seedClientInvoice(t, db, practiceID, contractID, "open", amount)
		}
		return clientID
	}
	single := owing("Single Larger", 35000)
	pair := owing("Two Smaller", 20000, 20000)
	smallPair := owing("Small Pair", 10000, 10000)

	srv, session := newServer(t, db, "staff-owner-owed-sum")
	defer srv.Close()

	got := idsInOrder(readClients(t, srv.URL, session, practiceID, "?all=true&order=owed").Items)
	if want := []string{pair, single, smallPair}; !slices.Equal(got, want) {
		t.Fatalf("most owed first = %v, want %v", got, want)
	}
}

func TestListHandler_ARowToldToAContractorCarriesNoOwedTotal(t *testing.T) {
	db := testdb.New(t)
	practiceID, _ := testdb.SeedStaffAtNewPractice(t, db, "staff-owner-owed-contractor", []string{ownerRole}, "employee")
	const contractorUID = "contractor-owed-total"
	contractor := testdb.SeedContractorAtPractice(t, db, practiceID, contractorUID)
	_, engagementID := testdb.SeedEngagementInStatus(t, db, practiceID, "Hers", "", "active")
	seedClientInvoice(t, db, practiceID, seedClientContract(t, db, engagementID, contractSigned), "open", 10000)
	testdb.SeedGrantedAttachment(t, db, engagementID, contractor)

	srv, session := newServer(t, db, contractorUID)
	defer srv.Close()

	page := readClients(t, srv.URL, session, practiceID, "")
	if len(page.Items) != 1 {
		t.Fatalf("items = %d, want 1", len(page.Items))
	}
	if item := page.Items[0]; item.OwedCents != nil || item.OwedInvoiceCount != nil {
		t.Fatalf("a contractor's row carries an owed total: %+v", item)
	}
}
