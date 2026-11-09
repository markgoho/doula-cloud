package clientfieldtemplate

import (
	"fmt"
	"strings"

	"doula-cloud/api/internal/templatefield"
)

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
		if msg := templatefield.CheckIdentity(seenIDs, f.ID, f.Type, f.Label); msg != "" {
			return nil, msg
		}
		if structural, blocked := structuralFieldNames[strings.ToLower(strings.TrimSpace(f.Label))]; blocked {
			return nil, fmt.Sprintf("field %s: %q is already on every Client record as the structural %s field", f.ID, f.Label, structural)
		}

		if msg := templatefield.CheckOptions(f.ID, f.Type, f.Options); msg != "" {
			return nil, msg
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
