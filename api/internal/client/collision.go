package client

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"
)

// Collision is one Client FindCollisions found: her record, her
// Engagement history (the same shape the save-time prompt already
// shows), when her row was made (what breaks a tie between two
// unattached rows -- the older survives a merge), and whether this
// particular hit is a name substitution -- ADR-0017's amendment gate
// one, narrowed to an exact given-and-family match.
type Collision struct {
	Match
	CreatedAt        time.Time `json:"createdAt"`
	NameSubstitution bool      `json:"-"`
}

// FindCollisions runs ADR-0017's amendment predicate -- the one both
// write paths (create and edit) share, replacing FindMatches' substring
// recall on those two paths. Two Clients collide when, comparing trimmed
// and case-insensitively: given name and family name are both exactly
// equal; or email is exactly equal; or phone is exactly equal; or date
// of birth is exactly equal AND the two records share at least one whole
// name word across any of their three name columns.
//
// The shared-word half of the date-of-birth branch is checked in Go, not
// SQL (sharesNameWord below) -- the SQL here fetches every date-of-birth
// match and every exact name/email/phone match in one query, and the
// caller filters a date-of-birth-only hit that shares no word. A bare
// date-of-birth collision between two unrelated names is a coincidence
// and passes silently (ADR-0017's amendment).
//
// A row already merged into another, or already erased, is never a
// candidate: the former is a tombstone excluded from the collision
// predicate by name (ADR-0017's amendment); the latter is never offered
// as a merge target (ADR-0027), and excluding it here means it can never
// reach that offer at all. excludeClientID, when non-empty, omits that
// Client's own row, so an edit's re-run can't match itself. Blank
// familyName/email/phone/dateOfBirth are ignored the same way FindMatches
// ignores them; givenName is never blank at either of this function's two
// callers (normalizeAndValidate already refuses that before create.go or
// edit.go ever reaches this), so unlike FindMatches this function does
// not special-case "every field blank" -- there is no caller with
// nothing to match on.
func FindCollisions(ctx context.Context, tx *sql.Tx, practiceID, givenName, familyName, dateOfBirth, email, phone, excludeClientID string) ([]Collision, error) {
	givenName = strings.TrimSpace(givenName)
	familyName = strings.TrimSpace(familyName)
	dateOfBirth = strings.TrimSpace(dateOfBirth)
	email = strings.TrimSpace(email)
	phone = strings.TrimSpace(phone)

	rows, err := tx.QueryContext(ctx,
		`SELECT `+recordColumns+`, created_at
		 FROM clients
		 WHERE practice_id = $1
		   AND merged_into IS NULL
		   AND erased_at IS NULL
		   AND (NULLIF($7, '') IS NULL OR id <> NULLIF($7, '')::uuid)
		   AND (
		       ($2 <> '' AND $3 <> '' AND lower(trim(given_name)) = lower($2) AND lower(trim(coalesce(family_name, ''))) = lower($3))
		    OR ($4 <> '' AND lower(email) = lower($4))
		    OR ($5 <> '' AND phone = $5)
		    OR (NULLIF($6, '')::date IS NOT NULL AND date_of_birth = NULLIF($6, '')::date)
		   )
		 ORDER BY given_name`,
		practiceID, givenName, familyName, email, phone, dateOfBirth, excludeClientID,
	)
	if err != nil {
		// coverage:ignore reason: DB query failure, not exercised by unit tests
		return nil, fmt.Errorf("client: find collisions: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var candidates []Collision
	for rows.Next() {
		var createdAt time.Time
		// scanRecord builds its own fixed 13-pointer dest slice and calls
		// back into this closure with it; created_at rides as the query's
		// 14th column, appended here rather than taught to scanRecord,
		// which every other caller uses against a 13-column SELECT.
		rec, err := scanRecord(func(dest ...any) error {
			return rows.Scan(append(dest, &createdAt)...)
		})
		if err != nil {
			// coverage:ignore reason: row scan failure, not exercised by unit tests
			return nil, fmt.Errorf("client: scan collision: %w", err)
		}
		candidates = append(candidates, Collision{Match: Match{Record: rec}, CreatedAt: createdAt})
	}
	if err := rows.Err(); err != nil {
		// coverage:ignore reason: row iteration failure, not exercised by unit tests
		return nil, fmt.Errorf("client: iterate collisions: %w", err)
	}

	ourWords := nameWords(givenName, familyName, "")
	filtered := candidates[:0]
	for _, c := range candidates {
		exactName := givenName != "" && familyName != "" &&
			strings.EqualFold(strings.TrimSpace(c.GivenName), givenName) &&
			strings.EqualFold(strings.TrimSpace(c.FamilyName), familyName)
		exactEmail := email != "" && strings.EqualFold(c.Email, email)
		exactPhone := phone != "" && c.Phone == phone
		dobExact := dateOfBirth != "" && c.DateOfBirth == dateOfBirth
		if exactName || exactEmail || exactPhone {
			c.NameSubstitution = exactName
			filtered = append(filtered, c)
			continue
		}
		if dobExact && sharesNameWord(ourWords, nameWords(c.GivenName, c.FamilyName, c.PreferredName)) {
			filtered = append(filtered, c)
		}
	}
	candidates = filtered

	for i := range candidates {
		engagements, err := ListEngagementsForClient(ctx, tx, candidates[i].ID)
		if err != nil {
			// coverage:ignore reason: DB query failure, not exercised by unit tests
			return nil, err
		}
		candidates[i].Engagements = engagements
	}
	return candidates, nil
}

// nameWords is every whole word across given, family and preferred name,
// lower-cased -- what the date-of-birth branch of the collision
// predicate compares. Split on whitespace only, so a hyphenated name
// ("Mary-Jane") is one word, matching how a reader would say it aloud
// counts as one name rather than two.
func nameWords(givenName, familyName, preferredName string) []string {
	var words []string
	for _, field := range [...]string{givenName, familyName, preferredName} {
		words = append(words, strings.Fields(strings.ToLower(field))...)
	}
	return words
}

// sharesNameWord reports whether a and b -- each a nameWords result --
// hold at least one word in common. Two Clients may legitimately share a
// date of birth (twins) with no name in common at all, which is exactly
// the coincidence ADR-0017's amendment says passes silently; sharing a
// name word alongside the date is what turns the coincidence into a
// question worth asking.
func sharesNameWord(a, b []string) bool {
	seen := make(map[string]bool, len(a))
	for _, w := range a {
		seen[w] = true
	}
	for _, w := range b {
		if seen[w] {
			return true
		}
	}
	return false
}
