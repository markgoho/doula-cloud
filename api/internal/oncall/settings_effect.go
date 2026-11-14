package oncall

import (
	"cmp"
	"context"
	"database/sql"
	"net/http"
	"slices"
	"strconv"

	"doula-cloud/api/internal/apierr"
	"doula-cloud/api/internal/clock"
	"doula-cloud/api/internal/staffauth"
)

// MsgEffectNotShown is the refusal of a preview the typed rule cannot be
// read for; the reasons are the write's own, under the same keys.
const MsgEffectNotShown = "The on-call rule could not be previewed."

// EffectBirth is one birth a typed rule would change the day of.
type EffectBirth struct {
	EngagementID string `json:"engagementId"`
	ClientName   string `json:"clientName"`
}

// Effect is what saving a typed rule would do to the births on call today:
// the ones it takes off call, the ones it puts on, and the ones that keep
// their own start whatever the Practice's rule says.
type Effect struct {
	StopToday    []EffectBirth `json:"stopToday"`
	StartToday   []EffectBirth `json:"startToday"`
	KeepOwnStart []EffectBirth `json:"keepOwnStart"`
}

// GetEffectHandler previews a rule: the births on call today under the
// Practice's current rule against the same births under the one in the
// query (startRule, startWeek, graceDays), validated as the write is.
// Owner and Admin only, as the rule is. A read only; nothing is saved.
//
// Must be mounted behind staffauth.Middleware.
func GetEffectHandler() http.Handler {
	return staffauth.Handler(func(w http.ResponseWriter, r *http.Request, c staffauth.Call) {
		query := r.URL.Query()
		typed := Settings{
			StartRule: StartRule(query.Get("startRule")),
			StartWeek: wholeNumber(query.Get("startWeek")),
			GraceDays: wholeNumber(query.Get("graceDays")),
		}
		if details := validateSettings(typed); len(details) > 0 {
			apierr.Write(w, http.StatusBadRequest, apierr.CodeInvalidArgument, MsgEffectNotShown, details)
			return
		}

		current, err := loadPracticeSettings(r.Context(), c.Tx, c.PracticeID)
		if err != nil {
			// coverage:ignore reason: DB query failure, not exercised by unit tests
			apierr.WriteError(w, apierr.MsgInternalError, http.StatusInternalServerError)
			return
		}
		effect, err := effectOf(r.Context(), c.Tx, c.PracticeID, current, Rule{Start: typed.StartRule, Week: typed.StartWeek, GraceDays: typed.GraceDays})
		if err != nil {
			// coverage:ignore reason: DB query failure, not exercised by unit tests
			apierr.WriteError(w, apierr.MsgInternalError, http.StatusInternalServerError)
			return
		}
		apierr.WriteJSON(w, http.StatusOK, effect)
	})
}

// wholeNumber reads a query value as a whole number, and as -1 where it is
// not one, which no bound on the rule admits.
func wholeNumber(raw string) int {
	n, err := strconv.Atoi(raw)
	if err != nil {
		return -1
	}
	return n
}

func effectOf(ctx context.Context, tx *sql.Tx, practiceID string, current practiceSettings, typed Rule) (Effect, error) {
	before, err := loadEngagements(ctx, tx, practiceID, current, engagementsFilter{})
	if err != nil {
		// coverage:ignore reason: DB query failure, not exercised by unit tests
		return Effect{}, err
	}
	proposed := current
	proposed.rule = typed
	after, err := loadEngagements(ctx, tx, practiceID, proposed, engagementsFilter{})
	if err != nil {
		// coverage:ignore reason: DB query failure, not exercised by unit tests
		return Effect{}, err
	}

	today := clock.Now(ctx).In(current.zone).Format(dateLayout)
	onCallAfter := map[string]bool{}
	for _, e := range after {
		onCallAfter[e.id] = e.window != nil && e.window.Start <= today && today <= e.window.End
	}

	effect := Effect{StopToday: []EffectBirth{}, StartToday: []EffectBirth{}, KeepOwnStart: []EffectBirth{}}
	for _, e := range before {
		was := e.window != nil && e.window.Start <= today && today <= e.window.End
		is := onCallAfter[e.id]
		birth := EffectBirth{EngagementID: e.id, ClientName: e.clientName}
		switch {
		case was && !is:
			effect.StopToday = append(effect.StopToday, birth)
		case is && !was:
			effect.StartToday = append(effect.StartToday, birth)
		}
		if e.ownStart != nil && (was || is) {
			effect.KeepOwnStart = append(effect.KeepOwnStart, birth)
		}
	}
	for _, list := range [][]EffectBirth{effect.StopToday, effect.StartToday, effect.KeepOwnStart} {
		slices.SortFunc(list, func(a, b EffectBirth) int {
			return cmp.Or(cmp.Compare(a.ClientName, b.ClientName), cmp.Compare(a.EngagementID, b.EngagementID))
		})
	}
	return effect, nil
}
