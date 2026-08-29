package client

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"

	"doula-cloud/api/internal/activity"
)

type eventType string

const (
	eventCreated eventType = "created"
	eventUpdated eventType = "updated"
)

// change is one changed fact's client_events diff entry: both sides, so
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
// `created` client_events row states every field intake actually set --
// "from" is always empty, "to" is what was saved.
func createdDiff(rec Record) map[string]change {
	return diffRecords(Record{FieldValues: []byte("{}")}, rec)
}

// recordEvent writes one activity row (subject_kind 'client', ADR-0022):
// eventType, the diff, and the acting Staff member. Every create and
// every edit writes exactly one row, even a no-op edit whose diff is
// empty -- "one row per act" (ADR-0017), not one row per changed fact.
func recordEvent(ctx context.Context, tx *sql.Tx, practiceID, clientID string, et eventType, diff map[string]change, actorStaffID string) error {
	diffJSON, err := json.Marshal(diff)
	if err != nil {
		// coverage:ignore reason: a map of strings/RawMessage always marshals cleanly, not exercised by unit tests
		return fmt.Errorf("client: marshal event diff: %w", err)
	}
	if err := activity.Record(ctx, tx, activity.Entry{
		PracticeID:  practiceID,
		SubjectKind: "client",
		SubjectID:   clientID,
		Action:      string(et),
		Diff:        diffJSON,
		Actor:       activity.StaffActor(actorStaffID),
	}); err != nil {
		// coverage:ignore reason: DB query failure, not exercised by unit tests
		return fmt.Errorf("client: record client event: %w", err)
	}
	return nil
}
