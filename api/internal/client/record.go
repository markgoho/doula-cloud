package client

import (
	"database/sql"
	"encoding/json"
)

// Record is a Client's structural core -- the twelve columns ADR-0017
// specifies -- plus her Practice-defined values. Every text field but
// GivenName is optional: an empty string on the wire means "not set" and
// round-trips to and from SQL NULL, the same convention offer.CreateRequest
// already uses for Terms. DateOfBirth is a plain "YYYY-MM-DD" string, or
// "" when unset.
type Record struct {
	ID                string          `json:"id"`
	GivenName         string          `json:"givenName"`
	FamilyName        string          `json:"familyName"`
	PreferredName     string          `json:"preferredName"`
	Email             string          `json:"email"`
	Phone             string          `json:"phone"`
	AddressLine1      string          `json:"addressLine1"`
	AddressLine2      string          `json:"addressLine2"`
	AddressLocality   string          `json:"addressLocality"`
	AddressRegion     string          `json:"addressRegion"`
	AddressPostalCode string          `json:"addressPostalCode"`
	DateOfBirth       string          `json:"dateOfBirth"`
	FieldValues       json.RawMessage `json:"fieldValues,omitempty"`
}

// nullIfEmpty turns "" into a SQL NULL, and anything else into itself --
// the wire convention Record's optional fields all follow.
func nullIfEmpty(s string) sql.NullString {
	return sql.NullString{String: s, Valid: s != ""}
}

// scanRecord reads the twelve-column-plus-field_values shape of a single
// clients row, in the fixed column order every query in this package
// selects them in: id, given_name, family_name, preferred_name, email,
// phone, address_line1, address_line2, address_locality, address_region,
// address_postal_code, date_of_birth, field_values.
func scanRecord(scan func(dest ...any) error) (Record, error) {
	var rec Record
	var familyName, preferredName, email, phone sql.NullString
	var line1, line2, locality, region, postalCode sql.NullString
	var dob sql.NullString
	var fieldValues []byte
	if err := scan(
		&rec.ID, &rec.GivenName, &familyName, &preferredName, &email, &phone,
		&line1, &line2, &locality, &region, &postalCode, &dob, &fieldValues,
	); err != nil {
		// coverage:ignore reason: DB scan failure, not exercised by unit tests
		return Record{}, err
	}
	rec.FamilyName = familyName.String
	rec.PreferredName = preferredName.String
	rec.Email = email.String
	rec.Phone = phone.String
	rec.AddressLine1 = line1.String
	rec.AddressLine2 = line2.String
	rec.AddressLocality = locality.String
	rec.AddressRegion = region.String
	rec.AddressPostalCode = postalCode.String
	rec.DateOfBirth = dob.String
	// field_values is NOT NULL DEFAULT '{}'::jsonb, so fieldValues is
	// never empty here -- no fallback needed.
	rec.FieldValues = fieldValues
	return rec, nil
}

// recordColumns is the fixed SELECT list scanRecord expects, in order.
// date_of_birth is cast to text so scanRecord's plain sql.NullString scan
// works the same as every other optional column.
const recordColumns = `id, given_name, family_name, preferred_name, email, phone,
	address_line1, address_line2, address_locality, address_region, address_postal_code,
	date_of_birth::text, field_values`
