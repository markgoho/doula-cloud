package legal_test

import (
	"testing"
	"time"

	"doula-cloud/api/internal/legal"
)

func TestCurrentIsTheNewestVersion(t *testing.T) {
	doc := legal.Document{Versions: []legal.Version{
		{Effective: "2026-09-29", Material: false, Change: "First version."},
		{Effective: "2026-11-01", Material: true, Change: "The price of a Credit changes."},
	}}
	if got := doc.Current().Effective; got != "2026-11-01" {
		t.Fatalf("Current().Effective = %q, want the newest version", got)
	}
}

// TestEachDocumentsHistory holds the shape #1547, #1548 and #1549 read:
// a version is a date (ADR-0053), the history runs oldest first with no
// two versions on one date, and every version says what changed.
func TestEachDocumentsHistory(t *testing.T) {
	for _, doc := range []legal.Document{legal.Terms, legal.Privacy} {
		t.Run(doc.Name, func(t *testing.T) {
			if len(doc.Versions) == 0 {
				t.Fatal("no versions")
			}
			var previous time.Time
			for i, v := range doc.Versions {
				effective, err := time.Parse(time.DateOnly, v.Effective)
				if err != nil {
					t.Fatalf("version %d: Effective %q is not a date: %v", i, v.Effective, err)
				}
				if i > 0 && !effective.After(previous) {
					t.Fatalf("version %d: %s is not after the version before it", i, v.Effective)
				}
				if v.Change == "" {
					t.Fatalf("version %d: says nothing about what changed", i)
				}
				previous = effective
			}
			// A first version changes nothing anybody agreed to, so no
			// Owner is emailed or asked again for it.
			if doc.Versions[0].Material {
				t.Fatal("the first version is marked material")
			}
		})
	}
}

func TestEachDocumentLivesAtItsOwnAddress(t *testing.T) {
	if legal.Terms.Path != "/terms" || legal.Privacy.Path != "/privacy" {
		t.Fatalf("paths = %q, %q", legal.Terms.Path, legal.Privacy.Path)
	}
}
