package contracts

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

// contractsExportHeader is every column the Contracts export writes, in
// order (#2954): what the Contracts list shows, in its words. It names no
// email, phone, address, due date or birth outcome.
var contractsExportHeader = []string{
	"Client", "Care", "Status", "Status day", "Sent", "Days waiting", "Who", "Doula", "Fee", "Void requested",
}

// contractsExportScope is the diff of the one Activity row a download
// writes: which slice of the book left the product, and how big it was.
// Whether a search was applied is kept, never its text, which can be a
// Client's name that an Activity row would outlive her Erasure with. The
// Doula is an id, never a name.
type contractsExportScope struct {
	View     string `json:"view"`
	Mine     bool   `json:"mine,omitzero"`
	DoulaID  string `json:"doulaId,omitzero"`
	SentFrom string `json:"sentFrom,omitzero"`
	SentTo   string `json:"sentTo,omitzero"`
	Order    string `json:"order,omitzero"`
	Rows     int    `json:"rows"`
	Searched bool   `json:"searched"`
}

// ExportContractsHandler answers a POST of the Contracts list's own body, a
// ListRequest whose cursor it ignores, with every Contract it matches as a
// CSV (#2954), in the list's order and under the list's own checks, so what
// leaves is what the screen showed. It is a POST so a search, which can be a
// Client's name, stays out of the URL (#1829). The rate limit, the Activity
// row contracts_exported and the response are csvexport's. Must be mounted
// behind staffauth.Middleware, for the Owner and Admin roles: the Fee column
// is the Practice's money, and no contractor Doula reaches this route.
func ExportContractsHandler() http.Handler {
	spec := csvexport.Spec{Limit: "contracts_export", Action: activity.ActionContractsExported, File: "contracts"}
	return csvexport.Handler(spec, func(w http.ResponseWriter, r *http.Request, c staffauth.Call, env csvexport.Env) (csvexport.File, bool) {
		var req ListRequest
		if !apierr.DecodeJSON(w, r, &req) {
			return csvexport.File{}, false
		}
		q, ok := newListQuery(w, c, req, env.Zone)
		if !ok {
			return csvexport.File{}, false
		}
		items, err := q.readRows(r.Context(), c.Tx, c.PracticeID, nil, 0, env.Zone, env.Now)
		if err != nil {
			// coverage:ignore reason: DB query failure, not exercised by unit tests
			apierr.WriteError(w, apierr.MsgInternalError, http.StatusInternalServerError)
			return csvexport.File{}, false
		}
		view := req.View
		if view == viewNotSigned {
			view = "not-signed"
		}
		return csvexport.File{
			Body: writeContractsCSV(items),
			Diff: contractsExportScope{
				View: view, Mine: req.Mine, DoulaID: req.Doula, SentFrom: q.period.SentFrom, SentTo: q.period.SentTo,
				Order: req.Order, Rows: len(items), Searched: q.search != "",
			},
			Headers: map[string]string{"Contract-Count": strconv.Itoa(len(items))},
		}, true
	})
}

// writeContractsCSV is the header and one row per Contract, every cell
// through csvsafe. Writing to a bytes.Buffer cannot fail, so the writer's
// errors are not read.
func writeContractsCSV(items []ContractListItem) []byte {
	var body bytes.Buffer
	cw := csvsafe.NewWriter(&body)
	_ = cw.Write(contractsExportHeader)
	for _, it := range items {
		waiting := ""
		if it.DaysWaiting != nil {
			waiting = strconv.Itoa(*it.DaysWaiting)
		}
		fee := ""
		if it.AmountCents != nil {
			fee = fmt.Sprintf("%d.%02d", *it.AmountCents/100, *it.AmountCents%100)
		}
		requested := ""
		if it.VoidRequested {
			requested = "yes"
		}
		_ = cw.Write([]string{
			it.ClientName, exportCare(it.EngagementKind), exportStatus(it.Status), it.SteppedOn, it.sentOn, waiting,
			exportWho(it), strings.Join(it.DoulaNames, "; "), fee, requested,
		})
	}
	cw.Flush()
	return body.Bytes()
}

func exportCare(kind string) string {
	if kind == "postpartum" {
		return "Postpartum"
	}
	return "Birth"
}

func exportStatus(status string) string {
	return strings.ToUpper(status[:1]) + status[1:]
}

// exportWho is the list's Who cell in its words: "Sent by Renata Alvarez".
func exportWho(it ContractListItem) string {
	verb, name := "Sent", it.SentByName
	switch it.Status {
	case string(StatusDraft):
		verb, name = "Made", it.MadeByName
	case string(StatusVoided):
		verb, name = "Voided", it.VoidedByName
	}
	if name == "" {
		return ""
	}
	return verb + " by " + name
}
