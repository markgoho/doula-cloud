package payments

import (
	"database/sql"
	"fmt"
	"net/http"
	"time"

	"doula-cloud/api/internal/apierr"
)

const (
	msgSentFromInvalid  = "Use a date like 2026-10-31 for sentFrom."
	msgSentToInvalid    = "Use a date like 2026-10-31 for sentTo."
	msgSentBothDays     = "Send both sentFrom and sentTo, or neither."
	msgSentToBeforeFrom = "Use a sentTo that is not before sentFrom."
)

type invoicePeriod struct {
	sentFrom, sentTo string
	from, to         time.Time
	set              bool
}

func parseInvoicePeriod(w http.ResponseWriter, sentFrom, sentTo string) (invoicePeriod, bool) {
	if sentFrom == "" && sentTo == "" {
		return invoicePeriod{}, true
	}
	if sentFrom == "" || sentTo == "" {
		apierr.WriteError(w, msgSentBothDays, http.StatusBadRequest)
		return invoicePeriod{}, false
	}
	from, err := time.Parse(paidOnLayout, sentFrom)
	if err != nil {
		apierr.WriteError(w, msgSentFromInvalid, http.StatusBadRequest)
		return invoicePeriod{}, false
	}
	to, err := time.Parse(paidOnLayout, sentTo)
	if err != nil {
		apierr.WriteError(w, msgSentToInvalid, http.StatusBadRequest)
		return invoicePeriod{}, false
	}
	if to.Before(from) {
		apierr.WriteError(w, msgSentToBeforeFrom, http.StatusBadRequest)
		return invoicePeriod{}, false
	}
	return invoicePeriod{sentFrom: sentFrom, sentTo: sentTo, from: from, to: to, set: true}, true
}

func (p invoicePeriod) bounds(zone *time.Location) (start, end sql.NullTime) {
	if !p.set {
		return sql.NullTime{}, sql.NullTime{}
	}
	start = sql.NullTime{Time: time.Date(p.from.Year(), p.from.Month(), p.from.Day(), 0, 0, 0, 0, zone), Valid: true}
	end = sql.NullTime{Time: time.Date(p.to.Year(), p.to.Month(), p.to.Day()+1, 0, 0, 0, 0, zone), Valid: true}
	return start, end
}

func (p invoicePeriod) clause(zone *time.Location, bind func(any) int) string {
	if !p.set {
		return ""
	}
	start, end := p.bounds(zone)
	return fmt.Sprintf(` AND i.created_at >= $%d AND i.created_at < $%d`, bind(start.Time), bind(end.Time))
}
