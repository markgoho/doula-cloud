package payments

import (
	"bytes"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"doula-cloud/api/internal/activity"
	"doula-cloud/api/internal/apierr"
	"doula-cloud/api/internal/csvexport"
	"doula-cloud/api/internal/csvsafe"
	"doula-cloud/api/internal/staffauth"
)

// invoicesExportHeader is every column the Invoices export writes, in
// order (#2871). It names no clinical field and no way to reach a Client:
// the Client's name and money, nothing else.
var invoicesExportHeader = []string{
	"Client", "Invoice", "Care", "Billed", "Amount", "Currency", "State",
	"Days overdue", "Sent", "Payment due", "Paid", "Refunded",
}

// PracticeInvoicesExportRequest is PracticeInvoicesListRequest without a
// cursor: the export is every matching row, never a page.
type PracticeInvoicesExportRequest struct {
	View     string `json:"view"`
	Search   string `json:"search"`
	Order    string `json:"order"`
	SentFrom string `json:"sentFrom"`
	SentTo   string `json:"sentTo"`
}

// invoicesExportScope is the diff of the one Activity row a download
// writes: which slice of the book left the product, and how big it was.
// Whether a search was applied is kept, never its text, which can be a
// Client's name that an Activity row would outlive her Erasure with.
type invoicesExportScope struct {
	View     string `json:"view"`
	SentFrom string `json:"sentFrom,omitempty"`
	SentTo   string `json:"sentTo,omitempty"`
	Order    string `json:"order,omitempty"`
	Rows     int    `json:"rows"`
	Searched bool   `json:"searched"`
}

// ExportPracticeInvoicesHandler answers a POST of the Invoices list's own
// view, search, order and period with every Invoice they match as a CSV
// (#2871). The rows, the order and the Practice's zone are the list's own,
// so what leaves is what the screen showed. It is a POST so a search, which
// can be a Client's name, stays out of the URL (#1829).
//
// The rate limit, the Activity row invoices_exported and the response are
// csvexport's. Must be mounted behind staffauth.Middleware, for the Owner and
// Admin roles.
func ExportPracticeInvoicesHandler() http.Handler {
	spec := csvexport.Spec{Limit: "invoices_export", Action: activity.ActionInvoicesExported, File: "invoices"}
	return csvexport.Handler(spec, func(w http.ResponseWriter, r *http.Request, c staffauth.Call, env csvexport.Env) (csvexport.File, bool) {
		var req PracticeInvoicesExportRequest
		if !apierr.DecodeJSON(w, r, &req) {
			return csvexport.File{}, false
		}
		q, ok := newBodyInvoiceListQuery(w, PracticeInvoicesListRequest{
			View: req.View, Search: req.Search, Order: req.Order, SentFrom: req.SentFrom, SentTo: req.SentTo,
		})
		if !ok {
			return csvexport.File{}, false
		}
		items, err := readPracticeInvoices(r.Context(), c.Tx, c.PracticeID, q, bookDays{zone: env.Zone, now: env.Now}, 0)
		if err != nil {
			// coverage:ignore reason: DB query failure, not exercised by unit tests
			apierr.WriteError(w, apierr.MsgInternalError, http.StatusInternalServerError)
			return csvexport.File{}, false
		}
		return csvexport.File{
			Body: writeInvoicesCSV(items),
			Diff: invoicesExportScope{
				View: q.narrowing.name(), SentFrom: q.period.sentFrom, SentTo: q.period.sentTo,
				Order: req.Order, Rows: len(items), Searched: q.search != "",
			},
			Headers: map[string]string{"Invoice-Count": strconv.Itoa(len(items))},
		}, true
	})
}

// writeInvoicesCSV is the header and one row per Invoice, every cell
// through csvsafe. Writing to a bytes.Buffer cannot fail, so the writer's
// errors are not read.
func writeInvoicesCSV(items []PracticeInvoiceView) []byte {
	var body bytes.Buffer
	cw := csvsafe.NewWriter(&body)
	_ = cw.Write(invoicesExportHeader)
	for _, it := range items {
		paid := ""
		if it.PaidOn != nil {
			paid = *it.PaidOn
		}
		_ = cw.Write([]string{
			it.ClientName, it.Reference, exportCare(it.EngagementKind), exportBilled(it.BillingMode),
			exportAmount(it.AmountCents), strings.ToUpper(it.Currency), exportState(it),
			strconv.Itoa(it.DaysOverdue), it.RaisedOn, it.DueOn, paid, exportAmount(it.RefundedCents),
		})
	}
	cw.Flush()
	return body.Bytes()
}

func exportAmount(cents int64) string {
	return fmt.Sprintf("%d.%02d", cents/100, cents%100)
}

func exportCare(kind string) string {
	if kind == "postpartum" {
		return "Postpartum"
	}
	return "Birth"
}

func exportBilled(mode string) string {
	if mode == string(BillingModeStripe) {
		return "Stripe"
	}
	return "By hand"
}

func exportState(it PracticeInvoiceView) string {
	switch {
	case it.PaymentProcessing:
		return "payment on its way"
	case it.DaysOverdue > 0:
		return "overdue"
	}
	switch it.Status {
	case "open":
		return "owed"
	case "uncollectible":
		return "written off"
	}
	return it.Status
}
