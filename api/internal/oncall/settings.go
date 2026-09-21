package oncall

import (
	"context"
	"database/sql"
	"fmt"
	"net/http"

	"doula-cloud/api/internal/activity"
	"doula-cloud/api/internal/apierr"
	"doula-cloud/api/internal/staffauth"
)

// actionPracticeOnCallChanged records a change to a Practice's on-call
// rule -- Practice-scoped and plain-string, the shape
// practicetimezone's actionTimezoneChanged uses for its sibling setting.
const actionPracticeOnCallChanged = "practice_on_call_changed"

// The bounds 00114's CHECKs hold, restated so the refusal can say them.
const (
	minStartWeek = 20
	maxStartWeek = 42
	maxGraceDays = 42
)

// The refusals a rule write owns.
const (
	MsgUnknownStartRule = "Choose when on call starts: a week of pregnancy, or the day the doula was attached."
	MsgStartWeekRange   = "Enter a week from 20 to 42."
	MsgGraceDaysRange   = "Enter from 0 to 42 days."
	MsgWeekWithoutRule  = "A week applies only when on call starts at a week of pregnancy."
)

// Settings is a Practice's on-call rule, as read and as written.
type Settings struct {
	StartRule StartRule `json:"startRule"`
	StartWeek int       `json:"startWeek"`
	GraceDays int       `json:"graceDays"`
}

// GetSettingsHandler reads the Practice's on-call rule. Owner and Admin
// only, declared at the mount, the pair that may state it -- the same
// authority practicetimezone gives the zone. Every Staff member still
// meets the rule's effect, on the roster and on the Engagement panel.
//
// Must be mounted behind staffauth.Middleware.
func GetSettingsHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		tx, practiceID, ok := staffauth.RequireTx(w, r)
		// coverage:ignore reason: staffauth.Middleware always sets a tx before this handler runs
		if !ok {
			return
		}
		s, err := loadPracticeSettings(r.Context(), tx, practiceID)
		if err != nil {
			// coverage:ignore reason: DB query failure, not exercised by unit tests
			apierr.WriteError(w, apierr.MsgInternalError, http.StatusInternalServerError)
			return
		}
		apierr.WriteJSON(w, http.StatusOK, settingsOf(s.rule))
	})
}

// PutSettingsHandler states a Practice's on-call rule. Idempotent by
// construction, as practicetimezone.PutHandler is: the same body writes
// nothing and records nothing new. Changing it moves every window at
// once, because nothing stores a window.
//
// Must be mounted behind staffauth.Middleware.
func PutSettingsHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		tx, practiceID, ok := staffauth.RequireTx(w, r)
		// coverage:ignore reason: staffauth.Middleware always sets a tx before this handler runs
		if !ok {
			return
		}
		var req Settings
		if !apierr.DecodeJSON(w, r, &req) {
			return
		}
		details := map[string]string{}
		if req.StartRule != StartGestationalWeek && req.StartRule != StartAttachmentGranted {
			details["startRule"] = MsgUnknownStartRule
		}
		if req.StartWeek < minStartWeek || req.StartWeek > maxStartWeek {
			details["startWeek"] = MsgStartWeekRange
		}
		if req.GraceDays < 0 || req.GraceDays > maxGraceDays {
			details["graceDays"] = MsgGraceDaysRange
		}
		if len(details) > 0 {
			apierr.Write(w, http.StatusBadRequest, apierr.CodeInvalidArgument, "The on-call rule could not be saved.", details)
			return
		}

		current, err := loadPracticeSettings(r.Context(), tx, practiceID)
		if err != nil {
			// coverage:ignore reason: DB query failure, not exercised by unit tests
			apierr.WriteError(w, apierr.MsgInternalError, http.StatusInternalServerError)
			return
		}
		if before := settingsOf(current.rule); before != req {
			if err := writeSettings(r.Context(), tx, practiceID, before, req); err != nil {
				// coverage:ignore reason: DB query failure, not exercised by unit tests
				apierr.WriteError(w, apierr.MsgInternalError, http.StatusInternalServerError)
				return
			}
		}
		apierr.WriteJSON(w, http.StatusOK, req)
	})
}

func settingsOf(r Rule) Settings {
	return Settings{StartRule: r.Start, StartWeek: r.Week, GraceDays: r.GraceDays}
}

// writeSettings persists the rule and records who changed it and when,
// on one path so the row and its Activity entry cannot disagree.
func writeSettings(ctx context.Context, tx *sql.Tx, practiceID string, before, after Settings) error {
	if _, err := tx.ExecContext(ctx,
		`UPDATE practices SET on_call_start_rule = $2, on_call_start_week = $3, on_call_grace_days = $4 WHERE id = $1`,
		practiceID, string(after.StartRule), after.StartWeek, after.GraceDays,
	); err != nil {
		// coverage:ignore reason: DB query failure, not exercised by unit tests
		return fmt.Errorf("oncall: write practice on-call rule: %w", err)
	}
	actor, _ := staffauth.StaffID(ctx)
	return record(ctx, tx, activity.SubjectPractice, practiceID, practiceID, actor,
		actionPracticeOnCallChanged, map[string]Settings{diffBefore: before, diffAfter: after})
}
