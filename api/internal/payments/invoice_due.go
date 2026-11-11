package payments

import (
	"context"
	"database/sql"
	"fmt"
	"net/http"
	"time"

	"doula-cloud/api/internal/apierr"
	"doula-cloud/api/internal/oncall"
)

const (
	fieldDueOn           = "dueOn"
	fieldGestationalWeek = "gestationalWeek"

	minInvoiceWeek = 20
	maxInvoiceWeek = 42
)

const (
	msgDueOnMalformed        = "Enter the due day as a real date, like 2027-04-23."
	msgDueOnPast             = "The due day cannot be before today."
	msgDueDayChoiceAmbiguous = "Give the due day as a date or as a week of pregnancy, not both."
	msgWeekOutOfRange        = "Enter a week of pregnancy from 20 to 42."
	msgWeekNeedsBirthDueDate = "A week of pregnancy needs a birth Engagement with a due date. Give a date instead."

	detailDueOnMalformed        = "Due day must be a real date, like 2027-04-23"
	detailDueOnPast             = "Due day cannot be before today"
	detailDueDayChoiceAmbiguous = "Due day cannot be a date and a week of pregnancy at once"
	detailWeekOutOfRange        = "Week of pregnancy must be from 20 to 42"
	detailWeekNeedsBirthDueDate = "Week of pregnancy needs a birth Engagement with a due date"
)

type dueDayRefusal struct {
	message string
	field   string
	detail  string
}

func (rf *dueDayRefusal) write(w http.ResponseWriter) {
	apierr.Write(w, http.StatusBadRequest, apierr.CodeInvalidArgument, rf.message, map[string]string{rf.field: rf.detail})
}

// requestedDueDay reads the due day a request names, as a calendar day at
// UTC midnight, or nil when it names none and the Practice's payment terms
// decide. A week of pregnancy resolves from the Engagement's due date by
// oncall.WeekStart, the rule the on-call window counts with. A resolved
// day, like a typed one, may not be before today in the Practice's zone.
func requestedDueDay(ctx context.Context, tx *sql.Tx, engagementID string, req CreateInvoiceRequest, now time.Time, zone *time.Location) (*time.Time, *dueDayRefusal, error) {
	switch {
	case req.DueOn != nil && req.GestationalWeek != nil:
		return nil, &dueDayRefusal{msgDueDayChoiceAmbiguous, fieldDueOn, detailDueDayChoiceAmbiguous}, nil
	case req.DueOn != nil:
		day, isDay := parseDay(*req.DueOn)
		if !isDay {
			return nil, &dueDayRefusal{msgDueOnMalformed, fieldDueOn, detailDueOnMalformed}, nil
		}
		return notBeforeToday(day, now, zone)
	case req.GestationalWeek != nil:
		week := *req.GestationalWeek
		if week < minInvoiceWeek || week > maxInvoiceWeek {
			return nil, &dueDayRefusal{msgWeekOutOfRange, fieldGestationalWeek, detailWeekOutOfRange}, nil
		}
		due, hasDue, err := birthDueDate(ctx, tx, engagementID)
		if err != nil {
			// coverage:ignore reason: DB query failure, not exercised by unit tests
			return nil, nil, err
		}
		if !hasDue {
			return nil, &dueDayRefusal{msgWeekNeedsBirthDueDate, fieldGestationalWeek, detailWeekNeedsBirthDueDate}, nil
		}
		return notBeforeToday(oncall.WeekStart(due, week), now, zone)
	}
	return nil, nil, nil
}

func notBeforeToday(day, now time.Time, zone *time.Location) (*time.Time, *dueDayRefusal, error) {
	year, month, d := now.In(zone).Date()
	if day.Before(time.Date(year, month, d, 0, 0, 0, 0, time.UTC)) {
		return nil, &dueDayRefusal{msgDueOnPast, fieldDueOn, detailDueOnPast}, nil
	}
	return &day, nil, nil
}

func parseDay(value string) (time.Time, bool) {
	day, err := time.Parse(time.DateOnly, value)
	return day, err == nil
}

// birthDueDate is the Engagement's due date, at UTC midnight, and whether
// it has one: only a birth Engagement with a due date does.
func birthDueDate(ctx context.Context, tx *sql.Tx, engagementID string) (time.Time, bool, error) {
	var due sql.NullTime
	var kind string
	err := tx.QueryRowContext(ctx, `SELECT kind::text, due_date FROM engagements WHERE id = $1`, engagementID).Scan(&kind, &due)
	if err != nil {
		// coverage:ignore reason: DB query failure, not exercised by unit tests
		return time.Time{}, false, fmt.Errorf("payments: read engagement due date: %w", err)
	}
	if kind != oncall.KindBirth || !due.Valid {
		return time.Time{}, false, nil
	}
	return time.Date(due.Time.Year(), due.Time.Month(), due.Time.Day(), 0, 0, 0, 0, time.UTC), true, nil
}

// dueAtOnDay is the stored instant for an Invoice due on day: noon that
// day in the Practice's zone, so the day reads the same there and in any
// zone Stripe is likely to print it in. An Invoice raised on its own due
// day after noon falls due at the last second of that day instead, still
// that day and still after the instant it was raised. A whole second, so
// the stored and the sent instants are one.
func dueAtOnDay(day, raisedAt time.Time, zone *time.Location) time.Time {
	year, month, d := day.Date()
	if noon := time.Date(year, month, d, 12, 0, 0, 0, zone); noon.After(raisedAt) {
		return noon
	}
	return time.Date(year, month, d, 23, 59, 59, 0, zone)
}
