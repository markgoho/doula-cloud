package client

import (
	"bytes"
	"context"
	"encoding/json/v2"
	"fmt"
	"net/http"
	"slices"
	"strconv"
	"strings"
	"time"

	"doula-cloud/api/internal/activity"
	"doula-cloud/api/internal/apierr"
	"doula-cloud/api/internal/clock"
	"doula-cloud/api/internal/csvsafe"
	"doula-cloud/api/internal/pagecursor"
	"doula-cloud/api/internal/practicetimezone"
	"doula-cloud/api/internal/ratelimit"
	"doula-cloud/api/internal/staffauth"
)

// clientsExportHeader is every column the Clients export writes, in order
// (#2891): what the Clients list shows, in its words. It names no email, phone,
// address, Care Plan, Birth Plan or clinical field.
var clientsExportHeader = []string{
	"Client", "Kind", "Status", "Contract", "Owed", "Invoices owed", "Days overdue", "Due or birth date", "Doula",
}

// clientsExportRules limits the export as a heavy endpoint
// (docs/api-design.md section 6): every call reads every Client the request
// matches. One Staff member's session, 30 an hour, is a person downloading a
// few views of the book, well below a script's rate. The count is taken on
// the request's own transaction, because the route sits behind
// staffauth.Middleware, which holds the request's one pool connection.
var clientsExportRules = []ratelimit.Rule{
	ratelimit.SessionCookieRule(30, time.Hour),
}

// clientsExportScope is the diff of the one Activity row a download writes:
// which narrowing of the book left the product, and how many Clients it was.
// Whether a name was searched is kept, never its text, which an Activity row
// would outlive her Erasure with. The Doula is an id, never a name.
type clientsExportScope struct {
	All        bool   `json:"all,omitzero"`
	Mine       bool   `json:"mine,omitzero"`
	Owes       bool   `json:"owes,omitzero"`
	NoContract bool   `json:"noContract,omitzero"`
	DoulaID    string `json:"doulaId,omitzero"`
	Due        string `json:"due,omitzero"`
	DueFrom    string `json:"dueFrom,omitzero"`
	DueTo      string `json:"dueTo,omitzero"`
	Order      string `json:"order,omitzero"`
	Rows       int    `json:"rows"`
	Searched   bool   `json:"searched"`
}

// ExportHandler answers a POST of the Clients list's own body, a ListRequest
// whose cursor it ignores, with every Client it matches as a CSV (#2891), in the list's order and
// under the list's own checks, so what leaves is what the screen showed. It
// is a POST so a name, which is a Client's, stays out of the URL (#1829).
//
// One row per Client, as the list draws her (ADR-0017): a Client with a
// Birth line and a Postpartum line is one row whose Kind, Status, Contract,
// Due or birth date cells hold each line in turn, joined by "; ", so Owed is
// never counted twice by a spreadsheet's sum and the file's rows are the
// list's Clients.
//
// The CSV is built whole before anything is written: the one Activity row,
// clients_exported, goes in the same transaction as the read, and a read that
// failed answers 500 with no file and no row. Must be mounted behind
// staffauth.Middleware, for the Owner and Admin roles.
func ExportHandler() http.Handler {
	return staffauth.Handler(func(w http.ResponseWriter, r *http.Request, c staffauth.Call) {
		if !ratelimit.Enforce(w, r, c.Tx, "clients_export", clientsExportRules) {
			return
		}
		req, ok := decodeListRequest(w, r)
		if !ok {
			return
		}
		filters, ok := filtersFor(w, c, req)
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

		list, err := listEveryClient(r.Context(), c, filters, req)
		if err != nil {
			// coverage:ignore reason: DB query failure, not exercised by unit tests
			apierr.WriteError(w, apierr.MsgInternalError, http.StatusInternalServerError)
			return
		}
		body := writeClientsCSV(list)

		diff, _ := json.Marshal(clientsExportScope{
			All: req.All, Mine: req.Mine, Owes: req.Owes, NoContract: req.NoContract, DoulaID: req.Doula,
			Due: req.Due, DueFrom: req.DueFrom, DueTo: req.DueTo, Order: req.Order,
			Rows: len(list), Searched: req.Name != "",
		})
		if err := activity.Record(r.Context(), c.Tx, activity.Entry{
			PracticeID:  c.PracticeID,
			SubjectKind: activity.SubjectPractice,
			SubjectID:   c.PracticeID,
			Action:      string(activity.ActionClientsExported),
			Diff:        diff,
			Actor:       activity.StaffActor(c.StaffID),
			CreatedAt:   now,
		}); err != nil {
			// coverage:ignore reason: DB query failure, not exercised by unit tests
			apierr.WriteError(w, apierr.MsgInternalError, http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "text/csv; charset=utf-8")
		w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="clients-%s.csv"`, now.In(zone).Format(time.DateOnly)))
		w.Header().Set("Client-Count", strconv.Itoa(len(list)))
		_, _ = w.Write(body)
	}, staffauth.NotAmbientContractor())
}

// listEveryClient reads the list's own query a page at a time from the first
// Client to the last, so the rows, the order and the narrowing are the list's
// and nothing is skipped or repeated at a page's edge.
func listEveryClient(ctx context.Context, c staffauth.Call, filters listFilters, req ListRequest) ([]ListItem, error) {
	attached := req.Mine
	var every []ListItem
	var after *listAfter
	for {
		var page []ListItem
		var err error
		if attached {
			page, err = listAttachedClients(ctx, c.Tx, c.PracticeID, c.StaffID, filters, req.Order, after)
		} else {
			page, err = listClients(ctx, c.Tx, c.PracticeID, filters, req.Order, after)
		}
		if err != nil {
			// coverage:ignore reason: DB query failure, not exercised by unit tests
			return nil, err
		}
		page, hasMore := pagecursor.Trim(page, pageSize)
		every = append(every, page...)
		if !hasMore {
			break
		}
		last := page[len(page)-1]
		cur, err := decodeListCursor(req.Order)(pagecursor.EncodeKeyed(req.Order, last.cursorKeys(req.Order), last.ClientID))
		if err != nil {
			// coverage:ignore reason: a cursor this function just encoded, not exercised by unit tests
			return nil, err
		}
		after = &cur
	}
	if err := attachOpenEngagements(ctx, c.Tx, c.PracticeID, every, c.Reader, c.StaffID); err != nil {
		// coverage:ignore reason: DB query failure, not exercised by unit tests
		return nil, err
	}
	if err := attachOwedTotals(ctx, c.Tx, every, c.Reader); err != nil {
		// coverage:ignore reason: DB query failure, not exercised by unit tests
		return nil, err
	}
	return every, nil
}

// writeClientsCSV is the header and one row per Client, every cell through
// csvsafe. Writing to a bytes.Buffer cannot fail, so the writer's errors are
// not read.
func writeClientsCSV(list []ListItem) []byte {
	var body bytes.Buffer
	cw := csvsafe.NewWriter(&body)
	_ = cw.Write(clientsExportHeader)
	for _, item := range list {
		_ = cw.Write(clientsExportRow(item))
	}
	cw.Flush()
	return body.Bytes()
}

func clientsExportRow(item ListItem) []string {
	var kinds, statuses, contracts, dues, doulas []string
	var cents int64
	var owedCount, overdue int
	if item.OwedCents != nil {
		cents = *item.OwedCents
	}
	if item.OwedInvoiceCount != nil {
		owedCount = *item.OwedInvoiceCount
	}
	for _, line := range item.OpenEngagements {
		kinds = append(kinds, exportKind(line.Kind))
		statuses = append(statuses, exportEngagementStatus(line.EngagementStatus))
		contracts = append(contracts, exportContract(line.ContractStatus))
		dues = append(dues, exportDay(line))
		for _, name := range line.DoulaNames {
			if !slices.Contains(doulas, name) {
				doulas = append(doulas, name)
			}
		}
		if line.InvoiceStatus != nil && *line.InvoiceStatus == "open" && (line.PaymentProcessing == nil || !*line.PaymentProcessing) && line.DaysOverdue != nil {
			overdue = max(overdue, *line.DaysOverdue)
		}
	}
	for _, kind := range item.PendingRequestKinds {
		statuses = append(statuses, exportKind(kind)+" request pending")
	}
	if len(statuses) == 0 {
		statuses = append(statuses, exportNoLines(item.HasWork))
	}
	return []string{
		item.Name, strings.Join(kinds, "; "), strings.Join(statuses, "; "), strings.Join(contracts, "; "),
		fmt.Sprintf("%d.%02d", cents/100, cents%100), strconv.Itoa(owedCount), strconv.Itoa(overdue),
		strings.Join(dues, "; "), strings.Join(doulas, "; "),
	}
}

func exportKind(kind string) string {
	if kind == "postpartum" {
		return "Postpartum"
	}
	return "Birth"
}

func exportEngagementStatus(status string) string {
	return strings.ToUpper(status[:1]) + status[1:]
}

func exportContract(status *string) string {
	if status == nil {
		return "No Contract yet"
	}
	return "Contract " + *status
}

func exportNoLines(hasWork bool) string {
	if hasWork {
		return "Care ended"
	}
	return "Not yet in care"
}

func exportDay(line OpenEngagement) string {
	switch {
	case line.PregnancyEndedOn != nil:
		return *line.PregnancyEndedOn
	case line.DueDate != nil:
		return *line.DueDate
	}
	return ""
}
