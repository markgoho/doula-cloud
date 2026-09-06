package client

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"

	"doula-cloud/api/internal/activity"
	"doula-cloud/api/internal/clientkey"
)

type eventType string

const (
	eventCreated eventType = "created"
	eventUpdated eventType = "updated"
	// eventMerged is written on a merge's survivor row: its own field
	// changes, plus a synthetic "mergedFrom" entry naming the absorbed
	// row -- "the survivor records what changed and where it came from"
	// (ADR-0017's amendment).
	eventMerged eventType = "merged"
	// eventAbsorbed is written on a merge's tombstoned row: no field
	// diff of its own (nothing on it changed; it is the row that stopped
	// being current), only a "mergedInto" entry naming the survivor.
	// ADR-0017's amendment puts it this way: the absorbed row records
	// where it went.
	eventAbsorbed eventType = "absorbed"
	// eventErased is the one client-subject action whose diff is NOT
	// sealed (ADR-0027). It describes the erasure, not the data erased,
	// and it has to stay readable after her key is destroyed -- so it
	// goes through activity.Record directly (see erase.go), never
	// recordEvent.
	eventErased eventType = "erased"
)

// change is one changed fact's activity diff entry: both sides, so
// an event row stays legible without joining back to the row it
// describes.
type change struct {
	From any `json:"from"`
	To   any `json:"to"`
}

// diffRecords compares old and next field-by-field, returning only the
// facts that differ -- structural fields keyed on their column name.
// field_values compares as one whole-blob "fieldValues" entry rather than
// per-field-id: Client Field Template editing isn't built in this
// ticket, and the whole-blob diff still tells a reader exactly what
// changed.
func diffRecords(old, next Record) map[string]change {
	diff := map[string]change{}
	add := func(key, from, to string) {
		if from != to {
			diff[key] = change{From: from, To: to}
		}
	}
	add("givenName", old.GivenName, next.GivenName)
	add("familyName", old.FamilyName, next.FamilyName)
	add("preferredName", old.PreferredName, next.PreferredName)
	add("email", old.Email, next.Email)
	add("phone", old.Phone, next.Phone)
	add("addressLine1", old.AddressLine1, next.AddressLine1)
	add("addressLine2", old.AddressLine2, next.AddressLine2)
	add("addressLocality", old.AddressLocality, next.AddressLocality)
	add("addressRegion", old.AddressRegion, next.AddressRegion)
	add("addressPostalCode", old.AddressPostalCode, next.AddressPostalCode)
	add("dateOfBirth", old.DateOfBirth, next.DateOfBirth)
	if string(old.FieldValues) != string(next.FieldValues) {
		diff["fieldValues"] = change{From: old.FieldValues, To: next.FieldValues}
	}
	return diff
}

// createdDiff is diffRecords against a wholly-empty Record, so a
// `created` activity row states every field intake actually set --
// "from" is always empty, "to" is what was saved.
func createdDiff(rec Record) map[string]change {
	return diffRecords(Record{FieldValues: []byte("{}")}, rec)
}

// recordEvent writes one activity row (subject_kind 'client', ADR-0022):
// eventType, the diff, and the acting Staff member. Every create and
// every edit writes exactly one row, even a no-op edit whose diff is
// empty -- "one row per act" (ADR-0017), not one row per changed fact.
//
// The diff is sealed under the Client's own key before it is written
// (ADR-0027). This is the one activity write site in the product that
// puts a person's name, address, phone and date of birth into a jsonb
// column, and activity can never be UPDATEd or DELETEd, so erasure has
// no way to reach these rows afterwards -- it destroys the key instead.
// The plaintext columns around the diff (what happened, when, who did
// it) are untouched and stay readable forever.
func recordEvent(ctx context.Context, tx *sql.Tx, practiceID, clientID string, et eventType, diff map[string]change, actorStaffID string) error {
	diffJSON, err := json.Marshal(diff)
	if err != nil {
		// coverage:ignore reason: a map of strings/RawMessage always marshals cleanly, not exercised by unit tests
		return fmt.Errorf("client: marshal event diff: %w", err)
	}
	// Ensure before Seal, not only at create time: a Client who predates
	// #394 has no key, and the first thing written about her makes one.
	// It refuses to remake a key for an erased Client, so Seal below
	// still reports ErrNoKey there -- an erased Client's history does not
	// start again.
	if err := clientkey.Ensure(ctx, tx, practiceID, clientID); err != nil {
		// coverage:ignore reason: DB query failure, not exercised by unit tests
		return fmt.Errorf("client: ensure client key: %w", err)
	}
	sealed, err := clientkey.Seal(ctx, tx, clientID, diffJSON)
	if err != nil {
		// coverage:ignore reason: Ensure above has just made the key, so the only way here is an erased Client -- and EditHandler refuses one before it reaches this. The branch stays as the backstop for a future write site that does not.
		return fmt.Errorf("client: seal event diff: %w", err)
	}
	if err := activity.Record(ctx, tx, activity.Entry{
		PracticeID:  practiceID,
		SubjectKind: activity.SubjectClient,
		SubjectID:   clientID,
		Action:      string(et),
		Diff:        sealed,
		Actor:       activity.StaffActor(actorStaffID),
	}); err != nil {
		// coverage:ignore reason: DB query failure, not exercised by unit tests
		return fmt.Errorf("client: record client event: %w", err)
	}
	return nil
}
