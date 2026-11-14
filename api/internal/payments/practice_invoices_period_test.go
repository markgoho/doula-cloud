package payments_test

import (
	"net/http"
	"strconv"
	"testing"
	"time"

	"doula-cloud/api/internal/apierr"
	"doula-cloud/api/internal/payments"
	"doula-cloud/api/internal/reqtest"
)

const (
	sentFromOct25 = "sentFrom=2026-10-25"
	sentToOct25   = "sentTo=2026-10-25"
)

// seedPeriodEdges seeds one open Invoice on each side of both edges of
// October 25 in the Practice's zone, New York, which is four hours behind
// UTC on that day: the last second of October 24, the first and last
// seconds of October 25, and the first second of October 26. The amounts
// are distinct powers of two so a total names the Invoices it holds.
func (e *env) seedPeriodEdges() {
	e.T.Helper()
	_, contractID := e.contractWithStatus(invoiceStatusDraft)
	for i, at := range []time.Time{
		time.Date(2026, 10, 25, 3, 59, 59, 0, time.UTC),
		time.Date(2026, 10, 25, 4, 0, 0, 0, time.UTC),
		time.Date(2026, 10, 26, 3, 59, 59, 0, time.UTC),
		time.Date(2026, 10, 26, 4, 0, 0, 0, time.UTC),
	} {
		seedInvoice(e.T, e.DB, e.Practice.ID, contractID, "in_edge_"+strconv.Itoa(i), invoiceStatusOpen, int64(100<<i), at)
	}
}

func (e *env) searchPeriod(req payments.PracticeInvoicesListRequest) payments.PracticeInvoicesResponse {
	e.T.Helper()
	return e.searchInvoices(req)
}

// TestPracticeInvoices_PeriodIsTheSentDayInThePracticesZone proves the
// period's edges are the Practice's midnights, not UTC's: an Invoice sent
// at 11:59:59pm in New York on October 24 is outside October 25 and one
// sent at midnight is inside, and the same at the far edge. Both entry
// points read it alike.
func TestPracticeInvoices_PeriodIsTheSentDayInThePracticesZone(t *testing.T) {
	e := newEnv(t)
	e.seedPeriodEdges()

	get := e.practiceInvoices(sentFromOct25, sentToOct25)
	post := e.searchPeriod(payments.PracticeInvoicesListRequest{SentFrom: dayOct25, SentTo: dayOct25})

	for name, out := range map[string]payments.PracticeInvoicesResponse{"GET": get, "POST": post} {
		if len(out.Items) != 2 || out.Items[0].AmountCents != 400 || out.Items[1].AmountCents != 200 {
			t.Fatalf("%s items = %+v, want the Invoices of 400 and 200 cents, newest first", name, out.Items)
		}
		if out.Items[0].RaisedOn != dayOct25 || out.Items[1].RaisedOn != dayOct25 {
			t.Fatalf("%s raisedOn = %s, %s, want %s", name, out.Items[0].RaisedOn, out.Items[1].RaisedOn, dayOct25)
		}
	}
}

// TestPracticeInvoices_PeriodSpansBothDaysIncluded proves To is a day that
// is included, so a two-day period holds the Invoices of both.
func TestPracticeInvoices_PeriodSpansBothDaysIncluded(t *testing.T) {
	e := newEnv(t)
	e.seedPeriodEdges()

	out := e.practiceInvoices("sentFrom=2026-10-24", "sentTo=2026-10-25")
	if len(out.Items) != 3 {
		t.Fatalf("items = %d, want the three sent on October 24 and 25", len(out.Items))
	}
}

// TestPracticeInvoices_TotalsFollowThePeriodAndIgnoreViewAndSearch proves
// the four totals count only the Invoices sent in the period, so the rows
// and the totals cannot disagree, and that they still ignore the view and
// the search.
func TestPracticeInvoices_TotalsFollowThePeriodAndIgnoreViewAndSearch(t *testing.T) {
	e := newEnv(t)
	e.seedPeriodEdges()

	whole := e.practiceInvoices()
	if whole.InvoiceCount != 4 || whole.OutstandingCents != 1500 || whole.OutstandingCount != 4 {
		t.Fatalf("without a period: count %d, outstanding %d over %d, want 4, 1500 over 4", whole.InvoiceCount, whole.OutstandingCents, whole.OutstandingCount)
	}

	for name, out := range map[string]payments.PracticeInvoicesResponse{
		"the whole book":  e.practiceInvoices(sentFromOct25, sentToOct25),
		"the unpaid view": e.practiceInvoices(sentFromOct25, sentToOct25, "unpaid=true"),
		"a search":        e.searchPeriod(payments.PracticeInvoicesListRequest{View: unpaidView, Search: "in_edge_1", SentFrom: dayOct25, SentTo: dayOct25}),
	} {
		if out.InvoiceCount != 2 || out.OutstandingCents != 600 || out.OutstandingCount != 2 {
			t.Fatalf("%s: count %d, outstanding %d over %d, want 2, 600 over 2", name, out.InvoiceCount, out.OutstandingCents, out.OutstandingCount)
		}
	}
}

// TestPracticeInvoices_BilledCountsWhatWasSentAndStands proves billed is the
// Invoices sent that still stand as a bill: open, paid (refunded or not) and
// written off count, a draft and a void do not, and the period is the
// Practice's New York day at both edges of the month.
func TestPracticeInvoices_BilledCountsWhatWasSentAndStands(t *testing.T) {
	e := newEnv(t)
	_, contractID := e.contractWithStatus(invoiceStatusDraft)
	inOctober := time.Date(2026, 10, 15, 16, 0, 0, 0, time.UTC)
	for i, in := range []struct {
		status string
		cents  int64
		at     time.Time
	}{
		{invoiceStatusOpen, 1, inOctober},
		{invoiceStatusPaid, 2, inOctober},
		{invoiceStatusUncollectible, 4, inOctober},
		{invoiceStatusVoid, 8, inOctober},
		{invoiceStatusDraft, 16, inOctober},
		{invoiceStatusOpen, 32, time.Date(2026, 11, 1, 3, 59, 59, 0, time.UTC)},
		{invoiceStatusOpen, 64, time.Date(2026, 10, 1, 4, 0, 0, 0, time.UTC)},
		{invoiceStatusOpen, 128, time.Date(2026, 10, 1, 3, 59, 59, 0, time.UTC)},
		{invoiceStatusOpen, 256, time.Date(2026, 11, 1, 4, 0, 0, 0, time.UTC)},
	} {
		seedInvoice(e.T, e.DB, e.Practice.ID, contractID, "in_billed_"+strconv.Itoa(i), in.status, in.cents, in.at)
	}

	october := e.practiceInvoices("sentFrom=2026-10-01", "sentTo=2026-10-31")
	if october.BilledCents != 1+2+4+32+64 || october.BilledCount != 5 || october.InvoiceCount != 7 {
		t.Fatalf("October: billed %d over %d, count %d, want 103 over 5, count 7", october.BilledCents, october.BilledCount, october.InvoiceCount)
	}
	unpaid := e.practiceInvoices("sentFrom=2026-10-01", "sentTo=2026-10-31", "unpaid=true")
	if unpaid.BilledCents != october.BilledCents || unpaid.BilledCount != october.BilledCount {
		t.Fatalf("unpaid view: billed %d over %d, want the same as the whole book's", unpaid.BilledCents, unpaid.BilledCount)
	}
	whole := e.practiceInvoices()
	if whole.BilledCents != 1+2+4+32+64+128+256 || whole.BilledCount != 7 || whole.InvoiceCount != 9 {
		t.Fatalf("no period: billed %d over %d, count %d, want 487 over 7, count 9", whole.BilledCents, whole.BilledCount, whole.InvoiceCount)
	}
}

// TestPracticeInvoices_RefundedTotalFollowsThePeriod proves the money that
// went back counts under the period its Invoice was sent in.
func TestPracticeInvoices_RefundedTotalFollowsThePeriod(t *testing.T) {
	e := newEnv(t)
	_, invoiceID, paymentID := e.paidByHandInvoice()
	if _, err := e.DB.Admin.ExecContext(t.Context(), `UPDATE invoices SET created_at = $2 WHERE id = $1`,
		invoiceID, time.Date(2026, 10, 25, 16, 0, 0, 0, time.UTC)); err != nil {
		t.Fatalf("move the Invoice's sent instant: %v", err)
	}
	e.refund(invoiceID, paymentID, checkRefund(seedByHandInvoiceAmountCents))

	inside := e.practiceInvoices(sentFromOct25, sentToOct25)
	outside := e.practiceInvoices("sentFrom=2026-10-26", "sentTo=2026-10-26")
	if inside.RefundedCents != seedByHandInvoiceAmountCents || inside.PaidCents != seedByHandInvoiceAmountCents || inside.BilledCents != seedByHandInvoiceAmountCents || inside.BilledCount != 1 {
		t.Fatalf("inside: refunded %d, paid %d, want %d each", inside.RefundedCents, inside.PaidCents, seedByHandInvoiceAmountCents)
	}
	if outside.RefundedCents != 0 || outside.PaidCents != 0 || outside.InvoiceCount != 0 || outside.BilledCents != 0 || outside.BilledCount != 0 {
		t.Fatalf("outside: refunded %d, paid %d, count %d, want none", outside.RefundedCents, outside.PaidCents, outside.InvoiceCount)
	}
}

// TestPracticeInvoices_PeriodCarriesAcrossPages proves a cursor walks the
// whole period and nothing outside it, with the totals the same on every
// page.
func TestPracticeInvoices_PeriodCarriesAcrossPages(t *testing.T) {
	e := newEnv(t)
	_, contractID := e.contractWithStatus(invoiceStatusDraft)
	start := time.Date(2026, 10, 25, 4, 0, 0, 0, time.UTC)
	for i := range 35 {
		seedInvoice(t, e.DB, e.Practice.ID, contractID, "in_in_"+strconv.Itoa(i), invoiceStatusOpen, 1000, start.Add(time.Duration(i)*time.Minute))
	}
	for i := range 3 {
		seedInvoice(t, e.DB, e.Practice.ID, contractID, "in_out_"+strconv.Itoa(i), invoiceStatusOpen, 1000, start.Add(-time.Duration(i+1)*time.Hour))
	}

	req := payments.PracticeInvoicesListRequest{SentFrom: dayOct25, SentTo: dayOct25}
	first := e.searchPeriod(req)
	if len(first.Items) != 30 || !first.HasMore || first.InvoiceCount != 35 {
		t.Fatalf("first page = %d rows, hasMore %v, count %d, want 30, true, 35", len(first.Items), first.HasMore, first.InvoiceCount)
	}
	req.Cursor = *first.NextCursor
	second := e.searchPeriod(req)
	if len(second.Items) != 5 || second.HasMore || second.InvoiceCount != 35 || second.OutstandingCents != 35000 {
		t.Fatalf("second page = %d rows, hasMore %v, count %d, outstanding %d, want 5, false, 35, 35000", len(second.Items), second.HasMore, second.InvoiceCount, second.OutstandingCents)
	}

	getFirst := e.practiceInvoices(sentFromOct25, sentToOct25)
	getSecond := e.practiceInvoices(sentFromOct25, sentToOct25, "cursor="+*getFirst.NextCursor)
	if len(getFirst.Items) != 30 || len(getSecond.Items) != 5 {
		t.Fatalf("GET pages = %d, %d rows, want 30 and 5", len(getFirst.Items), len(getSecond.Items))
	}
}

var badPeriods = []struct {
	name, from, to, message string
}{
	{"a From that is not a day", "10/25/2026", dayOct25, "Use a date like 2026-10-31 for sentFrom."},
	{"a To that is not a day", dayOct25, "tomorrow", "Use a date like 2026-10-31 for sentTo."},
	{"a day that does not exist", "2026-02-30", dayOct25, "Use a date like 2026-10-31 for sentFrom."},
	{"a From with no To", dayOct25, "", "Send both sentFrom and sentTo, or neither."},
	{"a To with no From", "", dayOct25, "Send both sentFrom and sentTo, or neither."},
	{"a To before the From", "2026-10-26", dayOct25, "Use a sentTo that is not before sentFrom."},
}

// TestPracticeInvoices_RefusesAMalformedPeriod proves a day that is not a
// day, one end without the other, and a To before the From are a 400 with
// the message the caller reads, on both entry points.
func TestPracticeInvoices_RefusesAMalformedPeriod(t *testing.T) {
	for _, tc := range badPeriods {
		t.Run(tc.name, func(t *testing.T) {
			e := newEnv(t)
			var query []string
			if tc.from != "" {
				query = append(query, "sentFrom="+tc.from)
			}
			if tc.to != "" {
				query = append(query, "sentTo="+tc.to)
			}
			got := reqtest.Expect[apierr.APIError](t, http.MethodGet, e.PracticeURL(practiceInvoicesPath(query...)), http.StatusBadRequest, e.SignedIn)
			if got.Message != tc.message {
				t.Fatalf("GET message = %q, want %q", got.Message, tc.message)
			}
			got = e.postRefused("/invoices/list", http.StatusBadRequest, reqtest.JSON(payments.PracticeInvoicesListRequest{SentFrom: tc.from, SentTo: tc.to}))
			if got.Message != tc.message {
				t.Fatalf("POST message = %q, want %q", got.Message, tc.message)
			}
		})
	}
}
