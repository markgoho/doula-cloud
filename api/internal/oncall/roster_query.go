package oncall

import (
	"net/http"
	"time"

	"doula-cloud/api/internal/apierr"
	"doula-cloud/api/internal/clock"
)

// The refusals this endpoint owns, keyed in details by the query
// parameter's own name.
const (
	MsgDaysNotShown    = "The days could not be shown."
	MsgRosterNotShown  = "Who is on call could not be shown."
	MsgEnterFromDay    = "Enter a From day"
	MsgEnterToDay      = "Enter a To day"
	MsgFromBeforeToday = "Choose today or a later day. On call shows only births still to come."
	MsgToBeforeFrom    = "Choose a To day that is the same as the From day or later."
	MsgRangeTooLong    = "Choose a range of 92 days or fewer."
	MsgUnknownOrder    = "Choose an order this list offers."
	MsgUnknownCover    = "Choose which births to show."
)

// The query parameters and their values.
const (
	paramFrom    = "from"
	paramTo      = "to"
	paramOrder   = "order"
	paramCover   = "cover"
	paramStaffID = "staffId"

	orderCover  = "cover"
	coverNeeded = "needed"
)

// rosterQuery is a roster request after it has been read: the days, the
// Practice's today and the instant the read is made at, and the three
// narrowings.
type rosterQuery struct {
	rng         Window
	today       string
	now         time.Time
	staffID     string
	coverNeeded bool
	byCover     bool
}

// includesToday reports whether the range holds the Practice's today.
func (q rosterQuery) includesToday() bool {
	return q.rng.Start <= q.today && q.today <= q.rng.End
}

// parseRosterQuery reads the roster's query string, writing the refusal
// itself when any part is wrong. An absent from is today in the Practice's
// own zone -- "who is on call tonight" is the question the screen opens on
// -- and an absent to is the same day as from. Every fault is reported at
// once, under the parameter at fault.
func parseRosterQuery(w http.ResponseWriter, r *http.Request, zone *time.Location) (rosterQuery, bool) {
	query := r.URL.Query()
	now := clock.Now(r.Context())
	today := now.In(zone).Format(dateLayout)
	q := rosterQuery{rng: Window{Start: today}, today: today, now: now, staffID: query.Get(paramStaffID)}
	details := map[string]string{}

	fromOK, toOK := true, true
	if raw := query.Get(paramFrom); raw != "" {
		if _, err := time.Parse(dateLayout, raw); err != nil {
			details[paramFrom], fromOK = MsgEnterFromDay, false
		} else {
			q.rng.Start = raw
		}
	}
	q.rng.End = q.rng.Start
	if raw := query.Get(paramTo); raw != "" {
		if _, err := time.Parse(dateLayout, raw); err != nil {
			details[paramTo], toOK = MsgEnterToDay, false
		} else {
			q.rng.End = raw
		}
	}
	if fromOK && q.rng.Start < today {
		details[paramFrom] = MsgFromBeforeToday
	}
	if fromOK && toOK {
		switch {
		case q.rng.End < q.rng.Start:
			details[paramTo] = MsgToBeforeFrom
		case q.rng.End > shiftDay(q.rng.Start, maxRosterDays-1):
			details[paramTo] = MsgRangeTooLong
		}
	}
	rangeFault := len(details) > 0

	switch query.Get(paramOrder) {
	case "":
	case orderCover:
		q.byCover = true
	default:
		details[paramOrder] = MsgUnknownOrder
	}
	switch query.Get(paramCover) {
	case "":
	case coverNeeded:
		q.coverNeeded = true
	default:
		details[paramCover] = MsgUnknownCover
	}

	if len(details) > 0 {
		message := MsgRosterNotShown
		if rangeFault {
			message = MsgDaysNotShown
		}
		apierr.Write(w, http.StatusBadRequest, apierr.CodeInvalidArgument, message, details)
		return rosterQuery{}, false
	}
	return q, true
}
