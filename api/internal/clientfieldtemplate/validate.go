package clientfieldtemplate

import (
	"fmt"
	"slices"
	"strings"
)

// fieldTypeSingleSelect and fieldTypeMultiSelect are shared with
// selectFieldTypes below, the same split plans/template.go draws for its
// own field-type palette.
const (
	fieldTypeSingleSelect = "single_select"
	fieldTypeMultiSelect  = "multi_select"
)

// validFieldTypes is ADR-0001's palette, unchanged by ADR-0017: the only
// kinds of field a Client Field Template may contain. No date type --
// ADR-0017 names this explicitly, since a Practice-defined date is
// exactly the shape #370/#371 moved date_of_birth out of.
var validFieldTypes = map[string]bool{
	"short_text":          true,
	"long_text":           true,
	fieldTypeSingleSelect: true,
	fieldTypeMultiSelect:  true,
	"checkbox":            true,
	"section_header":      true,
}

var selectFieldTypes = map[string]bool{fieldTypeSingleSelect: true, fieldTypeMultiSelect: true}

// The structural fact names structuralFieldNames' error message reports
// -- named constants, not repeated literals, so a canonical name changes
// in one place.
const (
	structuralGivenName     = "given name"
	structuralFamilyName    = "family name"
	structuralPreferredName = "preferred name"
	structuralName          = "name"
	structuralEmail         = "email"
	structuralPhone         = "phone"
	structuralAddress       = "address"
	structuralDateOfBirth   = "date of birth"
)

// structuralFieldNames blocks a Practice-defined label that restates or
// shadows one of ADR-0017's twelve structural columns -- "a Practice
// asking for a middle name is not asking for a custom field; it is
// telling us the structural name has the wrong shape." Matched on the
// trimmed, lowercased label, so "Emergency contact phone" stays legal
// while "Phone" and "Phone Number" do not. This is a block, not a warn:
// the flagged state is always a mistake with a correct alternative (edit
// the structural fact instead), never something legitimate and common.
var structuralFieldNames = map[string]string{
	"given name":     structuralGivenName,
	"first name":     structuralGivenName,
	"family name":    structuralFamilyName,
	"last name":      structuralFamilyName,
	"surname":        structuralFamilyName,
	"preferred name": structuralPreferredName,
	"name":           structuralName,
	"email":          structuralEmail,
	"email address":  structuralEmail,
	"phone":          structuralPhone,
	"phone number":   structuralPhone,
	"telephone":      structuralPhone,
	"address":        structuralAddress,
	"street address": structuralAddress,
	"address line 1": structuralAddress,
	"address line 2": structuralAddress,
	"city":           structuralAddress,
	"locality":       structuralAddress,
	"state":          structuralAddress,
	"region":         structuralAddress,
	"zip":            structuralAddress,
	"zip code":       structuralAddress,
	"postal code":    structuralAddress,
	"date of birth":  structuralDateOfBirth,
	"dob":            structuralDateOfBirth,
	"birth date":     structuralDateOfBirth,
	"birthday":       structuralDateOfBirth,
}

// normalizeFields validates fields against the ADR-0001 palette, the
// shadow-structural-field block, and ADR-0017's archive-never-delete
// rule, and returns them with Order rewritten to each field's array
// position. previous is the Practice's field list before this write --
// every id it names must still be present in fields (archived or not),
// and its Type may not change. Returns a non-empty error message, and a
// nil slice, on the first invalid field.
func normalizeFields(fields []Field, previous []Field) ([]Field, string) {
	previousByID := make(map[string]Field, len(previous))
	for _, f := range previous {
		previousByID[f.ID] = f
	}

	seenIDs := make(map[string]bool, len(fields))
	out := make([]Field, len(fields))
	for i, f := range fields {
		if f.ID == "" {
			return nil, "field id is required"
		}
		if seenIDs[f.ID] {
			return nil, "duplicate field id: " + f.ID
		}
		seenIDs[f.ID] = true

		if !validFieldTypes[f.Type] {
			return nil, "unknown field type: " + f.Type
		}
		if f.Label == "" {
			return nil, "field label is required"
		}
		if structural, blocked := structuralFieldNames[strings.ToLower(strings.TrimSpace(f.Label))]; blocked {
			return nil, fmt.Sprintf("field %s: %q is already on every Client record as the structural %s field", f.ID, f.Label, structural)
		}

		if selectFieldTypes[f.Type] {
			if len(f.Options) == 0 {
				return nil, "field " + f.ID + " requires at least one option"
			}
			if slices.Contains(f.Options, "") {
				return nil, "field " + f.ID + " has a blank option"
			}
		} else if len(f.Options) > 0 {
			return nil, "field " + f.ID + " of type " + f.Type + " may not have options"
		}

		if was, existed := previousByID[f.ID]; existed && was.Type != f.Type {
			return nil, "field " + f.ID + ": type cannot change once created -- archive it and add a new field instead"
		}

		out[i] = Field{ID: f.ID, Type: f.Type, Label: f.Label, Options: f.Options, Order: i, Archived: f.Archived}
	}

	for _, was := range previous {
		if !seenIDs[was.ID] {
			return nil, "field " + was.ID + " cannot be removed -- archive it instead"
		}
	}

	return out, ""
}
