package oncall

import (
	"net/http"
	"time"

	"doula-cloud/api/internal/apierr"
	"doula-cloud/api/internal/staffauth"
)

// maxRosterDays bounds one roster read. A quarter holds every question
// the view answers -- tonight, this week, "she is on six births due in
// March" -- and a bound keeps the gap read a range scan, not an archive.
const maxRosterDays = 92

// RosterResponse is the whole roster for a range of days. Today is the
// Practice's calendar day, the one From can never precede, and Rule is the
// Practice's own rule every birth without its own start follows.
type RosterResponse struct {
	From     string         `json:"from"`
	To       string         `json:"to"`
	Today    string         `json:"today"`
	Rule     Settings       `json:"rule"`
	Windows  []RosterWindow `json:"windows"`
	NoWindow []NoWindowRow  `json:"noWindow"`
	Doulas   []RosterDoula  `json:"doulas"`
}

// RosterWindow is one birth with on-call days in the range: who is on call
// for it, the gaps in force, and whether it is uncovered. A birth with no
// Doula yet is here too, over the days its rule would give it, with
// NoDoula set and nobody on call.
type RosterWindow struct {
	EngagementID  string         `json:"engagementId"`
	ClientName    string         `json:"clientName"`
	DueDate       *string        `json:"dueDate"`
	Window        Window         `json:"window"`
	OwnStart      *OwnStart      `json:"ownStart,omitempty"`
	NoDoula       bool           `json:"noDoula"`
	OnCall        []RosterOnCall `json:"onCall"`
	Gaps          []Gap          `json:"gaps"`
	UnstaffedDays []Window       `json:"unstaffedDays"`
	// Uncovered is true where some part of the range has nobody on call:
	// a run of days no narrowing covers, or a gap with nobody covering it.
	Uncovered bool `json:"uncovered"`
	// Tonight is present only when today is in the range and in the
	// birth's days.
	Tonight *Tonight `json:"tonight,omitempty"`
}

// Tonight says who holds the birth for the rest of the Practice's today.
type Tonight struct {
	Holders  []Holder `json:"holders"`
	IsNobody bool     `json:"isNobody"`
}

// Holder is one Doula holding a birth tonight. CoveringFor is the name of
// the Doula she covers where a covered gap is in force.
type Holder struct {
	StaffID     string  `json:"staffId"`
	Name        string  `json:"name"`
	CoveringFor *string `json:"coveringFor,omitempty"`
}

// RosterOnCall is one attached Doula and the part of the window she is on
// call for. Narrowed says whether that is her own narrowing or the whole
// window. AtOnce names each other birth whose days she shares with this
// one inside the range; absent where there is none, and absent for a
// contractor reading a colleague.
type RosterOnCall struct {
	StaffID  string        `json:"staffId"`
	Name     string        `json:"name"`
	From     string        `json:"from"`
	To       string        `json:"to"`
	Narrowed bool          `json:"narrowed"`
	AtOnce   []AtOnceBirth `json:"atOnce,omitempty"`
}

// AtOnceBirth is the other birth and the days both are hers.
type AtOnceBirth struct {
	EngagementID string `json:"engagementId"`
	ClientName   string `json:"clientName"`
	Days         Window `json:"days"`
}

// StaffRef is a Doula as a list names her.
type StaffRef struct {
	StaffID string `json:"staffId"`
	Name    string `json:"name"`
}

// NoWindowRow is an Engagement that would have a window but cannot say
// when it is, with the reason stated rather than a date guessed. Doulas
// are the people on it, none for a birth with nobody on it yet.
type NoWindowRow struct {
	EngagementID string         `json:"engagementId"`
	ClientName   string         `json:"clientName"`
	Reason       NoWindowReason `json:"reason"`
	Doulas       []StaffRef     `json:"doulas"`
}

// RosterDoula is one Doula at the Practice: the births she is on call for
// or covers a gap on in the range, the births whose days she shares, the
// gaps of hers in force, and whether she holds a birth tonight. For a
// contractor the list is only the Doulas on her births, each with only
// those births, and AtOnce is absent for a colleague.
type RosterDoula struct {
	StaffID         string        `json:"staffId"`
	Name            string        `json:"name"`
	Births          []DoulaBirth  `json:"births"`
	AtOnce          []DoulaAtOnce `json:"atOnce,omitempty"`
	Unreachable     []Unreachable `json:"unreachable"`
	IsOnCallTonight bool          `json:"isOnCallTonight"`
}

// DoulaBirth is one birth a Doula is on call for, or covers a gap on.
type DoulaBirth struct {
	EngagementID string `json:"engagementId"`
	ClientName   string `json:"clientName"`
	Covering     bool   `json:"covering"`
}

// DoulaAtOnce is two births whose days she is on call for at once, and the
// days they share inside the range.
type DoulaAtOnce struct {
	Days          Window   `json:"days"`
	EngagementIDs []string `json:"engagementIds"`
}

// Unreachable is one gap of a Doula's in force in the range.
type Unreachable struct {
	GapID        string    `json:"gapId"`
	EngagementID string    `json:"engagementId"`
	ClientName   string    `json:"clientName"`
	StartsAt     time.Time `json:"startsAt"`
	EndsAt       time.Time `json:"endsAt"`
	IsCovered    bool      `json:"isCovered"`
}

// RosterHandler is the on-call roster (#1093): for a range of days, every
// birth with on-call days in it, who is on call, the gaps in force, and
// every birth left uncovered; and per Doula, the births she holds.
//
// Mounted AnyStaff, per ADR-0008's Engagements row: an Owner, an Admin
// and an employee Doula read the whole Practice; a contractor Doula
// reads exactly the Engagements she holds an open, granted Attachment
// on, and nothing else -- no other Client's name, due date or Visit, no
// birth with nobody on it, and of a colleague only a name and what the
// colleague does on her own births. The narrowing is in the query
// (engagementsSelect's $2), decided from the request's own Reader, never
// from anything the caller sent; staffId, cover and order only narrow or
// reorder what that query returned.
//
// Not paginated, which is a departure from docs/api-design.md section 4
// and a deliberate one. That rule is for "any unbounded or growing
// dataset"; this is neither. What comes back is every live birth in a
// range the endpoint itself caps at 92 days, drawn only from Engagements
// that are births not yet completed -- a Practice's live book, not its
// history. And the answer is a whole: the flags and the Doula totals are
// over the range, so a page of it would be a different, wrong answer
// rather than a slower one. A Practice that outgrows this outgrows the
// range, and the fix is a narrower range rather than a cursor.
//
// A read only. Nothing here changes state.
func RosterHandler() http.Handler {
	return staffauth.Handler(func(w http.ResponseWriter, r *http.Request, c staffauth.Call) {
		settings, err := loadPracticeSettings(r.Context(), c.Tx, c.PracticeID)
		if err != nil {
			// coverage:ignore reason: DB query failure, not exercised by unit tests
			apierr.WriteError(w, apierr.MsgInternalError, http.StatusInternalServerError)
			return
		}

		q, ok := parseRosterQuery(w, r, settings.zone)
		if !ok {
			return
		}

		filter := engagementsFilter{}
		if c.Reader.IsAmbientContractor() {
			filter.contractorStaffID = c.StaffID
		}
		resp, err := buildRoster(r.Context(), c.Tx, c.PracticeID, settings, q, filter)
		if err != nil {
			// coverage:ignore reason: DB query failure, not exercised by unit tests
			apierr.WriteError(w, apierr.MsgInternalError, http.StatusInternalServerError)
			return
		}
		apierr.WriteJSON(w, http.StatusOK, resp)
	})
}
