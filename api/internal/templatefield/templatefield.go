// Package templatefield is what a Plan Template and a Client Field
// Template share: ADR-0001's field-type palette, the checks every field in
// either list must pass, and the audit row a change to either list leaves.
// Each template package keeps its own Field type and the rules that are
// its alone.
package templatefield

import (
	"context"
	"database/sql"
	"encoding/json/jsontext"
	"encoding/json/v2"
	"fmt"
	"slices"

	"doula-cloud/api/internal/activity"
)

// ADR-0001's field-type palette is the only set of kinds a field in either
// template may have.
var types = map[string]bool{
	"short_text":     true,
	"long_text":      true,
	"single_select":  true,
	"multi_select":   true,
	"checkbox":       true,
	"section_header": true,
}

var selectTypes = map[string]bool{"single_select": true, "multi_select": true}

// CheckIdentity returns the message for the first thing wrong with a
// field's id, type or label, or "" when all three pass. seenIDs holds the
// ids of the fields before it in the list and gains this one.
func CheckIdentity(seenIDs map[string]bool, id, fieldType, label string) string {
	if id == "" {
		return "field id is required"
	}
	if seenIDs[id] {
		return "duplicate field id: " + id
	}
	seenIDs[id] = true

	if !types[fieldType] {
		return "unknown field type: " + fieldType
	}
	if label == "" {
		return "field label is required"
	}
	return ""
}

// CheckOptions returns the message for options that do not fit the field's
// type -- a select needs at least one and none blank, anything else may
// have none -- or "" when they fit.
func CheckOptions(id, fieldType string, options []string) string {
	if selectTypes[fieldType] {
		if len(options) == 0 {
			return "field " + id + " requires at least one option"
		}
		if slices.Contains(options, "") {
			return "field " + id + " has a blank option"
		}
	} else if len(options) > 0 {
		return "field " + id + " of type " + fieldType + " may not have options"
	}
	return ""
}

// RecordChange writes the Activity row for a template's field list going
// from previous to fieldsJSON, subject_kind subjectKind, holding both
// sides. A save that changes nothing writes no row, so the trail stays one
// row per real act.
func RecordChange(ctx context.Context, tx *sql.Tx, practiceID, staffID, subjectKind, templateID string, previous any, fieldsJSON []byte) error {
	previousJSON, err := json.Marshal(previous)
	if err != nil {
		// coverage:ignore reason: a Field list always marshals cleanly, not exercised by unit tests
		return fmt.Errorf("templatefield: marshal previous fields: %w", err)
	}
	if string(previousJSON) == string(fieldsJSON) {
		return nil
	}

	diff, err := json.Marshal(map[string]jsontext.Value{"before": previousJSON, "after": fieldsJSON})
	if err != nil {
		// coverage:ignore reason: both sides always marshal cleanly, not exercised by unit tests
		return fmt.Errorf("templatefield: marshal diff: %w", err)
	}
	if err := activity.Record(ctx, tx, activity.Entry{
		PracticeID:  practiceID,
		SubjectKind: subjectKind,
		SubjectID:   templateID,
		Action:      "updated",
		Diff:        diff,
		Actor:       activity.StaffActor(staffID),
	}); err != nil {
		// coverage:ignore reason: DB query failure, not exercised by unit tests
		return fmt.Errorf("templatefield: record audit event: %w", err)
	}
	return nil
}
