package client

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"
)

// EngagementSummary is one row of a matched Client's Engagement history --
// what the save-time prompt and the search result print alongside her
// name and history, unrestricted inside the Practice, per ADR-0017.
type EngagementSummary struct {
	EngagementID string    `json:"engagementId"`
	Kind         string    `json:"kind"`
	Status       string    `json:"status"`
	CreatedAt    time.Time `json:"createdAt"`
}

// Match is one Client the match query found: her record plus her
// Engagement history, unrestricted inside the Practice -- what the
// save-time prompt and the search screen both need to print.
type Match struct {
	Record
	Engagements []EngagementSummary `json:"engagements"`
}

// FindMatches runs ADR-0017's match query -- name, date of birth, email
// and phone, scoped to practiceID -- and is reused everywhere that query
// runs: the search that fronts intake, the lookup-before-insert check at
// create, and the re-run at edit. givenName and familyName each match
// given_name, family_name or preferred_name case-insensitively, by
// substring, independently of one another -- a search box that only has
// one free-text field can pass the same text as both, which still
// matches any of the three name columns; a structured create/edit
// request passes its own two fields apart. dateOfBirth (a "YYYY-MM-DD"
// string) and email (case-insensitively) match exactly; phone matches
// exactly. Blank fields are ignored. If every field is blank, FindMatches
// returns no rows rather than the whole Practice -- callers with nothing
// to match on should not call this at all. excludeClientID, when
// non-empty, omits that Client's own row, so an edit's re-run can't
// match itself.
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
		`SELECT `+recordColumns+`
		 FROM clients
		 WHERE practice_id = $1
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
		rec, err := scanRecord(rows.Scan)
		if err != nil {
			// coverage:ignore reason: row scan failure, not exercised by unit tests
			return nil, fmt.Errorf("client: scan match: %w", err)
		}
		matches = append(matches, Match{Record: rec})
	}
	if err := rows.Err(); err != nil {
		// coverage:ignore reason: row iteration failure, not exercised by unit tests
		return nil, fmt.Errorf("client: iterate matches: %w", err)
	}

	for i := range matches {
		engagements, err := listEngagementsForClient(ctx, tx, matches[i].ID)
		if err != nil {
			// coverage:ignore reason: DB query failure, not exercised by unit tests
			return nil, err
		}
		matches[i].Engagements = engagements
	}
	return matches, nil
}

// listEngagementsForClient reads clientID's Engagement history, oldest
// first -- shared by FindMatches (the save-time prompt's "history") and
// DetailHandler (her record's own Engagements past and present).
func listEngagementsForClient(ctx context.Context, tx *sql.Tx, clientID string) ([]EngagementSummary, error) {
	rows, err := tx.QueryContext(ctx,
		`SELECT id, kind::text, status, created_at FROM engagements WHERE client_id = $1 ORDER BY created_at`,
		clientID,
	)
	if err != nil {
		// coverage:ignore reason: DB query failure, not exercised by unit tests
		return nil, fmt.Errorf("client: list engagements for client: %w", err)
	}
	defer func() { _ = rows.Close() }()

	list := []EngagementSummary{}
	for rows.Next() {
		var e EngagementSummary
		if err := rows.Scan(&e.EngagementID, &e.Kind, &e.Status, &e.CreatedAt); err != nil {
			// coverage:ignore reason: row scan failure, not exercised by unit tests
			return nil, fmt.Errorf("client: scan engagement: %w", err)
		}
		list = append(list, e)
	}
	if err := rows.Err(); err != nil {
		// coverage:ignore reason: row iteration failure, not exercised by unit tests
		return nil, fmt.Errorf("client: iterate engagements: %w", err)
	}
	return list, nil
}
