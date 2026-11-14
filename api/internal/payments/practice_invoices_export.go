package payments

import (
	"bytes"
	"encoding/json/v2"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"doula-cloud/api/internal/activity"
	"doula-cloud/api/internal/apierr"
	"doula-cloud/api/internal/clock"
	"doula-cloud/api/internal/csvsafe"
	"doula-cloud/api/internal/practicetimezone"
	"doula-cloud/api/internal/ratelimit"
	"doula-cloud/api/internal/staffauth"
)

// invoicesExportHeader is every column the Invoices export writes, in
// order (#2871). It names no clinical field and no way to reach a Client:
// the Client's name and money, nothing else.
var invoicesExportHeader = []string{
	"Client", "Invoice", "Care", "Billed", "Amount", "Currency", "State",
	"Days overdue", "Sent", "Payment due", "Paid", "Refunded",
}

// invoicesExportRules limits the export as a heavy endpoint
// (docs/api-design.md section 6): every call reads the whole book the
// request matches. One Staff member's session, 30 an hour, is a person
// downloading a few views of the book, well below a script's rate. The
// count is taken on the request's own transaction, because the route sits
// behind staffauth.Middleware, which holds the request's one pool
// connection.
var invoicesExportRules = []ratelimit.Rule{
	ratelimit.SessionCookieRule(30, time.Hour),
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
// The CSV is built whole before anything is written: the one Activity row,
// invoices_exported, goes in the same transaction as the read, and a read
// that failed answers 500 with no file and no row. Must be mounted behind
// staffauth.Middleware, for the Owner and Admin roles.
func ExportPracticeInvoicesHandler() http.Handler {
	return staffauth.Handler(func(w http.ResponseWriter, r *http.Request, c staffauth.Call) {
		if !ratelimit.Enforce(w, r, c.Tx, "invoices_export", invoicesExportRules) {
			return
		}
		var req PracticeInvoicesExportRequest
		if !apierr.DecodeJSON(w, r, &req) {
			return
		}
		q, ok := newBodyInvoiceListQuery(w, PracticeInvoicesListRequest{
			View: req.View, Search: req.Search, Order: req.Order, SentFrom: req.SentFrom, SentTo: req.SentTo,
		})
		if !ok {
			return
		}
		zone, err := practicetimezone.Load(r.Context(), c.Tx, c.PracticeID)
		if err != nil {
			// coverage:ignore reason: DB query failure, not exercised by unit tests
			apierr.WriteError(w, apierr.MsgInternalError, http.StatusInternalServerError)
			return
		}
		now := clock.Now(r.Context())
		days := bookDays{zone: zone, now: now}

		items, err := readPracticeInvoices(r.Context(), c.Tx, c.PracticeID, q, days, 0)
		if err != nil {
			// coverage:ignore reason: DB query failure, not exercised by unit tests
			apierr.WriteError(w, apierr.MsgInternalError, http.StatusInternalServerError)
			return
		}
		body := writeInvoicesCSV(items)

		diff, _ := json.Marshal(invoicesExportScope{
			View: q.narrowing.name(), SentFrom: q.period.sentFrom, SentTo: q.period.sentTo,
			Order: req.Order, Rows: len(items), Searched: q.search != "",
		})
		if err := activity.Record(r.Context(), c.Tx, activity.Entry{
			PracticeID:  c.PracticeID,
			SubjectKind: activity.SubjectPractice,
			SubjectID:   c.PracticeID,
			Action:      string(activity.ActionInvoicesExported),
			Diff:        diff,
			Actor:       activity.StaffActor(c.StaffID),
			CreatedAt:   now,
		}); err != nil {
			// coverage:ignore reason: DB query failure, not exercised by unit tests
			apierr.WriteError(w, apierr.MsgInternalError, http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "text/csv; charset=utf-8")
		w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="invoices-%s.csv"`, practiceDay(now, zone)))
		_, _ = w.Write(body)
	}, staffauth.NotAmbientContractor())
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
