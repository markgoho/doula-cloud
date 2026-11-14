package payments_test

import (
	"net/http"
	"reflect"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"

	"doula-cloud/api/internal/pagecursor"
	"doula-cloud/api/internal/payments"
	"doula-cloud/api/internal/reqtest"
	"doula-cloud/api/internal/testdb"
)

// seededInvoice is one Invoice the order tests seeded, with the three keys
// the list can be ordered by.
type seededInvoice struct {
	id     string
	status string
	sent   time.Time
	due    time.Time
	amount int64
	isOpen bool
}

// seedOrderBook seeds 40 Invoices whose sent, due and amount values each
// repeat across runs of rows, so every order meets ties that fall inside a
// page of 30 and across the boundary of the first. Every fifth Invoice is
// paid, which the whole book holds and Unpaid and Overdue leave out.
func (e *env) seedOrderBook() []seededInvoice {
	e.T.Helper()
	_, contractID := e.contractWithStatus("draft")
	now := time.Now().UTC().Truncate(time.Microsecond)
	var book []seededInvoice
	for i := range 40 {
		status := invoiceStatusOpen
		if i%5 == 4 {
			status = invoiceStatusPaid
		}
		row := seededInvoice{
			status: status,
			sent:   now.Add(-time.Duration(i/4) * 24 * time.Hour),
			due:    now.Add(-60*24*time.Hour + time.Duration((i*7)%9/3)*24*time.Hour),
			amount: int64(1000 + (i*5)%7*250),
			isOpen: status == invoiceStatusOpen,
		}
		row.id = seedInvoiceDue(e.T, e.DB, e.Practice.ID, contractID, "in_order_"+strconv.Itoa(i), status, row.amount, row.sent, row.due)
		book = append(book, row)
	}
	return book
}

// orderedBy sorts the seeded rows a view holds the way an order says, ties
// by id in the key's direction.
func orderedBy(book []seededInvoice, view, order string) []string {
	var rows []seededInvoice
	for _, row := range book {
		if view == viewWhole || row.isOpen {
			rows = append(rows, row)
		}
	}
	desc := strings.HasSuffix(order, "-newest") || strings.HasSuffix(order, "-latest") || strings.HasSuffix(order, "-largest")
	compare := func(a, b seededInvoice) int {
		switch {
		case strings.HasPrefix(order, "sent"):
			return a.sent.Compare(b.sent)
		case strings.HasPrefix(order, "due"):
			return a.due.Compare(b.due)
		}
		return int(a.amount - b.amount)
	}
	slices.SortFunc(rows, func(a, b seededInvoice) int {
		c := compare(a, b)
		if c == 0 {
			c = strings.Compare(a.id, b.id)
		}
		if desc {
			return -c
		}
		return c
	})
	ids := make([]string, len(rows))
	for i, row := range rows {
		ids[i] = row.id
	}
	return ids
}

// The views the list reads, by the name the POST carries and the query the
// GET carries.
const (
	viewWhole   = "all"
	viewOverdue = "overdue"
	orderLarge  = "amount-largest"
	orderOldest = "sent-oldest"
)

var viewQueries = map[string]string{viewWhole: "", unpaidView: unpaidView + "=true", viewOverdue: viewOverdue + "=true"}

// TestGetPracticeInvoicesHandler_EveryOrderPagesWithoutSkipOrRepeat walks
// each order across the page boundary on each view, through the GET and the
// POST, and requires the rows in the order's own sequence: sorted by the
// key, then by id in the key's direction, with the first page full so a run
// of ties straddles the cursor.
func TestGetPracticeInvoicesHandler_EveryOrderPagesWithoutSkipOrRepeat(t *testing.T) {
	e := newEnv(t)
	book := e.seedOrderBook()
	orders := []string{"sent-newest", orderOldest, "due-earliest", "due-latest", orderLarge, "amount-smallest"}

	for view, query := range viewQueries {
		for _, order := range orders {
			t.Run(view+"/"+order, func(t *testing.T) {
				want := orderedBy(book, view, order)
				var narrowing []string
				if query != "" {
					narrowing = append(narrowing, query)
				}
				got := rowIDs(e.walkPracticeInvoices(strings.Join(append(narrowing, "order="+order), "&")))
				if !slices.Equal(got, want) {
					t.Fatalf("GET rows = %v, want %v", got, want)
				}
				posted := rowIDs(e.walkSearch(payments.PracticeInvoicesListRequest{View: view, Order: order}))
				if !slices.Equal(posted, want) {
					t.Fatalf("POST rows = %v, want %v", posted, want)
				}
			})
		}
	}
}

// TestGetPracticeInvoicesHandler_AnEmptyOrderIsTheViewsOwn proves no order
// is today's behavior: sent newest first on the whole book, payment due
// earliest first on Unpaid and Overdue, whether ?order= is absent or empty.
func TestGetPracticeInvoicesHandler_AnEmptyOrderIsTheViewsOwn(t *testing.T) {
	e := newEnv(t)
	book := e.seedOrderBook()
	for view, query := range viewQueries {
		t.Run(view, func(t *testing.T) {
			own := "sent-newest"
			if view != viewWhole {
				own = "due-earliest"
			}
			want := orderedBy(book, view, own)
			if got := rowIDs(e.walkPracticeInvoices(query)); !slices.Equal(got, want) {
				t.Fatalf("no order = %v, want %v", got, want)
			}
			if got := rowIDs(e.walkPracticeInvoices(query + "&order=")); !slices.Equal(got, want) {
				t.Fatalf("empty order = %v, want %v", got, want)
			}
		})
	}
}

// TestGetPracticeInvoicesHandler_TotalsDoNotDependOnTheOrder proves the
// whole book's figures read the same whichever order a page is in.
func TestGetPracticeInvoicesHandler_TotalsDoNotDependOnTheOrder(t *testing.T) {
	e := newEnv(t)
	e.seedOrderBook()
	want := e.practiceInvoices()
	want.Items, want.NextCursor, want.ClientsCanPay = nil, nil, false
	for _, order := range []string{orderOldest, "due-latest", "amount-largest"} {
		got := e.practiceInvoices("order=" + order)
		got.Items, got.NextCursor, got.ClientsCanPay = nil, nil, false
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("totals under %s = %+v, want %+v", order, got, want)
		}
	}
}

// TestGetPracticeInvoicesHandler_AnUnknownOrderReturns400 proves a name the
// list does not offer is refused on both entry points, never read as the
// default.
func TestGetPracticeInvoicesHandler_AnUnknownOrderReturns400(t *testing.T) {
	e := newEnv(t)
	for _, order := range []string{"newest", "SENT-NEWEST", "amounts", "sent-newest,due-latest", "latest"} {
		t.Run(order, func(t *testing.T) {
			e.expect(http.MethodGet, practiceInvoicesPath("order="+order), http.StatusBadRequest)
			refusal := e.postRefused("/invoices/list", http.StatusBadRequest, reqtest.JSON(payments.PracticeInvoicesListRequest{Order: order}))
			if refusal.Message == "" || refusal.Message[0] < 'A' || refusal.Message[0] > 'Z' {
				t.Fatalf("message = %q, want a capitalized sentence", refusal.Message)
			}
		})
	}
}

// TestGetPracticeInvoicesHandler_ACursorIsBoundToItsViewAndOrder proves a
// cursor minted for one view or one order is a 400 under another, including
// the empty order against the order it stands for, since a cursor is the
// one thing that would otherwise page the wrong rows.
func TestGetPracticeInvoicesHandler_ACursorIsBoundToItsViewAndOrder(t *testing.T) {
	e := newEnv(t)
	e.seedOrderBook()
	cursorOf := func(query ...string) string {
		page := e.practiceInvoices(query...)
		if page.NextCursor == nil {
			t.Fatalf("%v: no next cursor", query)
		}
		return *page.NextCursor
	}
	byAmount := cursorOf("order=" + orderLarge)
	byDefault := cursorOf()
	byExplicit := cursorOf("order=sent-newest")
	unpaidByDue := cursorOf("unpaid=true")

	e.expect(http.MethodGet, practiceInvoicesPath("order=amount-smallest", "cursor="+byAmount), http.StatusBadRequest)
	e.expect(http.MethodGet, practiceInvoicesPath("cursor="+byAmount), http.StatusBadRequest)
	e.expect(http.MethodGet, practiceInvoicesPath("unpaid=true", "order="+orderLarge, "cursor="+byAmount), http.StatusBadRequest)
	e.expect(http.MethodGet, practiceInvoicesPath("order=sent-newest", "cursor="+byDefault), http.StatusBadRequest)
	e.expect(http.MethodGet, practiceInvoicesPath("cursor="+byExplicit), http.StatusBadRequest)
	e.expect(http.MethodGet, practiceInvoicesPath("overdue=true", "cursor="+unpaidByDue), http.StatusBadRequest)
	e.expect(http.MethodGet, practiceInvoicesPath("cursor="+unpaidByDue), http.StatusBadRequest)

	e.postRefused("/invoices/list", http.StatusBadRequest, reqtest.JSON(payments.PracticeInvoicesListRequest{Order: "amount-smallest", Cursor: byAmount}))
	refusal := e.postRefused("/invoices/list", http.StatusBadRequest, reqtest.JSON(payments.PracticeInvoicesListRequest{View: "unpaid", Cursor: byDefault}))
	if refusal.Message != "invalid cursor" {
		t.Errorf("a cursor of another view is refused in the words the other lists use, got %q", refusal.Message)
	}
}

// TestGetPracticeInvoicesHandler_AMalformedKeyedCursorReturns400 proves a
// well-formed envelope whose key cannot be read under the order is refused.
func TestGetPracticeInvoicesHandler_AMalformedKeyedCursorReturns400(t *testing.T) {
	e := newEnv(t)
	for name, cursor := range map[string]string{
		"no keys":             pagecursor.EncodeKeyed("all:amount-largest", nil, "id"),
		"amount not a number": pagecursor.EncodeKeyed("all:amount-largest", []string{"lots"}, "id"),
		"time not a time":     pagecursor.EncodeKeyed("all:sent-oldest", []string{"soon"}, "id"),
	} {
		t.Run(name, func(*testing.T) {
			order := "amount-largest"
			if strings.Contains(name, "time") {
				order = orderOldest
			}
			e.expect(http.MethodGet, practiceInvoicesPath("order="+order, "cursor="+cursor), http.StatusBadRequest)
		})
	}
}

// TestPracticeInvoices_NoSortKeyIsNull records why no order has a rule for
// where missing values sort: sent, payment due and amount are NOT NULL, so
// every row has all three.
func TestPracticeInvoices_NoSortKeyIsNull(t *testing.T) {
	e := newEnv(t)
	for _, column := range []string{"created_at", "due_at", "amount_cents"} {
		nullable := testdb.Scalar[string](t, e.DB.Admin,
			`SELECT is_nullable FROM information_schema.columns WHERE table_name = 'invoices' AND column_name = $1`, column)
		if nullable != "NO" {
			t.Fatalf("invoices.%s is nullable; the orders would need a place for a missing value", column)
		}
	}
}
