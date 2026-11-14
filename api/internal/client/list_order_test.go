package client_test

import (
	"cmp"
	"fmt"
	"net/http"
	"slices"
	"strings"
	"testing"
	"time"

	"doula-cloud/api/internal/client"
	"doula-cloud/api/internal/pagecursor"
	"doula-cloud/api/internal/testdb"
)

const (
	orderName     = "name"
	orderNameDesc = "name-desc"
	orderOwed     = "owed"

	contractSigned = "signed"
	voidedName     = "Voided"
)

// walkOrdered follows nextCursor through every page of the list in order,
// by GET or, when post is set, by POST with the order in the body, and
// returns the Client ids in the order they came and how many pages it took.
func walkOrdered(t *testing.T, srvURL, session, practiceID, query, order string, post bool) (ids []string, pages int) {
	t.Helper()
	cursor := ""
	for {
		var page client.ListResponse
		if post {
			page = listByNameResponse(t, srvURL, session, practiceID, client.ListRequest{All: true, Order: order, Cursor: cursor})
		} else {
			q := query
			if cursor != "" {
				q += "&cursor=" + cursor
			}
			page = readClients(t, srvURL, session, practiceID, q)
		}
		pages++
		ids = append(ids, idsInOrder(page.Items)...)
		if !page.HasMore || page.NextCursor == nil {
			return ids, pages
		}
		cursor = *page.NextCursor
	}
}

type namedClient struct {
	id, key string
}

// seedNamed seeds a Client with an active Engagement under the given
// preferred and family names and returns the key the name order sorts her by.
func seedNamed(t *testing.T, db *testdb.DB, practiceID, given, preferred, family string) namedClient {
	t.Helper()
	clientID, _ := testdb.SeedEngagementInStatus(t, db, practiceID, given, "", "active")
	testdb.NameClient(t, db, clientID, preferred, family)
	first := given
	if preferred != "" {
		first = preferred
	}
	full := first
	if family != "" {
		full += " " + family
	}
	return namedClient{id: clientID, key: strings.ToLower(full)}
}

func byNameThenID(a, b namedClient) int {
	return cmp.Or(cmp.Compare(a.key, b.key), cmp.Compare(a.id, b.id))
}

func namedIDs(clients []namedClient) []string {
	ids := make([]string, len(clients))
	for i, c := range clients {
		ids[i] = c.id
	}
	return ids
}

// TestListHandler_OrdersByNameAcrossPageBoundariesWithTiesByID seeds 36
// Clients of which 33 share one name, so the tie runs across the page
// boundary of thirty, and walks the list in both name orders by GET and by
// POST: no Client is skipped or repeated, and ties fall to id in the
// direction of the order.
func TestListHandler_OrdersByNameAcrossPageBoundariesWithTiesByID(t *testing.T) {
	db := testdb.New(t)
	practiceID, _ := testdb.SeedStaffAtNewPractice(t, db, "staff-owner-name-order", []string{ownerRole}, "employee")

	var all []namedClient
	for range 33 {
		all = append(all, seedNamed(t, db, practiceID, "Marisol", "", "Vega"))
	}
	// Cased apart, a preferred name that wins over the given one, a name with
	// no family name: each is read as the list reads it (personname.Full).
	all = append(all,
		seedNamed(t, db, practiceID, "ALMA", "", "Cruz"),
		seedNamed(t, db, practiceID, "Elizabeth", "Beth", "Okafor"),
		seedNamed(t, db, practiceID, "Zed", "", ""),
	)

	srv, session := newServer(t, db, "staff-owner-name-order")
	defer srv.Close()

	asc := slices.Clone(all)
	slices.SortFunc(asc, byNameThenID)
	desc := slices.Clone(all)
	slices.SortFunc(desc, func(a, b namedClient) int { return -byNameThenID(a, b) })

	for name, c := range map[string]struct {
		order string
		want  []namedClient
	}{
		orderName:     {orderName, asc},
		orderNameDesc: {orderNameDesc, desc},
	} {
		for _, post := range []bool{false, true} {
			got, pages := walkOrdered(t, srv.URL, session, practiceID, "?all=true&order="+c.order, c.order, post)
			if pages != 2 {
				t.Errorf("%s (post=%v): %d pages, want 2", name, post, pages)
			}
			if !slices.Equal(got, namedIDs(c.want)) {
				t.Errorf("%s (post=%v): order = %v, want %v", name, post, got, namedIDs(c.want))
			}
		}
	}
}

// TestListHandler_OrderDoesNotChangeTheCount: the total is of the rows the
// narrowing holds, whichever order they are read in.
func TestListHandler_OrderDoesNotChangeTheCount(t *testing.T) {
	db := testdb.New(t)
	practiceID, _ := testdb.SeedStaffAtNewPractice(t, db, "staff-owner-order-count", []string{ownerRole}, "employee")
	for i := range 31 {
		testdb.SeedEngagementInStatus(t, db, practiceID, fmt.Sprintf("Ordered %02d", i), "", "active")
	}
	srv, session := newServer(t, db, "staff-owner-order-count")
	defer srv.Close()

	for _, order := range []string{"", orderName, orderNameDesc, orderOwed} {
		page := readClients(t, srv.URL, session, practiceID, "?order="+order)
		wantClientCount(t, page, 31)
		if len(page.Items) != 30 || !page.HasMore {
			t.Errorf("order %q: %d items, hasMore=%v, want 30/true", order, len(page.Items), page.HasMore)
		}
	}
}

type owingClient struct {
	id    string
	owed  int64
	dueOn string
}

// TestListHandler_OrdersByWhatTheMoneyLinesShowAsOwed is the owed order:
// the sum over a Client's open Engagements of the amount each line shows as
// owed (its live Contract's oldest open Invoice, with no payment on its
// way), most first, then the default key, then id. A paid Invoice, one whose
// payment is on its way and a voided Contract's add nothing.
func TestListHandler_OrdersByWhatTheMoneyLinesShowAsOwed(t *testing.T) {
	db := testdb.New(t)
	practiceID, _ := testdb.SeedStaffAtNewPractice(t, db, "staff-owner-owed-order", []string{ownerRole}, "employee")

	var all []owingClient
	owes := func(name, dueOn string, amounts ...int64) owingClient {
		clientID, engagementID := testdb.SeedEngagementInStatus(t, db, practiceID, name, "", "active")
		setDueDate(t, db, engagementID, dueOn)
		contractID := seedClientContract(t, db, engagementID, contractSigned)
		var owed int64
		for i, amount := range amounts {
			seedClientInvoice(t, db, practiceID, contractID, "open", amount)
			if i == 0 {
				owed = amount
			}
		}
		c := owingClient{id: clientID, owed: owed, dueOn: dueOn}
		all = append(all, c)
		return c
	}

	// Two Engagements add up: 200 and 300.
	two := owes("Two Engagements", "2027-02-01", 20000)
	secondEngagement := seedEngagementForClient(t, db, two.id, practiceID, "active", "postpartum")
	setDueDate(t, db, secondEngagement, "2027-02-01")
	seedClientInvoice(t, db, practiceID, seedClientContract(t, db, secondEngagement, contractSigned), "open", 30000)
	all[len(all)-1].owed = 50000

	// The line shows the oldest open Invoice, 100, never the 400 after it.
	owes("Deposit And Balance", "2027-03-01", 10000, 40000)

	owes("Mid", "2027-03-05", 30000)

	// Thirty-three tie at 150, so the tie crosses the boundary of thirty and
	// falls to the due date, then id.
	for i := range 33 {
		owes(fmt.Sprintf("Tied %02d", i), fmt.Sprintf("2027-04-%02d", 1+i%10), 15000)
	}

	for name, due := range map[string]string{"Paid Up": "2027-01-05", "On Its Way": "2027-01-06", voidedName: "2027-01-07", "No Invoice": "2027-01-08"} {
		clientID, engagementID := testdb.SeedEngagementInStatus(t, db, practiceID, name, "", "active")
		setDueDate(t, db, engagementID, due)
		contractStatus := contractSigned
		if name == voidedName {
			contractStatus = "voided"
		}
		contractID := seedClientContract(t, db, engagementID, contractStatus)
		switch name {
		case "Paid Up":
			seedClientInvoice(t, db, practiceID, contractID, "paid", 99900)
		case "On Its Way":
			seedClientInvoice(t, db, practiceID, contractID, "open", 99900)
			if _, err := db.Admin.ExecContext(t.Context(),
				`UPDATE invoices SET payment_processing_since = now() WHERE contract_id = $1`, contractID,
			); err != nil {
				t.Fatalf("mark payment processing: %v", err)
			}
		case voidedName:
			seedClientInvoice(t, db, practiceID, contractID, "void", 99900)
		}
		all = append(all, owingClient{id: clientID, dueOn: due})
	}

	srv, session := newServer(t, db, "staff-owner-owed-order")
	defer srv.Close()

	want := slices.Clone(all)
	slices.SortFunc(want, func(a, b owingClient) int {
		return cmp.Or(cmp.Compare(b.owed, a.owed), cmp.Compare(a.dueOn, b.dueOn), cmp.Compare(a.id, b.id))
	})
	wantIDs := make([]string, len(want))
	for i, c := range want {
		wantIDs[i] = c.id
	}

	for _, post := range []bool{false, true} {
		got, pages := walkOrdered(t, srv.URL, session, practiceID, "?all=true&order=owed", orderOwed, post)
		if pages != 2 {
			t.Errorf("post=%v: %d pages, want 2", post, pages)
		}
		if !slices.Equal(got, wantIDs) {
			t.Errorf("post=%v: order = %v, want %v", post, got, wantIDs)
		}
	}
}

// TestListHandler_MineReadsInTheChosenOrderToo: the attached read, which
// ?mine=true and a contractor's list share, pages through every order
// without a gap or a repeat.
func TestListHandler_MineReadsInTheChosenOrderToo(t *testing.T) {
	db := testdb.New(t)
	practiceID, _ := testdb.SeedStaffAtNewPractice(t, db, "staff-owner-mine-order", []string{ownerRole}, "employee")
	const doulaUID = "employee-mine-order"
	doula := testdb.SeedStaffAtPractice(t, db, practiceID, doulaUID, []string{doulaRole}, "employee")

	var mine []namedClient
	var owed []owingClient
	for i := range 33 {
		c := seedNamed(t, db, practiceID, fmt.Sprintf("Mine %02d", i%4), "", "Tied")
		engagements := db.Admin.QueryRowContext(t.Context(), `SELECT id FROM engagements WHERE client_id = $1`, c.id)
		var engagementID string
		if err := engagements.Scan(&engagementID); err != nil {
			t.Fatalf("read engagement: %v", err)
		}
		testdb.SeedGrantedAttachment(t, db, engagementID, doula)
		// A second Engagement she holds for the same Client must not double her row.
		second := seedEngagementForClient(t, db, c.id, practiceID, "active", "postpartum")
		testdb.SeedGrantedAttachment(t, db, second, doula)
		setDueDate(t, db, engagementID, fmt.Sprintf("2027-05-%02d", 1+i%9))
		amount := int64(10000 * (i % 3))
		if amount > 0 {
			seedClientInvoice(t, db, practiceID, seedClientContract(t, db, engagementID, contractSigned), "open", amount)
		}
		mine = append(mine, c)
		owed = append(owed, owingClient{id: c.id, owed: amount, dueOn: fmt.Sprintf("2027-05-%02d", 1+i%9)})
	}
	testdb.SeedEngagementInStatus(t, db, practiceID, "Not Mine", "", "active")

	srv, session := newServer(t, db, doulaUID)
	defer srv.Close()

	byName := slices.Clone(mine)
	slices.SortFunc(byName, byNameThenID)
	got, _ := walkOrdered(t, srv.URL, session, practiceID, "?mine=true&order=name", orderName, false)
	if !slices.Equal(got, namedIDs(byName)) {
		t.Errorf("mine by name: order = %v, want %v", got, namedIDs(byName))
	}

	byOwed := slices.Clone(owed)
	slices.SortFunc(byOwed, func(a, b owingClient) int {
		return cmp.Or(cmp.Compare(b.owed, a.owed), cmp.Compare(a.dueOn, b.dueOn), cmp.Compare(a.id, b.id))
	})
	wantOwed := make([]string, len(byOwed))
	for i, c := range byOwed {
		wantOwed[i] = c.id
	}
	got, _ = walkOrdered(t, srv.URL, session, practiceID, "?mine=true&order=owed", orderOwed, false)
	if !slices.Equal(got, wantOwed) {
		t.Errorf("mine by owed: order = %v, want %v", got, wantOwed)
	}
}

// TestListHandler_ContractorReadsByNameButNotByWhatIsOwed: a contractor sees
// no money, so ordering by it is refused rather than answered in some other
// order; her own rows read in name order like anyone's.
func TestListHandler_ContractorReadsByNameButNotByWhatIsOwed(t *testing.T) {
	db := testdb.New(t)
	practiceID, _ := testdb.SeedStaffAtNewPractice(t, db, "staff-owner-contractor-order", []string{ownerRole}, "employee")
	const contractorUID = "contractor-order"
	contractor := testdb.SeedContractorAtPractice(t, db, practiceID, contractorUID)

	var hers []namedClient
	for _, name := range []string{"Zoe", "Anna", "Mira"} {
		c := seedNamed(t, db, practiceID, name, "", "Reyes")
		var engagementID string
		if err := db.Admin.QueryRowContext(t.Context(), `SELECT id FROM engagements WHERE client_id = $1`, c.id).Scan(&engagementID); err != nil {
			t.Fatalf("read engagement: %v", err)
		}
		testdb.SeedGrantedAttachment(t, db, engagementID, contractor)
		hers = append(hers, c)
	}
	seedNamed(t, db, practiceID, "Aaron", "", "Nothers")

	srv, session := newServer(t, db, contractorUID)
	defer srv.Close()

	slices.SortFunc(hers, byNameThenID)
	got, _ := walkOrdered(t, srv.URL, session, practiceID, "?order=name", orderName, false)
	if !slices.Equal(got, namedIDs(hers)) {
		t.Errorf("contractor by name: order = %v, want %v", got, namedIDs(hers))
	}
	if status := listStatus(t, srv.URL, session, practiceID, "?order=owed"); status != http.StatusBadRequest {
		t.Errorf("GET order=owed as a contractor: status = %d, want 400", status)
	}
	if status := listByNameStatus(t, srv.URL, session, practiceID, client.ListRequest{Order: orderOwed}); status != http.StatusBadRequest {
		t.Errorf("POST order=owed as a contractor: status = %d, want 400", status)
	}
}

// TestListHandler_RefusesAnUnknownOrderAndACursorFromAnotherOrder: a link
// with a typo fails, and a cursor is only good in the order it was minted
// for, whichever way it was damaged or handed over.
func TestListHandler_RefusesAnUnknownOrderAndACursorFromAnotherOrder(t *testing.T) {
	db := testdb.New(t)
	practiceID, _ := testdb.SeedStaffAtNewPractice(t, db, "staff-owner-bad-order", []string{ownerRole}, "employee")
	for i := range 31 {
		testdb.SeedEngagementInStatus(t, db, practiceID, fmt.Sprintf("Cursor %02d", i), "", "active")
	}
	srv, session := newServer(t, db, "staff-owner-bad-order")
	defer srv.Close()

	for _, order := range []string{"newest", "Name", "name-asc", "owed "} {
		if status := listStatus(t, srv.URL, session, practiceID, "?order="+order); status != http.StatusBadRequest {
			t.Errorf("GET order=%q: status = %d, want 400", order, status)
		}
		if status := listByNameStatus(t, srv.URL, session, practiceID, client.ListRequest{Order: order}); status != http.StatusBadRequest {
			t.Errorf("POST order=%q: status = %d, want 400", order, status)
		}
	}

	const id = "00000000-0000-0000-0000-000000000000"
	now := pagecursor.TimeKey(time.Now())
	for name, c := range map[string]struct{ order, cursor string }{
		"default cursor in name order": {orderName, *readClients(t, srv.URL, session, practiceID, "").NextCursor},
		"name cursor in default order": {"", *readClients(t, srv.URL, session, practiceID, "?order=name").NextCursor},
		"name cursor in the reverse":   {orderNameDesc, *readClients(t, srv.URL, session, practiceID, "?order=name").NextCursor},
		"owed cursor in name order":    {orderName, *readClients(t, srv.URL, session, practiceID, "?order=owed").NextCursor},
		"default, no keys":             {"", pagecursor.EncodeKeyed("", nil, id)},
		"default, not a time":          {"", pagecursor.EncodeKeyed("", []string{"soon"}, id)},
		"name, no keys":                {orderName, pagecursor.EncodeKeyed(orderName, nil, id)},
		"owed, one key":                {orderOwed, pagecursor.EncodeKeyed(orderOwed, []string{"100"}, id)},
		"owed, not an amount":          {orderOwed, pagecursor.EncodeKeyed(orderOwed, []string{"lots", now}, id)},
		"owed, not a time":             {orderOwed, pagecursor.EncodeKeyed(orderOwed, []string{"100", "soon"}, id)},
	} {
		if status := listStatus(t, srv.URL, session, practiceID, "?order="+c.order+"&cursor="+c.cursor); status != http.StatusBadRequest {
			t.Errorf("GET %s: status = %d, want 400", name, status)
		}
		if status := listByNameStatus(t, srv.URL, session, practiceID, client.ListRequest{Order: c.order, Cursor: c.cursor}); status != http.StatusBadRequest {
			t.Errorf("POST %s: status = %d, want 400", name, status)
		}
	}
}
