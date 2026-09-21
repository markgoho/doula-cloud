package oncall

import (
	"net/http"
	"time"

	"doula-cloud/api/internal/apierr"
	"doula-cloud/api/internal/clock"
	"doula-cloud/api/internal/staffauth"
)

// maxRosterDays bounds one roster read. A quarter holds every question
// the view answers -- tonight, this week, "she is on six births due in
// March" -- and a bound keeps the gap read a range scan, not an archive.
const maxRosterDays = 92

// The refusals this endpoint owns.
const (
	MsgInvalidFrom  = "from must be a date, such as 2026-10-09"
	MsgInvalidTo    = "to must be a date, such as 2026-10-09"
	MsgEmptyRange   = "to must be on or after from"
	MsgRangeTooLong = "Choose a range of 92 days or fewer"
)

// RosterResponse is the whole roster for a range of days.
type RosterResponse struct {
	From     string         `json:"from"`
	To       string         `json:"to"`
	Windows  []RosterWindow `json:"windows"`
	NoWindow []NoWindowRow  `json:"noWindow"`
	Doulas   []RosterDoula  `json:"doulas"`
}

// RosterWindow is one Engagement with a live window in the range: who is
// on call for it, the gaps in force, and whether it is uncovered.
type RosterWindow struct {
	EngagementID  string         `json:"engagementId"`
	ClientName    string         `json:"clientName"`
	DueDate       *string        `json:"dueDate"`
	Window        Window         `json:"window"`
	OnCall        []RosterOnCall `json:"onCall"`
	Gaps          []Gap          `json:"gaps"`
	UnstaffedDays []Window       `json:"unstaffedDays"`
	// Uncovered is true where some part of the range has nobody on call:
	// a run of days no narrowing covers, or a gap with nobody covering it.
	Uncovered bool `json:"uncovered"`
}

// RosterOnCall is one attached Doula and the part of the window she is on
// call for. Narrowed says whether that is her own narrowing or the whole
// window.
type RosterOnCall struct {
	StaffID  string `json:"staffId"`
	Name     string `json:"name"`
	From     string `json:"from"`
	To       string `json:"to"`
	Narrowed bool   `json:"narrowed"`
}

// NoWindowRow is an Engagement that would have a window but cannot say
// when it is, with the reason stated rather than a date guessed.
type NoWindowRow struct {
	EngagementID string         `json:"engagementId"`
	ClientName   string         `json:"clientName"`
	Reason       NoWindowReason `json:"reason"`
}

// RosterDoula is one Doula at the Practice. Available is false where she
// has a live gap in the range. ConcurrentWindows is how many live windows
// she is on call for in it -- nil where the reader may not know, which is
// a contractor reading about a colleague: a count of someone else's
// births is not hers to hold.
type RosterDoula struct {
	StaffID           string `json:"staffId"`
	Name              string `json:"name"`
	Available         bool   `json:"available"`
	ConcurrentWindows *int   `json:"concurrentWindows,omitempty"`
}

// RosterHandler is the on-call roster (#1093): for a range of days,
// every Engagement with a live window, who is on call, the gaps in
// force, and every window left uncovered; and per Doula, how many live
// windows she is carrying.
//
// Mounted AnyStaff, per ADR-0008's Engagements row: an Owner, an Admin
// and an employee Doula read the whole Practice; a contractor Doula
// reads exactly the Engagements she holds an open, granted Attachment
// on, and nothing else -- no other Client's name, due date or Visit. The
// narrowing is in the query (engagementsSelect's $2), decided from the
// request's own Reader, never from anything the caller sent. She still
// reads every colleague's name and whether that colleague is available,
// because a bare name and an availability state carry no Client fact;
// that is the whole of what she reads about anyone else.
//
// Not paginated, which is a departure from docs/api-design.md section 4
// and a deliberate one. That rule is for "any unbounded or growing
// dataset"; this is neither. What comes back is every live window in a
// range the endpoint itself caps at 92 days, drawn only from Engagements
// that are active births with somebody granted on them -- a Practice's
// live book, not its history. And the answer is a whole: the per-Doula
// concurrent-window count and the unstaffed days are totals over the
// range, so a page of it would be a different, wrong answer rather than
// a slower one. A Practice that outgrows this outgrows the range, and
// the fix is a narrower range rather than a cursor.
//
// A read only. Nothing here changes state.
func RosterHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		tx, practiceID, ok := staffauth.RequireTx(w, r)
		// coverage:ignore reason: staffauth.Middleware always sets a tx before this handler runs
		if !ok {
			return
		}
		reader, hasReader := staffauth.ReaderFrom(r.Context())
		staffID, hasStaffID := staffauth.StaffID(r.Context())
		if !hasReader || !hasStaffID {
			// coverage:ignore reason: staffauth.Middleware always places a Reader and a staff id on context before this handler runs
			apierr.WriteError(w, apierr.MsgInternalError, http.StatusInternalServerError)
			return
		}

		settings, err := loadPracticeSettings(r.Context(), tx, practiceID)
		if err != nil {
			// coverage:ignore reason: DB query failure, not exercised by unit tests
			apierr.WriteError(w, apierr.MsgInternalError, http.StatusInternalServerError)
			return
		}

		rng, ok := parseRange(w, r, settings.zone)
		if !ok {
			return
		}

		filter := engagementsFilter{}
		if reader.IsAmbientContractor() {
			filter.contractorStaffID = staffID
		}
		resp, err := buildRoster(r.Context(), tx, practiceID, settings, rng, filter)
		if err != nil {
			// coverage:ignore reason: DB query failure, not exercised by unit tests
			apierr.WriteError(w, apierr.MsgInternalError, http.StatusInternalServerError)
			return
		}
		apierr.WriteJSON(w, http.StatusOK, resp)
	})
}

// parseRange reads from and to off the query string as calendar days,
// writing the refusal itself when either is malformed. An absent from is
// today in the Practice's own zone -- "who is on call tonight" is the
// question the screen opens on -- and an absent to is the same day.
func parseRange(w http.ResponseWriter, r *http.Request, zone *time.Location) (Window, bool) {
	query := r.URL.Query()
	rng := Window{Start: clock.Now(r.Context()).In(zone).Format(dateLayout)}
	if raw := query.Get("from"); raw != "" {
		if _, err := time.Parse(dateLayout, raw); err != nil {
			apierr.WriteError(w, MsgInvalidFrom, http.StatusBadRequest)
			return Window{}, false
		}
		rng.Start = raw
	}
	rng.End = rng.Start
	if raw := query.Get("to"); raw != "" {
		if _, err := time.Parse(dateLayout, raw); err != nil {
			apierr.WriteError(w, MsgInvalidTo, http.StatusBadRequest)
			return Window{}, false
		}
		rng.End = raw
	}
	if rng.End < rng.Start {
		apierr.WriteError(w, MsgEmptyRange, http.StatusBadRequest)
		return Window{}, false
	}
	if rng.End > shiftDay(rng.Start, maxRosterDays-1) {
		apierr.WriteError(w, MsgRangeTooLong, http.StatusBadRequest)
		return Window{}, false
	}
	return rng, true
}
