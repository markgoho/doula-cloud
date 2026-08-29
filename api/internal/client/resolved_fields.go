package client

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"

	"doula-cloud/api/internal/clientfieldtemplate"
)

// noLongerCollected is the wording a Client detail page shows beside an
// archived field she still holds a value in (ADR-0017, #399's AC3):
// still visible on her record, labelled and marked as no longer
// collected.
const noLongerCollected = "No longer collected"

// ResolvedField is one entry of a Client's Practice-defined layer,
// resolved against the Practice's *current* Client Field Template --
// ADR-0017's departure from ADR-0001: a Client's values are read live,
// never snapshotted, so this is recomputed on every read rather than
// stored. An active field always appears, blank or not, because it's on
// the form; an archived field appears only when the Client holds a
// stored value under its id, marked with Note.
type ResolvedField struct {
	FieldID string          `json:"fieldId"`
	Label   string          `json:"label"`
	Type    string          `json:"type"`
	Options []string        `json:"options,omitempty"`
	Value   json.RawMessage `json:"value"`
	Note    string          `json:"note,omitempty"`
}

// resolveFields reads practiceID's Client Field Template and pairs it
// with fieldValues (a Client's stored field_values JSONB), producing one
// ResolvedField per active field and per archived field the Client still
// holds a value in.
func resolveFields(ctx context.Context, tx *sql.Tx, practiceID string, fieldValues json.RawMessage) ([]ResolvedField, error) {
	fields, err := clientfieldtemplate.FetchFields(ctx, tx, practiceID)
	if err != nil {
		// coverage:ignore reason: DB query failure, not exercised by unit tests
		return nil, fmt.Errorf("client: fetch client field template: %w", err)
	}

	values := map[string]json.RawMessage{}
	if len(fieldValues) > 0 {
		if err := json.Unmarshal(fieldValues, &values); err != nil {
			// coverage:ignore reason: field_values is always written as a JSON object, not exercised by unit tests
			return nil, fmt.Errorf("client: unmarshal field values: %w", err)
		}
	}

	resolved := make([]ResolvedField, 0, len(fields))
	for _, f := range fields {
		value, held := values[f.ID]
		if f.Archived && !held {
			continue
		}
		rf := ResolvedField{FieldID: f.ID, Label: f.Label, Type: f.Type, Options: f.Options, Value: value}
		if f.Archived {
			rf.Note = noLongerCollected
		}
		resolved = append(resolved, rf)
	}
	return resolved, nil
}
