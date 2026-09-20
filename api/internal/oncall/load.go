package oncall

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"doula-cloud/api/internal/activity"
	"doula-cloud/api/internal/client"
	"doula-cloud/api/internal/ianazone"
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
}

// engagementsSelect reads every active birth Engagement at the Practice
// that has at least one open, granted Attachment, one row per such
// Attachment. Those are the only Engagements that can have a window
// (DeriveWindow's own first three refusals), so nothing that can never
// be on the roster is read at all.
//
// Performance: bounded by the Practice's live births rather than by its
// history, since status = 'active' excludes every completed Engagement;
// at a fourteen-doula agency that is on the order of a hundred rows. The
// window is then derived in Go, because a gestational week, a grant
// date and a grace do not reduce to one indexable column.
//
// $2 is the contractor narrowing and $3 the one-Engagement narrowing,
// each "NULL, or it matches" -- the same NULL-able parameter form
// visit.scheduleSelect uses, so no clause is assembled at run time.
const engagementsSelect = `SELECT e.id, cl.given_name, cl.preferred_name,
	       e.due_date::text, e.pregnancy_ended_on::text,
	       e.on_call_start_rule::text, e.on_call_start_week,
	       ea.staff_id, s.name, ea.attached_at, ea.on_call_from::text, ea.on_call_to::text
	  FROM engagements e
	  JOIN clients cl ON cl.id = e.client_id
	  JOIN engagement_attachments ea
	    ON ea.engagement_id = e.id AND ea.origin = 'granted' AND ea.ended_at IS NULL
	  LEFT JOIN staff s ON s.id = ea.staff_id
	 WHERE e.practice_id = $1
	   AND e.status = 'active'
	   AND e.kind = 'birth'
	   AND ($2::uuid IS NULL OR EXISTS (
	         SELECT 1 FROM engagement_attachments mine
	          WHERE mine.engagement_id = e.id AND mine.staff_id = $2::uuid
	            AND mine.origin = 'granted' AND mine.ended_at IS NULL))
	   AND ($3::uuid IS NULL OR e.id = $3::uuid)
	 ORDER BY e.id, ea.attached_at, ea.staff_id`

// loadEngagements reads and derives every Engagement filter admits, in
// no particular order.
func loadEngagements(ctx context.Context, tx *sql.Tx, practiceID string, settings practiceSettings, filter engagementsFilter) ([]engagementOnCall, error) {
	rows, err := tx.QueryContext(ctx, engagementsSelect,
		practiceID, nullableID(filter.contractorStaffID), nullableID(filter.engagementID))
	if err != nil {
		// coverage:ignore reason: DB query failure, not exercised by unit tests
		return nil, fmt.Errorf("oncall: list on-call engagements: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var out []engagementOnCall
	for rows.Next() {
		var (
			id, givenName            string
			preferredName, staffName sql.NullString
			dueDate, endedOn         sql.NullString
			overrideRule             sql.NullString
			overrideWeek             sql.NullInt16
			doula                    attachedDoula
			narrowFrom, narrowTo     sql.NullString
		)
		if err := rows.Scan(&id, &givenName, &preferredName, &dueDate, &endedOn,
			&overrideRule, &overrideWeek,
			&doula.staffID, &staffName, &doula.attachedAt, &narrowFrom, &narrowTo); err != nil {
			// coverage:ignore reason: row scan failure, not exercised by unit tests
			return nil, fmt.Errorf("oncall: scan on-call engagement: %w", err)
		}
		doula.name = staffName.String
		if !staffName.Valid {
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
				clientName: client.PreferredName(givenName, preferredName.String),
				dueDate:    nullString(dueDate),
				rule:       settings.rule.Resolve(nullString(overrideRule), week),
			})
			// The first row of each Engagement carries its earliest
			// Attachment (the ORDER BY), which is what the grant-date
			// rule opens on.
			granted := doula.attachedAt.In(settings.zone).Format(dateLayout)
			current := &out[len(out)-1]
			current.window, current.reason = DeriveWindow(WindowInput{
				Kind:             "birth",
				Status:           "active",
				DueDate:          current.dueDate,
				PregnancyEndedOn: nullString(endedOn),
				FirstGrantedOn:   &granted,
				Rule:             current.rule,
			})
		}
		current := &out[len(out)-1]
		current.doulas = append(current.doulas, doula)
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
	Reason            *string   `json:"reason"`
	CoveringStaffID   *string   `json:"coveringStaffId"`
	CoveringStaffName *string   `json:"coveringStaffName"`
}

// liveGapsSelect reads every live gap on a set of Engagements that
// overlaps a half-open instant range. engagement_coverage_gaps_live
// (00114) serves it: its leading column is the ANY($1) lookup and its
// predicate is this query's cleared_at clause.
const liveGapsSelect = `SELECT g.id, g.engagement_id, g.staff_id, s.name, g.starts_at, g.ends_at,
	       g.reason, g.covering_staff_id, cs.name
	  FROM engagement_coverage_gaps g
	  LEFT JOIN staff s ON s.id = g.staff_id
	  LEFT JOIN staff cs ON cs.id = g.covering_staff_id
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
		var g Gap
		var staffName, reason, coveringID, coveringName sql.NullString
		if err := rows.Scan(&g.ID, &g.EngagementID, &g.StaffID, &staffName, &g.StartsAt, &g.EndsAt,
			&reason, &coveringID, &coveringName); err != nil {
			// coverage:ignore reason: row scan failure, not exercised by unit tests
			return nil, fmt.Errorf("oncall: scan live gap: %w", err)
		}
		g.StaffName = displayName(staffName)
		g.Reason = nullString(reason)
		g.CoveringStaffID = nullString(coveringID)
		if g.CoveringStaffID != nil {
			name := displayName(coveringName)
			g.CoveringStaffName = &name
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
