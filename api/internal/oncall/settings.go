package oncall

import (
	"context"
	"database/sql"
	"fmt"
	"net/http"
	"time"

	"doula-cloud/api/internal/activity"
	"doula-cloud/api/internal/activitypage"
	"doula-cloud/api/internal/apierr"
	"doula-cloud/api/internal/staffauth"
)

// actionPracticeOnCallChanged records a change to a Practice's on-call
// rule -- Practice-scoped and plain-string, the shape
// practicetimezone's actionTimezoneChanged uses for its sibling setting.
const actionPracticeOnCallChanged = string(activity.ActionPracticeOnCallChanged)

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

// SettingsResponse is the rule and who last changed it. LastChanged is
// null while nobody has changed it since the Practice was set up.
type SettingsResponse struct {
	Settings
	LastChanged *LastChanged `json:"lastChanged"`
}

// LastChanged is the latest recorded change to the rule: who, and when.
type LastChanged struct {
	StaffName string    `json:"staffName"`
	At        time.Time `json:"at"`
}

func lastChanged(ctx context.Context, tx *sql.Tx, practiceID string) (*LastChanged, error) {
	stamp, err := activitypage.Latest(ctx, tx, activitypage.LatestQuery{
		PracticeID: practiceID, SubjectKind: activity.SubjectPractice, SubjectID: practiceID, Action: actionPracticeOnCallChanged,
	})
	if err != nil {
		// coverage:ignore reason: DB query failure, not exercised by unit tests
		return nil, fmt.Errorf("oncall: read when the rule last changed: %w", err)
	}
	if stamp == nil {
		return nil, nil //nolint:nilnil // not changed since the Practice was set up is an answer, not a failure
	}
	return &LastChanged{StaffName: stamp.By, At: stamp.At}, nil
}

// validateSettings is the rule's own refusals, shared by the write and the
// preview of it so the two say the same words under the same keys.
func validateSettings(s Settings) map[string]string {
	details := map[string]string{}
	if s.StartRule != StartGestationalWeek && s.StartRule != StartAttachmentGranted {
		details["startRule"] = MsgUnknownStartRule
	}
	if s.StartWeek < minStartWeek || s.StartWeek > maxStartWeek {
		details["startWeek"] = MsgStartWeekRange
	}
	if s.GraceDays < 0 || s.GraceDays > maxGraceDays {
		details["graceDays"] = MsgGraceDaysRange
	}
	return details
}

// GetSettingsHandler reads the Practice's on-call rule. Owner and Admin
// only, declared at the mount, the pair that may state it -- the same
// authority practicetimezone gives the zone. Every Staff member still
// meets the rule's effect, on the roster and on the Engagement panel.
//
// Must be mounted behind staffauth.Middleware.
func GetSettingsHandler() http.Handler {
	return staffauth.Handler(func(w http.ResponseWriter, r *http.Request, c staffauth.Call) {
		s, err := loadPracticeSettings(r.Context(), c.Tx, c.PracticeID)
		if err != nil {
			// coverage:ignore reason: DB query failure, not exercised by unit tests
			apierr.WriteError(w, apierr.MsgInternalError, http.StatusInternalServerError)
			return
		}
		writeSettingsResponse(w, r, c, settingsOf(s.rule))
	})
}

func writeSettingsResponse(w http.ResponseWriter, r *http.Request, c staffauth.Call, settings Settings) {
	changed, err := lastChanged(r.Context(), c.Tx, c.PracticeID)
	if err != nil {
		// coverage:ignore reason: DB query failure, not exercised by unit tests
		apierr.WriteError(w, apierr.MsgInternalError, http.StatusInternalServerError)
		return
	}
	apierr.WriteJSON(w, http.StatusOK, SettingsResponse{Settings: settings, LastChanged: changed})
}

// PutSettingsHandler states a Practice's on-call rule. Idempotent by
// construction, as practicetimezone.PutHandler is: the same body writes
// nothing and records nothing new. Changing it moves every window at
// once, because nothing stores a window.
//
// Must be mounted behind staffauth.Middleware.
func PutSettingsHandler() http.Handler {
	return staffauth.Handler(func(w http.ResponseWriter, r *http.Request, c staffauth.Call) {
		var req Settings
		if !apierr.DecodeJSON(w, r, &req) {
			return
		}
		if details := validateSettings(req); len(details) > 0 {
			apierr.Write(w, http.StatusBadRequest, apierr.CodeInvalidArgument, "The on-call rule could not be saved.", details)
			return
		}

		current, err := loadPracticeSettings(r.Context(), c.Tx, c.PracticeID)
		if err != nil {
			// coverage:ignore reason: DB query failure, not exercised by unit tests
			apierr.WriteError(w, apierr.MsgInternalError, http.StatusInternalServerError)
			return
		}
		if before := settingsOf(current.rule); before != req {
			if err := writeSettings(r.Context(), c.Tx, c.PracticeID, c.StaffID, before, req); err != nil {
				// coverage:ignore reason: DB query failure, not exercised by unit tests
				apierr.WriteError(w, apierr.MsgInternalError, http.StatusInternalServerError)
				return
			}
		}
		writeSettingsResponse(w, r, c, req)
	})
}

func settingsOf(r Rule) Settings {
	return Settings{StartRule: r.Start, StartWeek: r.Week, GraceDays: r.GraceDays}
}

// writeSettings persists the rule and records who changed it and when,
// on one path so the row and its Activity entry cannot disagree.
func writeSettings(ctx context.Context, tx *sql.Tx, practiceID, actor string, before, after Settings) error {
	if _, err := tx.ExecContext(ctx,
		`UPDATE practices SET on_call_start_rule = $2, on_call_start_week = $3, on_call_grace_days = $4 WHERE id = $1`,
		practiceID, string(after.StartRule), after.StartWeek, after.GraceDays,
	); err != nil {
		// coverage:ignore reason: DB query failure, not exercised by unit tests
		return fmt.Errorf("oncall: write practice on-call rule: %w", err)
	}
	return record(ctx, tx, activity.SubjectPractice, practiceID, practiceID, actor,
		actionPracticeOnCallChanged, map[string]Settings{diffBefore: before, diffAfter: after})
}
