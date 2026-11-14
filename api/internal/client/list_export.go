package client

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
	"slices"
	"strconv"
	"strings"

	"doula-cloud/api/internal/activity"
	"doula-cloud/api/internal/apierr"
	"doula-cloud/api/internal/csvexport"
	"doula-cloud/api/internal/csvsafe"
	"doula-cloud/api/internal/pagecursor"
	"doula-cloud/api/internal/staffauth"
)

// clientsExportHeader is every column the Clients export writes, in order
// (#2891): what the Clients list shows, in its words. It names no email, phone,
// address, Care Plan, Birth Plan or clinical field.
var clientsExportHeader = []string{
	"Client", "Kind", "Status", "Contract", "Owed", "Invoices owed", "Days overdue", "Due or birth date", "Doula",
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
// whose cursor it ignores, with every Client it matches as a CSV (#2891), in
// the list's order and under the list's own checks, so what leaves is what
// the screen showed. It is a POST so a name, which is a Client's, stays out of
// the URL (#1829). The rate limit, the Activity row clients_exported and the
// response are csvexport's.
//
// One row per Client, as the list draws her (ADR-0017): a Client with a
// Birth line and a Postpartum line is one row whose Kind, Status, Contract,
// Due or birth date cells hold each line in turn, joined by "; ", so Owed is
// never counted twice by a spreadsheet's sum and the file's rows are the
// list's Clients. Must be mounted behind staffauth.Middleware, for the Owner
// and Admin roles.
func ExportHandler() http.Handler {
	spec := csvexport.Spec{Limit: "clients_export", Action: activity.ActionClientsExported, File: "clients"}
	return csvexport.Handler(spec, func(w http.ResponseWriter, r *http.Request, c staffauth.Call, _ csvexport.Env) (csvexport.File, bool) {
		req, ok := decodeListRequest(w, r)
		if !ok {
			return csvexport.File{}, false
		}
		filters, ok := filtersFor(w, c, req)
		if !ok {
			return csvexport.File{}, false
		}
		list, err := listEveryClient(r.Context(), c, filters, req)
		if err != nil {
			// coverage:ignore reason: DB query failure, not exercised by unit tests
			apierr.WriteError(w, apierr.MsgInternalError, http.StatusInternalServerError)
			return csvexport.File{}, false
		}
		return csvexport.File{
			Body: writeClientsCSV(list),
			Diff: clientsExportScope{
				All: req.All, Mine: req.Mine, Owes: req.Owes, NoContract: req.NoContract, DoulaID: req.Doula,
				Due: req.Due, DueFrom: req.DueFrom, DueTo: req.DueTo, Order: req.Order,
				Rows: len(list), Searched: req.Name != "",
			},
			Headers: map[string]string{"Client-Count": strconv.Itoa(len(list))},
		}, true
	})
}

// listEveryClient reads the list's own query a page at a time from the first
// Client to the last, so the rows, the order and the narrowing are the list's
// and nothing is skipped or repeated at a page's edge.
func listEveryClient(ctx context.Context, c staffauth.Call, filters listFilters, req ListRequest) ([]ListItem, error) {
	attached := req.Mine
	zone, err := listZone(ctx, c, &filters)
	if err != nil {
		// coverage:ignore reason: DB query failure or a stored zone that will not load, not exercised by unit tests
		return nil, err
	}
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
	if err := attachOwedTotals(ctx, c.Tx, every, c.Reader, zone); err != nil {
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
