package oncall

import (
	"database/sql"
	"errors"
	"net/http"

	"doula-cloud/api/internal/apierr"
	"doula-cloud/api/internal/staffauth"
)

// EngagementOnCall is one Engagement's on-call panel: its window or the
// stated reason it has none, the rule that decides it, each Doula on the
// birth with her narrowing, and every gap in the window that has not
// been cleared.
type EngagementOnCall struct {
	Window         *Window           `json:"window"`
	NoWindowReason NoWindowReason    `json:"noWindowReason,omitempty"`
	Rule           RuleView          `json:"rule"`
	Doulas         []EngagementDoula `json:"doulas"`
	Gaps           []Gap             `json:"gaps"`
}

// RuleView is the rule in force and where it came from. Overridden is
// true where the Engagement carries its own start rule.
type RuleView struct {
	Settings
	Overridden bool `json:"overridden"`
}

// EngagementDoula is one Doula with an open, granted Attachment on the
// birth. From and To are her narrowing, absent where she has none;
// OnCallFrom and OnCallTo are the days she is actually on call, absent
// where there is no window or her narrowing misses it.
type EngagementDoula struct {
	StaffID    string  `json:"staffId"`
	Name       string  `json:"name"`
	From       *string `json:"from,omitempty"`
	To         *string `json:"to,omitempty"`
	OnCallFrom *string `json:"onCallFrom,omitempty"`
	OnCallTo   *string `json:"onCallTo,omitempty"`
}

// EngagementHandler reads one Engagement's on-call panel. Mounted
// AnyStaff and refused here, as every Engagement-scoped read is, with
// Reader.CanAccessEngagement: a contractor not on the birth gets the
// same 404 an Engagement at another Practice gets.
//
// Must be mounted behind staffauth.Middleware.
func EngagementHandler() http.Handler {
	return staffauth.Handler(func(w http.ResponseWriter, r *http.Request, c staffauth.Call) {
		canAccess, err := c.Reader.CanAccessEngagement(r.Context(), c.Tx, c.EngagementID)
		if err != nil {
			// coverage:ignore reason: DB query failure, not exercised by unit tests
			apierr.WriteError(w, apierr.MsgInternalError, http.StatusInternalServerError)
			return
		}

		var kind, status string
		var overridden bool
		err = c.Tx.QueryRowContext(r.Context(),
			`SELECT kind::text, status::text, on_call_start_rule IS NOT NULL
			   FROM engagements WHERE id = $1 AND practice_id = $2`,
			c.EngagementID, c.PracticeID,
		).Scan(&kind, &status, &overridden)
		switch {
		case errors.Is(err, sql.ErrNoRows) || (err == nil && !canAccess):
			apierr.WriteError(w, MsgEngagementGone, http.StatusNotFound)
			return
		case err != nil:
			// coverage:ignore reason: DB query failure, not exercised by unit tests
			apierr.WriteError(w, apierr.MsgInternalError, http.StatusInternalServerError)
			return
		}

		settings, err := loadPracticeSettings(r.Context(), c.Tx, c.PracticeID)
		if err != nil {
			// coverage:ignore reason: DB query failure, not exercised by unit tests
			apierr.WriteError(w, apierr.MsgInternalError, http.StatusInternalServerError)
			return
		}
		resp := EngagementOnCall{
			Rule:   RuleView{Settings: settingsOf(settings.rule), Overridden: overridden},
			Doulas: []EngagementDoula{},
			Gaps:   []Gap{},
		}

		engagements, err := loadEngagements(r.Context(), c.Tx, c.PracticeID, settings, engagementsFilter{engagementID: c.EngagementID})
		if err != nil {
			// coverage:ignore reason: DB query failure, not exercised by unit tests
			apierr.WriteError(w, apierr.MsgInternalError, http.StatusInternalServerError)
			return
		}
		if len(engagements) == 0 {
			// Not a live birth with anyone granted: DeriveWindow says which.
			_, resp.NoWindowReason = DeriveWindow(WindowInput{Kind: kind, Status: status})
			apierr.WriteJSON(w, http.StatusOK, resp)
			return
		}

		e := engagements[0]
		resp.Rule.Settings = settingsOf(e.rule)
		resp.Window, resp.NoWindowReason = e.window, e.reason
		for _, d := range e.doulas {
			row := EngagementDoula{StaffID: d.staffID, Name: d.name, From: d.from, To: d.to}
			if e.window != nil {
				if eff, ok := e.window.Effective(d.from, d.to); ok {
					row.OnCallFrom, row.OnCallTo = &eff.Start, &eff.End
				}
			}
			resp.Doulas = append(resp.Doulas, row)
		}
		if e.window != nil {
			from, to := e.window.Bounds(settings.zone)
			gaps, err := loadLiveGaps(r.Context(), c.Tx, []string{e.id}, from, to)
			if err != nil {
				// coverage:ignore reason: DB query failure, not exercised by unit tests
				apierr.WriteError(w, apierr.MsgInternalError, http.StatusInternalServerError)
				return
			}
			if g := gaps[e.id]; g != nil {
				resp.Gaps = g
			}
		}
		apierr.WriteJSON(w, http.StatusOK, resp)
	}, staffauth.EngagementID())
}
