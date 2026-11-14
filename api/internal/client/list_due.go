package client

import (
	"fmt"
	"net/http"
	"time"

	"doula-cloud/api/internal/apierr"
)

const (
	monthLayout = "2006-01"
	dayLayout   = time.DateOnly
)

type dueRange struct {
	from string
	to   string
}

func (d dueRange) args() []any {
	if d.from == "" {
		return []any{nil, nil}
	}
	return []any{d.from, d.to}
}

func parseDay(raw string) (time.Time, bool) {
	if len(raw) != len(dayLayout) {
		return time.Time{}, false
	}
	day, err := time.Parse(dayLayout, raw)
	return day, err == nil
}

// dueParams reads ?due or ?dueFrom and ?dueTo into inclusive days, writing a
// 400 and returning false for anything else.
func dueParams(w http.ResponseWriter, due, from, to string) (dueRange, bool) {
	refuse := func(field, message string) (dueRange, bool) {
		apierr.WriteFieldError(w, http.StatusBadRequest, apierr.CodeInvalidArgument, field, message)
		return dueRange{}, false
	}
	switch {
	case due != "" && (from != "" || to != ""):
		return refuse("due", "Send a due month or a range of days, not both.")
	case due != "":
		first, err := time.Parse(monthLayout, due)
		if err != nil || len(due) != len(monthLayout) {
			return refuse("due", "Use a month like 2027-03 for due.")
		}
		return dueRange{from: first.Format(dayLayout), to: first.AddDate(0, 1, -1).Format(dayLayout)}, true
	case from == "" && to == "":
		return dueRange{}, true
	case from == "":
		return refuse("dueFrom", "Send dueFrom with dueTo.")
	case to == "":
		return refuse("dueTo", "Send dueTo with dueFrom.")
	}
	start, ok := parseDay(from)
	if !ok {
		return refuse("dueFrom", "Use a date like 2026-10-31 for dueFrom.")
	}
	end, ok := parseDay(to)
	if !ok {
		return refuse("dueTo", "Use a date like 2026-10-31 for dueTo.")
	}
	if end.Before(start) {
		return refuse("dueTo", "Use a dueTo on or after dueFrom.")
	}
	return dueRange{from: from, to: to}, true
}

// dueExpr is ?due, ?dueFrom and ?dueTo, bound as $%[1]d and $%[2]d, both
// NULL for none: an open Engagement whose due date, a date with no zone, lies
// in the inclusive range, whether or not the pregnancy has ended.
const dueExpr = `($%[1]d::date IS NULL OR EXISTS (
	SELECT 1 FROM engagements ue
	WHERE ue.client_id = c.id AND ue.status <> 'completed'
	  AND ue.due_date BETWEEN $%[1]d::date AND $%[2]d::date
))`

func dueNarrowing(first int) string {
	return fmt.Sprintf(dueExpr, first, first+1)
}
