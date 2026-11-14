package oncall

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"doula-cloud/api/internal/activity"
	"doula-cloud/api/internal/ianazone"
	"doula-cloud/api/internal/personname"
)

// practiceSettings is everything the Practice itself contributes to a
// window: its rule and the zone its calendar days are in (ADR-0036).
type practiceSettings struct {
	rule Rule
	zone *time.Location
}

// loadPracticeSettings reads a Practice's on-call rule and zone in one
// query. practices carries no RLS, so this reads by id on the request's
// own tx, with the caller's practice id and never one off the body.
func loadPracticeSettings(ctx context.Context, tx *sql.Tx, practiceID string) (practiceSettings, error) {
	var zoneName, start string
	var week, grace int
	if err := tx.QueryRowContext(ctx,
		`SELECT timezone, on_call_start_rule, on_call_start_week, on_call_grace_days
		   FROM practices WHERE id = $1`, practiceID,
	).Scan(&zoneName, &start, &week, &grace); err != nil {
		// coverage:ignore reason: DB query failure, not exercised by unit tests
		return practiceSettings{}, fmt.Errorf("oncall: read practice settings: %w", err)
	}
	zone, err := ianazone.Parse(zoneName)
	if err != nil {
		// coverage:ignore reason: practices.timezone is only ever written through ianazone.Parse
		return practiceSettings{}, fmt.Errorf("oncall: load practice timezone: %w", err)
	}
	return practiceSettings{rule: Rule{Start: StartRule(start), Week: week, GraceDays: grace}, zone: zone}, nil
}

// attachedDoula is one open, granted Attachment on an Engagement: who
// she is, when she was attached, and her narrowing if she has one.
type attachedDoula struct {
	staffID    string
	name       string
	attachedAt time.Time
	from, to   *string
}

// engagementOnCall is one Engagement as the window derivation needs it,
// plus the Client's name the roster labels it with and nothing else of
// hers.
type engagementOnCall struct {
	id         string
	clientName string
	dueDate    *string
	rule       Rule
	window     *Window
	reason     NoWindowReason
	doulas     []attachedDoula
	ownStart   *OwnStart
	noDoula    bool
}

// OwnStart is the start rule one Engagement carries of its own, over the
// Practice's. The week is absent under the grant-day rule.
type OwnStart struct {
	StartRule StartRule `json:"startRule"`
	StartWeek *int      `json:"startWeek,omitempty"`
}

func ownStartOf(rule sql.NullString, week sql.NullInt16) *OwnStart {
	if !rule.Valid {
		return nil
	}
	own := &OwnStart{StartRule: StartRule(rule.String)}
	if week.Valid {
		w := int(week.Int16)
		own.StartWeek = &w
	}
	return own
}

// engagementsFilter narrows loadEngagements. Both fields are the empty
// string when unused.
type engagementsFilter struct {
	// contractorStaffID is set only for an ambient contractor Doula, and
	// confines the read to Engagements she holds an open, granted
	// Attachment on -- in the query, so nothing else is ever read.
	contractorStaffID string
	// engagementID confines the read to one Engagement.
	engagementID string
	// unattendedOn, when set, also reads the live births nobody holds a
	// granted Attachment on, each derived as though a Doula were put on it
	// that day (YYYY-MM-DD): the days a birth with no Doula yet would be
	// on call.
	unattendedOn string
}

// engagementsSelect reads every live (not completed) birth Engagement at the Practice
// that has at least one open, granted Attachment, one row per such
// Attachment. Those are the only Engagements that can have a window
// (DeriveWindow's own first three refusals), so nothing that can never
// be on the roster is read at all.
//
// Performance: bounded by the Practice's live births rather than by its
// history, since status <> 'completed' excludes every completed Engagement;
// at a fourteen-doula agency that is on the order of a hundred rows. The
// window is then derived in Go, because a gestational week, a grant
// date and a grace do not reduce to one indexable column.
//
// $2 is the contractor narrowing and $3 the one-Engagement narrowing,
// each "NULL, or it matches" -- the same NULL-able parameter form
// visit.scheduleSelect uses, so no clause is assembled at run time. $4
// admits the births nobody is granted on, one row each with a NULL
// Attachment; a contractor's narrowing refuses those rows by itself.
const engagementsSelect = `SELECT e.id, e.status::text, cl.given_name, cl.preferred_name, cl.family_name,
	       e.due_date::text, e.pregnancy_ended_on::text,
	       e.on_call_start_rule::text, e.on_call_start_week,
	       ea.staff_id, s.name, ea.attached_at, ea.on_call_from::text, ea.on_call_to::text
	  FROM engagements e
	  JOIN clients cl ON cl.id = e.client_id
	  LEFT JOIN engagement_attachments ea
	    ON ea.engagement_id = e.id AND ea.state = 'granted'
	  LEFT JOIN staff s ON s.id = ea.staff_id
	 WHERE e.practice_id = $1
	   AND ($4::boolean OR ea.staff_id IS NOT NULL)
	   AND e.status <> 'completed'
	   AND e.kind = 'birth'
	   AND ($2::uuid IS NULL OR EXISTS (
	         SELECT 1 FROM engagement_attachments mine
	          WHERE mine.engagement_id = e.id AND mine.staff_id = $2::uuid
	            AND mine.state = 'granted'))
	   AND ($3::uuid IS NULL OR e.id = $3::uuid)
	 ORDER BY e.id, ea.attached_at, ea.staff_id`

// loadEngagements reads and derives every Engagement filter admits, in
// no particular order.
func loadEngagements(ctx context.Context, tx *sql.Tx, practiceID string, settings practiceSettings, filter engagementsFilter) ([]engagementOnCall, error) {
	rows, err := tx.QueryContext(ctx, engagementsSelect,
		practiceID, nullableID(filter.contractorStaffID), nullableID(filter.engagementID), filter.unattendedOn != "")
	if err != nil {
		// coverage:ignore reason: DB query failure, not exercised by unit tests
		return nil, fmt.Errorf("oncall: list on-call engagements: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var out []engagementOnCall
	for rows.Next() {
		var (
			id, status, givenName     string
			preferredName, familyName sql.NullString
			staffName                 sql.NullString
			dueDate, endedOn          sql.NullString
			overrideRule              sql.NullString
			overrideWeek              sql.NullInt16
			doula                     attachedDoula
			staffID                   sql.NullString
			attachedAt                sql.NullTime
			narrowFrom, narrowTo      sql.NullString
		)
		if err := rows.Scan(&id, &status, &givenName, &preferredName, &familyName, &dueDate, &endedOn,
			&overrideRule, &overrideWeek,
			&staffID, &staffName, &attachedAt, &narrowFrom, &narrowTo); err != nil {
			// coverage:ignore reason: row scan failure, not exercised by unit tests
			return nil, fmt.Errorf("oncall: scan on-call engagement: %w", err)
		}
		doula.staffID = staffID.String
		doula.attachedAt = attachedAt.Time
		doula.name = staffName.String
		if staffID.Valid && !staffName.Valid {
			// coverage:ignore reason: an open Attachment ends with its Membership, so its staff row is always visible
			doula.name = activity.DepartedStaffName
		}
		doula.from = nullString(narrowFrom)
		doula.to = nullString(narrowTo)

		if len(out) == 0 || out[len(out)-1].id != id {
			var week *int16
			if overrideWeek.Valid {
				week = &overrideWeek.Int16
			}
			out = append(out, engagementOnCall{
				id:         id,
				clientName: personname.Full(givenName, preferredName.String, familyName.String),
				dueDate:    nullString(dueDate),
				rule:       settings.rule.Resolve(nullString(overrideRule), week),
				ownStart:   ownStartOf(overrideRule, overrideWeek),
				noDoula:    !staffID.Valid,
			})
			// The first row of each Engagement carries its earliest
			// Attachment (the ORDER BY), which is what the grant-date
			// rule opens on.
			granted := doula.attachedAt.In(settings.zone).Format(dateLayout)
			if !staffID.Valid {
				granted = filter.unattendedOn
			}
			current := &out[len(out)-1]
			current.window, current.reason = DeriveWindow(WindowInput{
				Kind:             KindBirth,
				Status:           status,
				DueDate:          current.dueDate,
				PregnancyEndedOn: nullString(endedOn),
				FirstGrantedOn:   &granted,
				Rule:             current.rule,
			})
			if current.noDoula && current.reason == NoWindowNoDueDate {
				current.reason = NoWindowNoDueDateNoDoula
			}
		}
		current := &out[len(out)-1]
		if staffID.Valid {
			current.doulas = append(current.doulas, doula)
		}
	}
	if err := rows.Err(); err != nil {
		// coverage:ignore reason: row iteration failure, not exercised by unit tests
		return nil, fmt.Errorf("oncall: iterate on-call engagements: %w", err)
	}
	return out, nil
}

// Gap is one live coverage gap as the roster and the Engagement panel
// show it.
type Gap struct {
	ID                string    `json:"id"`
	EngagementID      string    `json:"engagementId"`
	StaffID           string    `json:"staffId"`
	StaffName         string    `json:"staffName"`
	StartsAt          time.Time `json:"startsAt"`
	EndsAt            time.Time `json:"endsAt"`
	Reason            *string   `json:"reason,omitempty"`
	CoveringStaffID   *string   `json:"coveringStaffId,omitempty"`
	CoveringStaffName *string   `json:"coveringStaffName,omitempty"`
}

// liveGapsSelect reads every live gap on a set of Engagements that
// overlaps a half-open instant range. engagement_coverage_gaps_live
// (00114) serves it: its leading column is the ANY($1) lookup and its
// predicate is this query's cleared_at clause.
const liveGapsSelect = `SELECT ` + gapColumns + ` ` + gapJoins + `
	 WHERE g.engagement_id = ANY($1::uuid[])
	   AND g.cleared_at IS NULL
	   AND g.starts_at < $3
	   AND g.ends_at > $2
	 ORDER BY g.starts_at, g.id`

// loadLiveGaps reads the live gaps on engagementIDs that overlap
// [from, to), grouped by Engagement.
func loadLiveGaps(ctx context.Context, tx *sql.Tx, engagementIDs []string, from, to time.Time) (map[string][]Gap, error) {
	out := map[string][]Gap{}
	if len(engagementIDs) == 0 {
		return out, nil
	}
	rows, err := tx.QueryContext(ctx, liveGapsSelect, engagementIDs, from, to)
	if err != nil {
		// coverage:ignore reason: DB query failure, not exercised by unit tests
		return nil, fmt.Errorf("oncall: list live gaps: %w", err)
	}
	defer func() { _ = rows.Close() }()
	for rows.Next() {
		g, err := scanGap(rows)
		if err != nil {
			// coverage:ignore reason: row scan failure, not exercised by unit tests
			return nil, err
		}
		out[g.EngagementID] = append(out[g.EngagementID], g)
	}
	if err := rows.Err(); err != nil {
		// coverage:ignore reason: row iteration failure, not exercised by unit tests
		return nil, fmt.Errorf("oncall: iterate live gaps: %w", err)
	}
	return out, nil
}

// displayName is a Staff name, or the word a Practice reads for a
// colleague whose row it can no longer see (activity.DepartedStaffName).
func displayName(name sql.NullString) string {
	if !name.Valid {
		return activity.DepartedStaffName
	}
	return name.String
}

func nullString(s sql.NullString) *string {
	if !s.Valid {
		return nil
	}
	return &s.String
}

// nullableID turns "none" into a real SQL NULL for the `$n::uuid IS NULL`
// halves above; an empty string would be a malformed uuid, not an absent
// one.
func nullableID(id string) any {
	if id == "" {
		return nil
	}
	return id
}
