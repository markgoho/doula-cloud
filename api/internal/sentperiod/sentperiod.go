// Package sentperiod is the period a Staff list narrows to by the day a row
// was sent, in the Practice's zone (#2871, #2954): the Invoices list's and the
// Contracts list's, read alike from ?sentFrom= and ?sentTo= or a body's two
// days.
package sentperiod

import (
	"database/sql"
	"fmt"
	"net/http"
	"time"

	"doula-cloud/api/internal/apierr"
)

const (
	dayLayout = "2006-01-02"

	msgFromInvalid  = "Use a date like 2026-10-31 for sentFrom."
	msgToInvalid    = "Use a date like 2026-10-31 for sentTo."
	msgBothDays     = "Send both sentFrom and sentTo, or neither."
	msgToBeforeFrom = "Use a sentTo that is not before sentFrom."
)

// Period is two days, both included, or none.
type Period struct {
	SentFrom, SentTo string
	from, to         time.Time
	set              bool
}

// Parse reads the two days a request named, answering 400 itself when they
// are not a period.
func Parse(w http.ResponseWriter, sentFrom, sentTo string) (Period, bool) {
	if sentFrom == "" && sentTo == "" {
		return Period{}, true
	}
	if sentFrom == "" || sentTo == "" {
		apierr.WriteError(w, msgBothDays, http.StatusBadRequest)
		return Period{}, false
	}
	from, err := time.Parse(dayLayout, sentFrom)
	if err != nil {
		apierr.WriteError(w, msgFromInvalid, http.StatusBadRequest)
		return Period{}, false
	}
	to, err := time.Parse(dayLayout, sentTo)
	if err != nil {
		apierr.WriteError(w, msgToInvalid, http.StatusBadRequest)
		return Period{}, false
	}
	if to.Before(from) {
		apierr.WriteError(w, msgToBeforeFrom, http.StatusBadRequest)
		return Period{}, false
	}
	return Period{SentFrom: sentFrom, SentTo: sentTo, from: from, to: to, set: true}, true
}

// Set reports whether a period was named.
func (p Period) Set() bool { return p.set }

// Bounds are the instants the period starts and ends before, the Practice's
// midnights, or NULLs when there is no period.
func (p Period) Bounds(zone *time.Location) (start, end sql.NullTime) {
	if !p.set {
		return sql.NullTime{}, sql.NullTime{}
	}
	start = sql.NullTime{Time: time.Date(p.from.Year(), p.from.Month(), p.from.Day(), 0, 0, 0, 0, zone), Valid: true}
	end = sql.NullTime{Time: time.Date(p.to.Year(), p.to.Month(), p.to.Day()+1, 0, 0, 0, 0, zone), Valid: true}
	return start, end
}

// Clause is the AND narrowing column to the period, or empty with none. bind
// takes a value and returns the placeholder that stands for it.
func (p Period) Clause(column string, zone *time.Location, bind func(any) string) string {
	if !p.set {
		return ""
	}
	start, end := p.Bounds(zone)
	return fmt.Sprintf(` AND %s >= %s AND %s < %s`, column, bind(start.Time), column, bind(end.Time))
}
