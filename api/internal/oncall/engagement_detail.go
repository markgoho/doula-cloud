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
// birth. From and To are her narrowing, null where she has none;
// OnCallFrom and OnCallTo are the days she is actually on call, null
// where there is no window or her narrowing misses it.
type EngagementDoula struct {
	StaffID    string  `json:"staffId"`
	Name       string  `json:"name"`
	From       *string `json:"from"`
	To         *string `json:"to"`
	OnCallFrom *string `json:"onCallFrom"`
	OnCallTo   *string `json:"onCallTo"`
}

// EngagementHandler reads one Engagement's on-call panel. Mounted
// AnyStaff and refused here, as every Engagement-scoped read is, with
// Reader.CanAccessEngagement: a contractor not on the birth gets the
// same 404 an Engagement at another Practice gets.
//
// Must be mounted behind staffauth.Middleware.
func EngagementHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		tx, practiceID, ok := staffauth.RequireTx(w, r)
		// coverage:ignore reason: staffauth.Middleware always sets a tx before this handler runs
		if !ok {
			return
		}
		engagementID := r.PathValue("engagementId")
		if !staffauth.ParseUUID(w, "engagement", engagementID) {
			return
		}
		reader, _ := staffauth.ReaderFrom(r.Context())
		canAccess, err := reader.CanAccessEngagement(r.Context(), tx, engagementID)
		if err != nil {
			// coverage:ignore reason: DB query failure, not exercised by unit tests
			apierr.WriteError(w, apierr.MsgInternalError, http.StatusInternalServerError)
			return
		}

		var kind, status string
		var overridden bool
		err = tx.QueryRowContext(r.Context(),
			`SELECT kind::text, status::text, on_call_start_rule IS NOT NULL
			   FROM engagements WHERE id = $1 AND practice_id = $2`,
			engagementID, practiceID,
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

		settings, err := loadPracticeSettings(r.Context(), tx, practiceID)
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

		engagements, err := loadEngagements(r.Context(), tx, practiceID, settings, engagementsFilter{engagementID: engagementID})
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
			gaps, err := loadLiveGaps(r.Context(), tx, []string{e.id}, from, to)
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
	})
}
