package personname_test

import (
	"testing"

	"doula-cloud/api/internal/personname"
)

// Every reader of a Client's name reaches these rules here, so this is
// the one suite that holds them.

const (
	givenName       = "Renata"
	clientGiven     = "Margaretha"
	clientPreferred = "Greta"
	clientFamily    = "Okafor"
)

func TestLegal(t *testing.T) {
	for _, tc := range []struct {
		name         string
		given        string
		family       string
		want         string
		whyItMatters string
	}{
		{"both halves", givenName, "Alvarez", "Renata Alvarez", "the ordinary case"},
		{"no family name", givenName, "", givenName, "family_name is the only optional half, so there is no trailing separator to trim"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := personname.Legal(tc.given, tc.family); got != tc.want {
				t.Errorf("Legal(%q, %q) = %q, want %q -- %s", tc.given, tc.family, got, tc.want, tc.whyItMatters)
			}
		})
	}
}

func TestFull(t *testing.T) {
	for _, tc := range []struct {
		name         string
		given        string
		preferred    string
		family       string
		want         string
		whyItMatters string
	}{
		{"preferred and family", clientGiven, clientPreferred, clientFamily, clientPreferred + " " + clientFamily, "the conversation name leads, and the family name tells two Gretas apart"},
		{"preferred and no family name", clientGiven, clientPreferred, "", clientPreferred, "a record titled by Full never reads as a bare given name she does not go by"},
		{"no preferred name", givenName, "", "Alvarez", "Renata Alvarez", "the given name stands in for the preferred name, as Preferred says"},
		{"no family name", givenName, "", "", givenName, "family_name is optional, so there is no trailing separator to trim"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := personname.Full(tc.given, tc.preferred, tc.family); got != tc.want {
				t.Errorf("Full(%q, %q, %q) = %q, want %q -- %s", tc.given, tc.preferred, tc.family, got, tc.want, tc.whyItMatters)
			}
		})
	}
}

func TestPreferred(t *testing.T) {
	for _, tc := range []struct {
		name         string
		given        string
		preferred    string
		want         string
		whyItMatters string
	}{
		{"she has a preferred name", clientGiven, clientPreferred, clientPreferred, "the name she is called is the first name every sentence uses"},
		{"she has none", clientGiven, "", clientGiven, "given_name is the one column that is never empty"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := personname.Preferred(tc.given, tc.preferred); got != tc.want {
				t.Errorf("Preferred(%q, %q) = %q, want %q -- %s", tc.given, tc.preferred, got, tc.want, tc.whyItMatters)
			}
		})
	}
}
