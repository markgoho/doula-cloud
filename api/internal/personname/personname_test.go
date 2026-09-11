package personname_test

import (
	"testing"

	"doula-cloud/api/internal/personname"
)

// These two rules are reached through client.LegalName and
// client.PreferredName by most of the codebase, and directly by
// activitypage, which cannot import client. A package with no test file
// of its own contributes no blocks to the coverage profile at all, so it
// would pass the gate without ever being measured -- which is why these
// live here rather than only in client's own suite.

const (
	givenName   = "Renata"
	clientGiven = "Margaretha"
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

func TestPreferred(t *testing.T) {
	for _, tc := range []struct {
		name         string
		given        string
		preferred    string
		want         string
		whyItMatters string
	}{
		{"she has a preferred name", clientGiven, "Greta", "Greta", "the name she is called is the one every screen shows"},
		{"she has none", clientGiven, "", clientGiven, "given_name is the one column that is never empty"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := personname.Preferred(tc.given, tc.preferred); got != tc.want {
				t.Errorf("Preferred(%q, %q) = %q, want %q -- %s", tc.given, tc.preferred, got, tc.want, tc.whyItMatters)
			}
		})
	}
}
