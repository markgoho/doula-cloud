package ianazone_test

import (
	"errors"
	"testing"

	"doula-cloud/api/internal/ianazone"
)

// TestParse_AcceptsARealZone proves the check is against the IANA
// database that travels in the binary, not against a hand-kept list --
// a zone nobody thought to enumerate still loads.
func TestParse_AcceptsARealZone(t *testing.T) {
	for _, name := range []string{"America/New_York", "America/Denver", "Pacific/Honolulu", "America/Indiana/Indianapolis", "UTC"} {
		loc, err := ianazone.Parse(name)
		if err != nil {
			t.Fatalf("Parse(%q) = %v, want the zone", name, err)
		}
		if loc.String() != name {
			t.Fatalf("Parse(%q) loaded %q", name, loc.String())
		}
	}
}

// TestParse_RefusesTheTwoNamesGoAnswersOnItsOwn is the reason this
// function exists at all rather than a bare time.LoadLocation call: both
// of these load, and neither is a Practice stating a zone. "" would turn
// an omitted field into a silent choice of UTC, and "Local" would make
// the same stored string mean different days on different hosts.
func TestParse_RefusesTheTwoNamesGoAnswersOnItsOwn(t *testing.T) {
	for _, name := range []string{"", "Local"} {
		if _, err := ianazone.Parse(name); !errors.Is(err, ianazone.ErrNotRecognized) {
			t.Fatalf("Parse(%q) = %v, want ErrNotRecognized", name, err)
		}
	}
}

// TestParse_RefusesAZoneTheDatabaseDoesNotCarry covers the ordinary
// wrong answer -- a name that looks like a zone and is not one.
func TestParse_RefusesAZoneTheDatabaseDoesNotCarry(t *testing.T) {
	if _, err := ianazone.Parse("Nowhere/Atlantis"); !errors.Is(err, ianazone.ErrNotRecognized) {
		t.Fatalf("Parse of an unknown zone = %v, want ErrNotRecognized", err)
	}
}
