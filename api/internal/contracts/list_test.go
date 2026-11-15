package contracts_test

import (
	"fmt"
	"net/http"
	"slices"
	"sort"
	"strings"
	"testing"
	"time"

	"doula-cloud/api/internal/apierrtest"
	"doula-cloud/api/internal/contracts"
	"doula-cloud/api/internal/fixture"
	"doula-cloud/api/internal/pagecursor"
	"doula-cloud/api/internal/reqtest"
	"doula-cloud/api/internal/testdb"
)

var listBase = time.Date(2026, 11, 1, 15, 0, 0, 0, time.UTC)

const (
	viewAll        = "all"
	orderOldest    = "oldest"
	orderNewest    = "newest"
	orderClient    = "client"
	listPageSize   = 30
	listAmount     = 150000
	msgBadView     = "Choose which Contracts to show."
	msgBadOrder    = "Choose how the Contracts are ordered: oldest or newest first, or by Client A to Z."
	msgBadDoula    = "Choose a Doula from the list."
	msgSearchInGet = "Send the search in a POST to /contracts/list, not in the address."
	msgSearchLong  = "Use 100 characters or fewer for the search."
)

func (e *env) listURL(query string) string { return e.PracticeURL("/contracts" + query) }

func (e *env) searchURL() string { return e.PracticeURL("/contracts/list") }

// list reads the Contracts list as the Staff member at the screen.
func (e *env) list(query string) contracts.ListResponse {
	e.T.Helper()
	return reqtest.Expect[contracts.ListResponse](e.T, http.MethodGet, e.listURL(query), http.StatusOK, e.SignedIn)
}

// search reads the Contracts list through the POST that carries a search.
func (e *env) search(req contracts.ListRequest) contracts.ListResponse {
	e.T.Helper()
	return reqtest.Expect[contracts.ListResponse](e.T, http.MethodPost, e.searchURL(), http.StatusOK, e.SignedIn, reqtest.JSON(req))
}

// refusedList requires the list to refuse query with 400 and message.
func (e *env) refusedList(query, message string) {
	e.T.Helper()
	got := apierrtest.Expect(e.T, http.MethodGet, e.listURL(query), http.StatusBadRequest, e.SignedIn)
	if got.Message != message {
		e.T.Fatalf("GET %s message = %q, want %q", query, got.Message, message)
	}
}

// seedListed seeds a Contract at status whose current step happened at
// stepped, on a further Engagement.
func (e *env) seedListed(status string, stepped time.Time, opts ...fixture.EngagementOption) (fixture.EngagementRecord, string) {
	e.T.Helper()
	engagement := e.AnotherEngagement(opts...)
	return engagement, seedListedContract(e.T, e.DB, engagement.ID, status, stepped)
}

// seedEveryStatus seeds one Contract in each of the four statuses.
func (e *env) seedEveryStatus() {
	e.T.Helper()
	for _, status := range []string{statusDraft, statusSent, statusSigned, statusVoided} {
		e.seedListed(status, listBase)
	}
}

// seedListedContract inserts a Contract at status with the columns its
// status requires, its current step falling at stepped.
func seedListedContract(t *testing.T, db *testdb.DB, engagementID, status string, stepped time.Time) string {
	t.Helper()
	var created, sent, signed, voided any
	switch status {
	case statusDraft:
		created = stepped
	case statusSent:
		created, sent = stepped.Add(-2*time.Hour), stepped
	case statusSigned:
		created, sent, signed = stepped.Add(-3*time.Hour), stepped.Add(-2*time.Hour), stepped
	case statusVoided:
		created, sent, signed, voided = stepped.Add(-3*time.Hour), stepped.Add(-2*time.Hour), stepped.Add(-time.Hour), stepped
	}
	var id string
	if err := db.Admin.QueryRowContext(t.Context(),
		`INSERT INTO contracts (engagement_id, status, prose, amount_cents, created_at, sent_at, signed_at, voided_at)
		 VALUES ($1, $2::contract_status, 'Agreement.', $3, $4, $5, $6, $7) RETURNING id`,
		engagementID, status, listAmount, created, sent, signed, voided,
	).Scan(&id); err != nil {
		t.Fatalf("seed listed contract: %v", err)
	}
	return id
}

func ids(out contracts.ListResponse) []string {
	got := make([]string, 0, len(out.Items))
	for _, item := range out.Items {
		got = append(got, item.ContractID)
	}
	return got
}

func statuses(out contracts.ListResponse) []string {
	got := make([]string, 0, len(out.Items))
	for _, item := range out.Items {
		got = append(got, item.Status)
	}
	sort.Strings(got)
	return got
}

// TestListContracts_EachViewHoldsItsStatuses proves the six views: not
// signed is the default and holds Drafts and Sent Contracts, and each
// other view its own state.
func TestListContracts_EachViewHoldsItsStatuses(t *testing.T) {
	e := newEnv(t, signedInAs(fixture.Owner()))
	e.seedEveryStatus()

	cases := []struct {
		view string
		want []string
	}{
		{"", []string{statusDraft, statusSent}},
		{statusDraft, []string{statusDraft}},
		{statusSent, []string{statusSent}},
		{statusSigned, []string{statusSigned}},
		{statusVoided, []string{statusVoided}},
		{viewAll, []string{statusDraft, statusSent, statusSigned, statusVoided}},
	}
	for _, c := range cases {
		t.Run("view="+c.view, func(t *testing.T) {
			out := e.list("?view=" + c.view)
			if got := statuses(out); !slices.Equal(got, c.want) || out.Total != len(c.want) {
				t.Fatalf("view %q = %v (total %d), want %v", c.view, got, out.Total, c.want)
			}
		})
	}
}

// TestListContracts_EmptyPracticeAnswersWithAnEmptyList proves a Practice
// with no Contract gets [] rather than null, asserted on the raw bytes.
func TestListContracts_EmptyPracticeAnswersWithAnEmptyList(t *testing.T) {
	e := newEnv(t, signedInAs(fixture.Owner()))

	body := reqtest.ExpectBody(t, http.MethodGet, e.listURL(""), http.StatusOK, e.SignedIn)

	if !strings.Contains(string(body), `"items":[]`) || !strings.Contains(string(body), `"total":0`) || strings.Contains(string(body), "nextCursor") {
		t.Fatalf("body = %s, want an empty items array, a zero total and no cursor", body)
	}
}

// TestListContracts_IgnoresAnotherPracticesContracts proves the list is
// scoped to the caller's Practice.
func TestListContracts_IgnoresAnotherPracticesContracts(t *testing.T) {
	e := newEnv(t, signedInAs(fixture.Owner()))
	otherPracticeID := testdb.SeedPractice(t, e.DB, "Other Practice")
	_, otherEngagement := testdb.SeedEngagement(t, e.DB, otherPracticeID)
	seedListedContract(t, e.DB, otherEngagement, statusSent, listBase)
	_, mine := e.seedListed(statusSent, listBase)

	out := e.list("?view=" + viewAll)

	if got := ids(out); !slices.Equal(got, []string{mine}) || out.Total != 1 {
		t.Fatalf("ids = %v (total %d), want only this Practice's %s", got, out.Total, mine)
	}
}

type seededContract struct {
	id      string
	status  string
	stepped time.Time
	nameKey string
}

// seedBook seeds perStatus Contracts in each status with ties on the date
// of the current step and on the Client's name, so every order has a tie
// for the id to break.
func seedBook(t *testing.T, e *env) []seededContract {
	const perStatus = listPageSize + 1
	t.Helper()
	var book []seededContract
	for _, status := range []string{statusDraft, statusSent, statusSigned, statusVoided} {
		for i := range perStatus {
			family := fmt.Sprintf("F%02d", i%17)
			stepped := listBase.Add(time.Duration(i%12) * time.Hour)
			engagement := e.AnotherEngagement(fixture.NewClient(fixture.ClientName("Client")))
			testdb.NameClient(t, e.DB, engagement.Client.ID, "", family)
			id := seedListedContract(t, e.DB, engagement.ID, status, stepped)
			book = append(book, seededContract{id: id, status: status, stepped: stepped, nameKey: strings.ToLower("Client " + family)})
		}
	}
	return book
}

func inView(view string) func(seededContract) bool {
	return func(c seededContract) bool {
		switch view {
		case "":
			return c.status == statusDraft || c.status == statusSent
		case viewAll:
			return true
		}
		return c.status == view
	}
}

func ownOrder(view string) string {
	switch view {
	case "", statusDraft, statusSent:
		return orderOldest
	}
	return orderNewest
}

// expectedOrder is the book's rows in view, ordered by the key and the id,
// the way the list says it sorts.
func expectedOrder(book []seededContract, view, order string) []string {
	if order == "" {
		order = ownOrder(view)
	}
	var rows []seededContract
	for _, c := range book {
		if inView(view)(c) {
			rows = append(rows, c)
		}
	}
	sort.Slice(rows, func(i, j int) bool {
		a, b := rows[i], rows[j]
		switch order {
		case orderNewest:
			if !a.stepped.Equal(b.stepped) {
				return a.stepped.After(b.stepped)
			}
			return a.id > b.id
		case orderClient:
			if a.nameKey != b.nameKey {
				return a.nameKey < b.nameKey
			}
		default:
			if view == "" && (a.status == statusDraft) != (b.status == statusDraft) {
				return b.status == statusDraft
			}
			if !a.stepped.Equal(b.stepped) {
				return a.stepped.Before(b.stepped)
			}
		}
		return a.id < b.id
	})
	got := make([]string, len(rows))
	for i, c := range rows {
		got[i] = c.id
	}
	return got
}

// TestListContracts_EveryViewAndOrderWalksItsCursor walks every view in
// every order a page at a time and requires the rows in the order the list
// says it sorts by, a tie on the key falling to the id, and the total on
// every page equal to the rows across them.
func TestListContracts_EveryViewAndOrderWalksItsCursor(t *testing.T) {
	e := newEnv(t, signedInAs(fixture.Owner()))
	book := seedBook(t, e)

	for _, view := range []string{"", statusDraft, statusSent, statusSigned, statusVoided, viewAll} {
		for _, order := range []string{"", orderOldest, orderNewest, orderClient} {
			t.Run(fmt.Sprintf("view=%s/order=%s", view, order), func(t *testing.T) {
				want := expectedOrder(book, view, order)
				var got []string
				cursor := ""
				for range 10 {
					query := "?view=" + view + "&order=" + order
					if cursor != "" {
						query += "&cursor=" + cursor
					}
					out := e.list(query)
					if out.Total != len(want) {
						t.Fatalf("total = %d, want %d on every page", out.Total, len(want))
					}
					got = append(got, ids(out)...)
					if !out.HasMore {
						break
					}
					if len(out.Items) != listPageSize || out.NextCursor == nil {
						t.Fatalf("page = %d items, cursor %v; want a full page and a cursor", len(out.Items), out.NextCursor)
					}
					cursor = *out.NextCursor
				}
				if !slices.Equal(got, want) {
					t.Fatalf("order = %v, want %v", got, want)
				}
			})
		}
	}
}

// TestListContracts_ACursorBelongsToItsViewAndOrder proves a cursor used
// under another view or order, or named by another spelling of the same
// order, is a 400 with the shared message, and so is one that does not
// decode.
func TestListContracts_ACursorBelongsToItsViewAndOrder(t *testing.T) {
	e := newEnv(t, signedInAs(fixture.Owner()))
	seedBook(t, e)
	first := e.list("?view=" + statusSigned + "&order=" + orderNewest + "")
	if first.NextCursor == nil {
		t.Fatal("first page carries no cursor")
	}
	cursor := *first.NextCursor

	for _, query := range []string{
		"?view=voided&order=newest&cursor=" + cursor,
		"?view=signed&order=oldest&cursor=" + cursor,
		"?view=signed&cursor=" + cursor,
		"?view=" + statusSigned + "&order=" + orderNewest + "&cursor=not!valid!base64!",
		"?view=" + statusSigned + "&order=" + orderNewest + "&cursor=" + pagecursor.EncodeKeyed("signed:newest", []string{}, "an-id"),
		"?view=" + statusSigned + "&order=" + orderNewest + "&cursor=" + pagecursor.EncodeKeyed("signed:newest", []string{"yesterday"}, "an-id"),
	} {
		got := apierrtest.Expect(t, http.MethodGet, e.listURL(query), http.StatusBadRequest, e.SignedIn)
		if got.Message != pagecursor.MsgInvalidCursor {
			t.Errorf("GET %s message = %q, want %q", query, got.Message, pagecursor.MsgInvalidCursor)
		}
	}

	if got := e.list("?view=" + statusSigned + "&order=" + orderNewest + "&cursor=" + cursor); len(got.Items) == 0 {
		t.Fatal("the cursor under its own view and order read nothing")
	}
}

// TestListContracts_ACursorFromAClientOrderCarriesItsName proves the
// client order's cursor decodes its name key.
func TestListContracts_ACursorFromAClientOrderCarriesItsName(t *testing.T) {
	e := newEnv(t, signedInAs(fixture.Owner()))
	seedBook(t, e)

	first := e.search(contracts.ListRequest{View: statusSigned, Order: orderClient})
	second := e.search(contracts.ListRequest{View: statusSigned, Order: orderClient, Cursor: *first.NextCursor})

	if len(second.Items) != 1 {
		t.Fatalf("second page = %d items, want the 31st", len(second.Items))
	}
}

// TestListContracts_RefusesWhatItDoesNotOffer proves the 400s: an unknown
// view or order, a bad Doula, and a search in the address.
func TestListContracts_RefusesWhatItDoesNotOffer(t *testing.T) {
	e := newEnv(t, signedInAs(fixture.Owner()))

	e.refusedList("?view=pending", msgBadView)
	e.refusedList("?view=not-signed", msgBadView)
	e.refusedList("?order=due", msgBadOrder)
	e.refusedList("?doula=not-a-uuid", msgBadDoula)
	e.refusedList("?search=quinn", msgSearchInGet)
	e.refusedList("?search=", msgSearchInGet)

	for _, req := range []contracts.ListRequest{{View: "pending"}, {Order: "due"}, {Doula: "nope"}} {
		reqtest.ExpectStatus(t, http.MethodPost, e.searchURL(), http.StatusBadRequest, e.SignedIn, reqtest.JSON(req))
	}
	reqtest.ExpectStatus(t, http.MethodPost, e.searchURL(), http.StatusBadRequest, e.SignedIn, reqtest.JSON(map[string]any{"mine": "yes"}))
}

// TestListContracts_SearchMatchesTheNamesAClientIsKnownBy proves the POST
// finds a Client by her preferred name and by her family name, trims the
// text, treats "%" as itself, and bounds it at 100 characters.
func TestListContracts_SearchMatchesTheNamesAClientIsKnownBy(t *testing.T) {
	e := newEnv(t, signedInAs(fixture.Owner()))
	quinn, quinnContract := e.seedListed(statusSent, listBase, fixture.NewClient(fixture.ClientName("Quincy")))
	testdb.NameClient(t, e.DB, quinn.Client.ID, "Quinn", "Bishop")
	_, other := e.seedListed(statusSent, listBase, fixture.NewClient(fixture.ClientName("Rowan")))

	for _, search := range []string{"quinn", "BISHOP", "  quinn bishop ", "Quincy"} {
		out := e.search(contracts.ListRequest{Search: search})
		if got := ids(out); !slices.Equal(got, []string{quinnContract}) || out.Total != 1 {
			t.Errorf("search %q = %v (total %d), want %s", search, got, out.Total, quinnContract)
		}
	}
	if out := e.search(contracts.ListRequest{Search: "%"}); len(out.Items) != 0 {
		t.Errorf("search %% = %v, want nothing: it is text, not a pattern", ids(out))
	}
	if out := e.search(contracts.ListRequest{Search: ""}); !slices.Contains(ids(out), other) || len(out.Items) != 2 {
		t.Errorf("empty search = %v, want both", ids(out))
	}
	if out := e.search(contracts.ListRequest{Search: strings.Repeat("é", 100)}); len(out.Items) != 0 {
		t.Errorf("100-character search = %v, want it read, and nothing found", ids(out))
	}

	got := apierrtest.Expect(t, http.MethodPost, e.searchURL(), http.StatusBadRequest, e.SignedIn,
		reqtest.JSON(contracts.ListRequest{Search: strings.Repeat("é", 101)}))
	if got.Message != msgSearchLong {
		t.Fatalf("message = %q, want %q", got.Message, msgSearchLong)
	}
}

// TestListContracts_MineAndDoulaNarrowToGrantedAttachments proves Mine
// narrows to the caller's own granted attachments and a Doula to hers, the
// two ANDed.
func TestListContracts_MineAndDoulaNarrowToGrantedAttachments(t *testing.T) {
	e := newEnv(t, signedInAs(fixture.Owner(), fixture.Doula()))
	priya := e.Colleague(fixture.Doula(), fixture.StaffName("Priya Raman"))
	_, own := e.seedListed(statusSent, listBase, fixture.WithDoula(e.Staff))
	_, hers := e.seedListed(statusSent, listBase, fixture.WithDoula(priya))
	_, both := e.seedListed(statusSent, listBase, fixture.WithDoula(e.Staff), fixture.WithDoula(priya))
	_, nobody := e.seedListed(statusSent, listBase)
	_, accrued := e.seedListed(statusSent, listBase)
	testdb.SeedAttachment(t, e.DB, mustEngagementOf(t, e, accrued), e.Staff.ID, "accrued", false)

	cases := []struct {
		name  string
		query string
		want  []string
	}{
		{"everyone", "", []string{own, hers, both, nobody, accrued}},
		{"mine", "?mine=true", []string{own, both}},
		{"mine=false is not mine", "?mine=false", []string{own, hers, both, nobody, accrued}},
		{"a doula", "?doula=" + priya.ID, []string{hers, both}},
		{"a doula, uppercased", "?doula=" + strings.ToUpper(priya.ID), []string{hers, both}},
		{"mine and a doula", "?mine=true&doula=" + priya.ID, []string{both}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			out := e.list(c.query)
			got := ids(out)
			slices.Sort(got)
			want := slices.Clone(c.want)
			slices.Sort(want)
			if !slices.Equal(got, want) || out.Total != len(want) {
				t.Fatalf("%s = %v (total %d), want %v", c.query, got, out.Total, want)
			}
		})
	}

	posted := e.search(contracts.ListRequest{Mine: true, Doula: priya.ID})
	if got := ids(posted); !slices.Equal(got, []string{both}) {
		t.Fatalf("POST mine+doula = %v, want %v", got, both)
	}
}

func mustEngagementOf(t *testing.T, e *env, contractID string) string {
	t.Helper()
	return testdb.Scalar[string](t, e.DB.Admin, `SELECT engagement_id FROM contracts WHERE id = $1`, contractID)
}

// TestListContracts_AnAmbientContractorSeesOnlyWhatSheIsAttachedTo proves a
// plain contractor Doula reads her granted Engagements alone, and that her
// rows carry neither the amount nor the Signed PDF: the JSON has no
// amountCents key at all.
func TestListContracts_AnAmbientContractorSeesOnlyWhatSheIsAttachedTo(t *testing.T) {
	e := newEnv(t, signedInAs(fixture.Doula(), fixture.Contractor()))
	_, attached := e.seedListed(statusSigned, listBase, fixture.WithDoula(e.Staff))
	_, unattached := e.seedListed(statusSigned, listBase)
	e.Exec(`UPDATE contracts SET signed_pdf_object_path = 'pdfs/a.pdf' WHERE id IN ($1, $2)`, attached, unattached)

	body := reqtest.ExpectBody(t, http.MethodGet, e.listURL("?view="+viewAll), http.StatusOK, e.SignedIn)
	out := reqtest.Expect[contracts.ListResponse](t, http.MethodGet, e.listURL("?view="+viewAll), http.StatusOK, e.SignedIn)

	if got := ids(out); !slices.Equal(got, []string{attached}) || out.Total != 1 {
		t.Fatalf("ids = %v (total %d), want only the attached %s", got, out.Total, attached)
	}
	if strings.Contains(string(body), "amountCents") {
		t.Fatalf("body = %s, want no amountCents key for a contractor", body)
	}
	if out.Items[0].HasSignedPDF || out.Items[0].AmountCents != nil {
		t.Fatalf("item = %+v, want the Signed PDF and the amount withheld", out.Items[0])
	}
	if post := e.search(contracts.ListRequest{View: viewAll}); len(post.Items) != 1 || post.Items[0].AmountCents != nil {
		t.Fatalf("POST = %+v, want the same narrowing and the same withholding", post)
	}
}

// TestListContracts_ReadersWhoReachThePracticeSeeTheMoney proves an
// Owner, an Admin, an employed Doula, and an Owner who is a contractor each
// read the whole Practice with the amount and the Signed PDF.
func TestListContracts_ReadersWhoReachThePracticeSeeTheMoney(t *testing.T) {
	for name, opts := range map[string][]fixture.StaffOption{
		"owner":               {fixture.Owner()},
		"admin":               {fixture.Admin()},
		caseEmployedDoula:     {fixture.Doula()},
		"owner on a contract": {fixture.Owner(), fixture.Contractor()},
	} {
		t.Run(name, func(t *testing.T) {
			e := newEnv(t, signedInAs(opts...))
			_, signed := e.seedListed(statusSigned, listBase)
			e.seedListed(statusSigned, listBase)
			e.Exec(`UPDATE contracts SET signed_pdf_object_path = 'pdfs/a.pdf' WHERE id = $1`, signed)

			out := e.list("?view=signed&order=oldest")

			if out.Total != 2 || len(out.Items) != 2 {
				t.Fatalf("items = %d (total %d), want the whole Practice", len(out.Items), out.Total)
			}
			withPDF := 0
			for _, item := range out.Items {
				if item.AmountCents == nil || *item.AmountCents != listAmount {
					t.Fatalf("amount = %v, want %d", item.AmountCents, listAmount)
				}
				if item.HasSignedPDF {
					withPDF++
				}
			}
			if withPDF != 1 {
				t.Fatalf("hasSignedPdf on %d rows, want 1", withPDF)
			}
		})
	}
}

// TestListContracts_DaysWaitingIsCountedInThePracticesDays proves a Sent
// Contract's wait is whole days of the Practice's zone: a minute across the
// Practice's midnight is a day, an hour on one side of it is none, and the
// day a step is dated is the Practice's, not UTC's.
func TestListContracts_DaysWaitingIsCountedInThePracticesDays(t *testing.T) {
	sentAt := time.Date(2026, 11, 10, 4, 30, 0, 0, time.UTC)
	e := newEnv(t, signedInAs(fixture.Owner()), atInstant(sentAt.Add(10*time.Minute)))
	e.Exec(`UPDATE practices SET timezone = 'America/New_York' WHERE id = $1`, e.Practice.ID)
	e.seedListed(statusSent, sentAt)

	today := e.list("")
	if len(today.Items) != 1 || today.Items[0].DaysWaiting == nil || *today.Items[0].DaysWaiting != 0 {
		t.Fatalf("items = %+v, want a wait of 0 days at 23:40 on the sent day", today.Items)
	}
	if today.Items[0].SteppedOn != "2026-11-09" {
		t.Fatalf("steppedOn = %q, want the Practice's day, 2026-11-09, not UTC's", today.Items[0].SteppedOn)
	}

	e.MoveClockTo(sentAt.Add(30 * time.Minute))
	next := reqtest.Expect[contracts.ListResponse](t, http.MethodGet, e.listURL(""), http.StatusOK, e.As(e.Staff))
	if got := *next.Items[0].DaysWaiting; got != 1 {
		t.Fatalf("daysWaiting = %d, want 1 half an hour after the Practice's midnight", got)
	}

	e.MoveClockTo(sentAt.Add(10 * 24 * time.Hour))
	later := reqtest.Expect[contracts.ListResponse](t, http.MethodGet, e.listURL(""), http.StatusOK, e.As(e.Staff))
	if got := *later.Items[0].DaysWaiting; got != 10 {
		t.Fatalf("daysWaiting = %d, want 10", got)
	}
}

// TestListContracts_OnlyASentContractIsWaiting proves daysWaiting is on a
// Sent row and on no other.
func TestListContracts_OnlyASentContractIsWaiting(t *testing.T) {
	e := newEnv(t, signedInAs(fixture.Owner()))
	e.seedEveryStatus()

	for _, item := range e.list("?view=" + viewAll).Items {
		if (item.DaysWaiting != nil) != (item.Status == statusSent) {
			t.Errorf("%s row daysWaiting = %v", item.Status, item.DaysWaiting)
		}
	}
	body := reqtest.ExpectBody(t, http.MethodGet, e.listURL("?view=draft"), http.StatusOK, e.SignedIn)
	if strings.Contains(string(body), "daysWaiting") {
		t.Fatalf("body = %s, want no daysWaiting on a Draft", body)
	}
}

// TestListContracts_SteppedOnIsTheDayOfTheCurrentStep proves each state's
// day: made for a Draft, sent for a Sent one, signed, voided.
func TestListContracts_SteppedOnIsTheDayOfTheCurrentStep(t *testing.T) {
	e := newEnv(t, signedInAs(fixture.Owner()))
	e.Exec(`UPDATE practices SET timezone = 'America/Los_Angeles' WHERE id = $1`, e.Practice.ID)
	steps := map[string]time.Time{
		statusDraft:  time.Date(2026, 10, 2, 20, 0, 0, 0, time.UTC),
		statusSent:   time.Date(2026, 10, 3, 3, 0, 0, 0, time.UTC),
		statusSigned: time.Date(2026, 10, 5, 18, 0, 0, 0, time.UTC),
		statusVoided: time.Date(2026, 10, 9, 1, 0, 0, 0, time.UTC),
	}
	for status, at := range steps {
		e.seedListed(status, at)
	}
	want := map[string]string{statusDraft: "2026-10-02", statusSent: "2026-10-02", statusSigned: "2026-10-05", statusVoided: "2026-10-08"}

	for _, item := range e.list("?view=" + viewAll).Items {
		if item.SteppedOn != want[item.Status] {
			t.Errorf("%s steppedOn = %q, want %q", item.Status, item.SteppedOn, want[item.Status])
		}
	}
}

// TestListContracts_NamesTheDoulasInOrderAndNoOne proves doulaNames lists
// every granted Doula by last name then first, and is absent for an
// Engagement with none or with only an ended one.
func TestListContracts_NamesTheDoulasInOrderAndNoOne(t *testing.T) {
	e := newEnv(t, signedInAs(fixture.Owner()))
	zed := e.Colleague(fixture.Doula(), fixture.StaffName("Amy Zed"))
	adams := e.Colleague(fixture.Doula(), fixture.StaffName("Zoe Adams"))
	mid := e.Colleague(fixture.Doula(), fixture.StaffName("Bea Mid"))
	left := e.Colleague(fixture.Doula(), fixture.StaffName("Cy Left"))
	_, several := e.seedListed(statusSent, listBase, fixture.WithDoula(zed), fixture.WithDoula(adams), fixture.WithDoula(mid), fixture.WithDoula(left))
	_, none := e.seedListed(statusSent, listBase.Add(time.Hour))
	e.Exec(`UPDATE engagement_attachments SET ended_at = now(), ended_by = $2, ended_reason = 'taken_off' WHERE staff_id = $1`, left.ID, e.Staff.ID)

	out := e.list("")
	byID := map[string]contracts.ContractListItem{}
	for _, item := range out.Items {
		byID[item.ContractID] = item
	}

	if got, want := byID[several].DoulaNames, []string{adams.Name, mid.Name, zed.Name}; !slices.Equal(got, want) {
		t.Fatalf("doulaNames = %v, want %v", got, want)
	}
	if byID[none].DoulaNames != nil {
		t.Fatalf("doulaNames = %v, want none", byID[none].DoulaNames)
	}
	body := reqtest.ExpectBody(t, http.MethodGet, e.listURL("?view=sent&order=newest"), http.StatusOK, e.SignedIn)
	if strings.Count(string(body), "doulaNames") != 1 {
		t.Fatalf("body = %s, want the field on one row and absent on the other", body)
	}
}

// TestListContracts_VoidRequestedIsSetForAnOpenRequestOnly proves the flag
// is true for a Contract with an open void request and false once the
// request is declined.
func TestListContracts_VoidRequestedIsSetForAnOpenRequestOnly(t *testing.T) {
	e := newEnv(t, signedInAs(fixture.Owner()))
	asking := e.Colleague(fixture.Doula())
	open, openContract := e.seedListed(statusSigned, listBase)
	declined, declinedContract := e.seedListed(statusSigned, listBase)
	_, plainContract := e.seedListed(statusSigned, listBase)
	seedVoidRequest(t, e.DB, open.ID, asking.ID, "wrong Client")
	requestID := seedVoidRequest(t, e.DB, declined.ID, asking.ID, "changed my mind")
	reqtest.ExpectStatus(t, http.MethodPost, e.contractURLOf(declined.ID, "/void-request/"+requestID+"/decline"), http.StatusOK, e.SignedIn, reasoned("no"))

	got := map[string]bool{}
	for _, item := range e.list("?view=signed").Items {
		got[item.ContractID] = item.VoidRequested
	}

	if !got[openContract] || got[declinedContract] || got[plainContract] {
		t.Fatalf("voidRequested = %v, want true for %s alone", got, openContract)
	}
}

// TestListContracts_DescribesTheEngagementAndWhoSentIt proves the row
// carries the Engagement's kind, due date and ending, the Client's name in
// full, and who sent the Contract.
func TestListContracts_DescribesTheEngagementAndWhoSentIt(t *testing.T) {
	e := newEnv(t, signedInAs(fixture.Owner()))
	sender := e.Colleague(fixture.Doula(), fixture.StaffName("Dee Whitlock"))
	engagement, contractID := e.seedListed(statusSent, listBase,
		fixture.Kind("postpartum"), fixture.DueDate("2026-12-04"), fixture.Status("completed"),
		fixture.NewClient(fixture.ClientName("Quincy")))
	testdb.NameClient(t, e.DB, engagement.Client.ID, "Quinn", "Bishop")
	e.DB.ExecCorrectingOutcome(t, `UPDATE engagements SET pregnancy_ended_on = '2026-10-30' WHERE id = $1`, engagement.ID)
	e.Exec(`UPDATE contracts SET sent_by = $2 WHERE id = $1`, contractID, sender.ID)
	_, draft := e.seedListed(statusDraft, listBase.Add(time.Hour))

	byID := map[string]contracts.ContractListItem{}
	for _, item := range e.list("").Items {
		byID[item.ContractID] = item
	}

	item := byID[contractID]
	if item.ClientName != "Quinn Bishop" || item.ClientID != engagement.Client.ID || item.EngagementID != engagement.ID || item.EngagementKind != "postpartum" {
		t.Fatalf("item = %+v, want the Client and Engagement named", item)
	}
	if item.DueDate != "2026-12-04" || item.PregnancyEndedOn != "2026-10-30" || item.BirthOutcome == "" || item.SentByName != "Dee Whitlock" {
		t.Fatalf("item = %+v, want the due date, the ending, the outcome and the sender", item)
	}
	if byID[draft].SentByName != "" || byID[draft].DueDate != "" || byID[draft].BirthOutcome != "" {
		t.Fatalf("draft = %+v, want nothing sent by anyone and no dates", byID[draft])
	}
}

// TestListContracts_ASenderlessContractListsWithoutOne proves a Contract whose
// sender is unknown, as a backfilled one can be, lists with no name.
func TestListContracts_ASenderlessContractListsWithoutOne(t *testing.T) {
	e := newEnv(t, signedInAs(fixture.Owner()))
	_, contractID := e.seedListed(statusSent, listBase)

	out := e.list("?view=sent")

	if len(out.Items) != 1 || out.Items[0].ContractID != contractID || out.Items[0].SentByName != "" {
		t.Fatalf("items = %+v, want the row with no sender", out.Items)
	}
}

// TestListContracts_ABadStoredZoneIsAnErrorNotUTC proves a Practice whose
// stored zone will not load gets a 500 rather than the wrong day.
func TestListContracts_ABadStoredZoneIsAnErrorNotUTC(t *testing.T) {
	e := newEnv(t, signedInAs(fixture.Owner()))
	e.Exec(`UPDATE practices SET timezone = 'Nowhere/Bad' WHERE id = $1`, e.Practice.ID)

	reqtest.ExpectStatus(t, http.MethodGet, e.listURL(""), http.StatusInternalServerError, e.SignedIn)
}

// TestListContracts_NotSignedHoldsTheSentLongestWaitFirstThenTheDrafts
// proves the default view's own order: every Sent Contract by the day it
// was sent, then every Draft by the day it was made, whatever the dates, and
// that asking for oldest in that view is the same order.
func TestListContracts_NotSignedHoldsTheSentLongestWaitFirstThenTheDrafts(t *testing.T) {
	e := newEnv(t, signedInAs(fixture.Owner()))
	_, oldDraft := e.seedListed(statusDraft, listBase)
	_, lateSent := e.seedListed(statusSent, listBase.Add(48*time.Hour))
	_, earlySent := e.seedListed(statusSent, listBase.Add(24*time.Hour))
	_, newDraft := e.seedListed(statusDraft, listBase.Add(72*time.Hour))
	want := []string{earlySent, lateSent, oldDraft, newDraft}

	for _, query := range []string{"", "?order=oldest", "?view=&order="} {
		if got := ids(e.list(query)); !slices.Equal(got, want) {
			t.Errorf("GET %q = %v, want %v", query, got, want)
		}
	}
	if got := ids(e.list("?order=newest")); !slices.Equal(got, []string{newDraft, lateSent, earlySent, oldDraft}) {
		t.Errorf("order=newest = %v, want newest first with no Drafts-last rule", got)
	}
	if got := ids(e.list("?view=draft")); !slices.Equal(got, []string{oldDraft, newDraft}) {
		t.Errorf("view=draft = %v, want the Drafts oldest first", got)
	}
}

// TestListContracts_NotSignedCursorKeepsBothKeys proves the default view's
// cursor carries the Draft flag and the day, and refuses one that does not.
func TestListContracts_NotSignedCursorKeepsBothKeys(t *testing.T) {
	e := newEnv(t, signedInAs(fixture.Owner()))
	seedBook(t, e)
	first := e.list("")
	if first.NextCursor == nil {
		t.Fatal("first page carries no cursor")
	}
	if got := e.list("?cursor=" + *first.NextCursor); len(got.Items) != listPageSize || got.Items[0].Status != statusSent {
		t.Fatalf("second page = %d items starting %q, want 30 starting on the last Sent", len(got.Items), got.Items[0].Status)
	}

	stamp := listBase.Format(time.RFC3339Nano)
	for _, cursor := range []string{
		pagecursor.EncodeKeyed(":", []string{stamp}, "an-id"),
		pagecursor.EncodeKeyed(":", []string{"maybe", stamp}, "an-id"),
		pagecursor.EncodeKeyed(":", []string{"true", "yesterday"}, "an-id"),
		*first.NextCursor + "x",
	} {
		got := apierrtest.Expect(t, http.MethodGet, e.listURL("?cursor="+cursor), http.StatusBadRequest, e.SignedIn)
		if got.Message != pagecursor.MsgInvalidCursor {
			t.Errorf("cursor %q message = %q, want %q", cursor, got.Message, pagecursor.MsgInvalidCursor)
		}
	}
	other := apierrtest.Expect(t, http.MethodGet, e.listURL("?order=oldest&cursor="+*first.NextCursor), http.StatusBadRequest, e.SignedIn)
	if other.Message != pagecursor.MsgInvalidCursor {
		t.Errorf("a cursor from the view's own order under order=oldest message = %q, want %q", other.Message, pagecursor.MsgInvalidCursor)
	}
}

// TestListContracts_NotSignedOrderHoldsForAContractor proves a contractor's
// narrowing and the Drafts-last order together.
func TestListContracts_NotSignedOrderHoldsForAContractor(t *testing.T) {
	e := newEnv(t, signedInAs(fixture.Doula(), fixture.Contractor()))
	_, draft := e.seedListed(statusDraft, listBase, fixture.WithDoula(e.Staff))
	_, sent := e.seedListed(statusSent, listBase.Add(24*time.Hour), fixture.WithDoula(e.Staff))
	e.seedListed(statusSent, listBase)

	out := e.list("")
	if got := ids(out); !slices.Equal(got, []string{sent, draft}) || out.Total != 2 {
		t.Fatalf("ids = %v (total %d), want [%s %s]", got, out.Total, sent, draft)
	}
}
