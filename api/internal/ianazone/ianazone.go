// Package ianazone is the one place the BFF decides whether a string is
// a real IANA time zone name, and the one place the zone database itself
// is embedded.
//
// It is a leaf on purpose. Self-signup validates the zone a founder
// states (staffauth), the settings write validates the zone an Owner
// changes to (practicetimezone), and both would otherwise have to import
// the other -- staffauth cannot import a package that mounts handlers
// behind staffauth. Keeping the check here, with no dependency of its
// own, means one answer to "is this a zone" rather than two that can
// drift.
package ianazone

import (
	"errors"
	"fmt"
	"time"

	// Embeds the IANA database in the binary, as the fallback
	// time.LoadLocation reaches for when the host has no zoneinfo of its
	// own. gcr.io/distroless/static-debian12 does ship
	// /usr/share/zoneinfo today (checked by exporting the image and
	// listing it), so this is not repairing a broken image -- it is
	// refusing to depend on a base image's contents for an answer the
	// product gets wrong silently. The zone data then travels with the
	// code that reads it, which is why the import sits beside the check
	// rather than in main.
	_ "time/tzdata"
)

// MsgNotRecognized is what a person reads when the zone she sent is not
// one the IANA database names -- docs/api-design.md section 7 rule 4: it
// says what to do, and opens with "Enter", not "timezone", the wire's own
// name for the field (#1189). apierr's TestDetailsWording still bans the
// four words it always has. The settings screen and the signup form both
// offer a list, so the only way to reach this is a hand-built request, or
// a browser reporting a zone this database does not carry.
const MsgNotRecognized = "Enter a timezone from the IANA database, such as America/New_York"

// ErrNotRecognized is what Parse reports for a name the IANA database
// does not carry, and for the two names time.LoadLocation answers
// without consulting it at all.
var ErrNotRecognized = errors.New("ianazone: not an IANA zone name")

// Parse turns a submitted zone name into the location it names, refusing
// anything that is not a real IANA zone.
//
// time.LoadLocation answers two names out of thin air rather than out of
// the database, and both are refused here. "" is UTC, so an omitted
// field would otherwise be accepted as a deliberate choice of UTC, and a
// Practice that meant to say nothing would silently get a zone no US
// Practice works in. "Local" is whichever zone the process happens to
// run in, which is the server's fact and not the Practice's: the same
// stored string would mean a different calendar day depending on where
// the binary was deployed, which is precisely what ADR-0036 exists to
// stop.
func Parse(name string) (*time.Location, error) {
	if name == "" || name == "Local" {
		return nil, fmt.Errorf("%w: %q", ErrNotRecognized, name)
	}
	loc, err := time.LoadLocation(name)
	if err != nil {
		return nil, fmt.Errorf("%w: %q: %w", ErrNotRecognized, name, err)
	}
	return loc, nil
}
