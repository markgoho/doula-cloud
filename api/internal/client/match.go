package client

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	"doula-cloud/api/internal/activity"
)

// EngagementSummary is one row of a matched Client's Engagement history --
// what the save-time prompt and the search result print alongside her
// name and history, unrestricted inside the Practice, per ADR-0017.
//
// EndedAt is when a completed Engagement was completed, read from its
// latest engagement_completed entry in the activity ledger (ADR-0022:
// the ledger is where that fact is recorded; engagements carries no
// column for it). Nil while the Engagement is open, and nil for a
// completed one with no such entry, which only a seeded row can be.
// #2143: it is half of what tells two Clients with one name apart --
// when each was last cared for.
type EngagementSummary struct {
	EngagementID string     `json:"engagementId"`
	Kind         string     `json:"kind"`
	Status       string     `json:"status"`
	CreatedAt    time.Time  `json:"createdAt"`
	EndedAt      *time.Time `json:"endedAt,omitempty"`
}

// Match is one Client the match query found: her record, when it was
// added, and her Engagement history, unrestricted inside the Practice --
// what the save-time prompt and the search screen both need to print.
// CreatedAt sits on Match rather than Record because Record is also the
// body an edit PUTs, and when a row was made is never a caller's to
// write (#2143: the other half of telling two namesakes apart).
type Match struct {
	Record
	CreatedAt   time.Time           `json:"createdAt"`
	Engagements []EngagementSummary `json:"engagements"`
}

// FindMatches runs the search that fronts intake's substring recall --
// name, date of birth, email and phone, scoped to practiceID. ADR-0017's
// amendment moved the write paths (create's lookup-before-insert, edit's
// re-run) off this query and onto FindCollisions' exact-key predicate,
// leaving FindMatches to the one caller substring recall is right for:
// SearchHandler. givenName and familyName each match given_name,
// family_name or preferred_name case-insensitively, by substring,
// independently of one another -- a search box that only has one
// free-text field can pass the same text as both, which still matches
// any of the three name columns. dateOfBirth (a "YYYY-MM-DD" string) and
// email (case-insensitively) match exactly; phone matches exactly. Blank
// fields are ignored. If every field is blank, FindMatches returns no
// rows rather than the whole Practice -- callers with nothing to match
// on should not call this at all. excludeClientID, when non-empty, omits
// that Client's own row -- kept for callers that still want it, though
// SearchHandler itself never passes one. A row already merged into
// another is never a candidate: it is a tombstone, excluded from search
// the same way it is excluded from the Clients list.
func FindMatches(ctx context.Context, tx *sql.Tx, practiceID, givenName, familyName, dateOfBirth, email, phone, excludeClientID string) ([]Match, error) {
	givenName = strings.TrimSpace(givenName)
	familyName = strings.TrimSpace(familyName)
	dateOfBirth = strings.TrimSpace(dateOfBirth)
	email = strings.TrimSpace(email)
	phone = strings.TrimSpace(phone)
	if givenName == "" && familyName == "" && dateOfBirth == "" && email == "" && phone == "" {
		return nil, nil
	}

	rows, err := tx.QueryContext(ctx,
		`SELECT `+recordColumns+`, created_at
		 FROM clients
		 WHERE practice_id = $1
		   AND merged_into IS NULL
		   AND (NULLIF($7, '') IS NULL OR id <> NULLIF($7, '')::uuid)
		   AND (
		       ($2 <> '' AND (given_name ILIKE '%' || $2 || '%' OR family_name ILIKE '%' || $2 || '%' OR preferred_name ILIKE '%' || $2 || '%'))
		    OR ($3 <> '' AND (given_name ILIKE '%' || $3 || '%' OR family_name ILIKE '%' || $3 || '%' OR preferred_name ILIKE '%' || $3 || '%'))
		    OR (NULLIF($4, '')::date IS NOT NULL AND date_of_birth = NULLIF($4, '')::date)
		    OR ($5 <> '' AND lower(email) = lower($5))
		    OR ($6 <> '' AND phone = $6)
		   )
		 ORDER BY given_name`,
		practiceID, givenName, familyName, dateOfBirth, email, phone, excludeClientID,
	)
	if err != nil {
		// coverage:ignore reason: DB query failure, not exercised by unit tests
		return nil, fmt.Errorf("client: find matches: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var matches []Match
	for rows.Next() {
		var createdAt time.Time
		// created_at rides as the 14th column, appended here the way
		// FindCollisions appends it, rather than taught to scanRecord.
		rec, err := scanRecord(func(dest ...any) error {
			return rows.Scan(append(dest, &createdAt)...)
		})
		if err != nil {
			// coverage:ignore reason: row scan failure, not exercised by unit tests
			return nil, fmt.Errorf("client: scan match: %w", err)
		}
		matches = append(matches, Match{Record: rec, CreatedAt: createdAt})
	}
	if err := rows.Err(); err != nil {
		// coverage:ignore reason: row iteration failure, not exercised by unit tests
		return nil, fmt.Errorf("client: iterate matches: %w", err)
	}

	for i := range matches {
		engagements, err := ListEngagementsForClient(ctx, tx, matches[i].ID)
		if err != nil {
			// coverage:ignore reason: DB query failure, not exercised by unit tests
			return nil, err
		}
		matches[i].Engagements = engagements
	}
	return matches, nil
}

// ListEngagementsForClient reads clientID's Engagement history, oldest
// first -- shared by FindMatches (the save-time prompt's "history"),
// DetailHandler (her record's own Engagements past and present) and
// engagementrequest.DetailHandler (the approval screen's same list).
// Exported for that third consumer: RLS scopes the read to the caller's
// Practice, so it carries no access rule of its own.
func ListEngagementsForClient(ctx context.Context, tx *sql.Tx, clientID string) ([]EngagementSummary, error) {
	rows, err := tx.QueryContext(ctx,
		`SELECT e.id, e.kind::text, e.status, e.created_at,
		        CASE WHEN e.status = 'completed' THEN (
		            SELECT max(a.created_at) FROM activity a
		             WHERE a.practice_id = e.practice_id
		               AND a.subject_kind = $2
		               AND a.subject_id = e.id
		               AND a.action = $3
		        ) END
		 FROM engagements e
		 WHERE e.client_id = $1
		 ORDER BY e.created_at`,
		clientID, activity.SubjectEngagement, string(activity.ActionEngagementCompleted),
	)
	if err != nil {
		// coverage:ignore reason: DB query failure, not exercised by unit tests
		return nil, fmt.Errorf("client: list engagements for client: %w", err)
	}
	defer func() { _ = rows.Close() }()

	list := []EngagementSummary{}
	for rows.Next() {
		var e EngagementSummary
		var endedAt sql.NullTime
		if err := rows.Scan(&e.EngagementID, &e.Kind, &e.Status, &e.CreatedAt, &endedAt); err != nil {
			// coverage:ignore reason: row scan failure, not exercised by unit tests
			return nil, fmt.Errorf("client: scan engagement: %w", err)
		}
		if endedAt.Valid {
			e.EndedAt = &endedAt.Time
		}
		list = append(list, e)
	}
	if err := rows.Err(); err != nil {
		// coverage:ignore reason: row iteration failure, not exercised by unit tests
		return nil, fmt.Errorf("client: iterate engagements: %w", err)
	}
	return list, nil
}
