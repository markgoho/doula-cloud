package legal_test

import (
	"testing"
	"time"

	"doula-cloud/api/internal/legal"
)

// history is a document with a version in force and a material one
// still to come, the shape the 30 days of notice leave behind.
var history = legal.Document{Versions: []legal.Version{
	{Effective: "2026-09-29", Material: false, Change: "First version."},
	{Effective: inForce, Material: false, Change: "Writes the name of the product as one word."},
	{Effective: coming, Material: true, Change: "The price of a Credit changes."},
}}

// The dates of history's version in force on October 20, 2026 and of
// the one coming.
const (
	inForce = "2026-10-03"
	coming  = "2026-11-01"
)

func TestInForceIsTheNewestVersionThatHasStarted(t *testing.T) {
	got, ok := history.InForce(time.Date(2026, 10, 20, 12, 0, 0, 0, time.UTC))
	if !ok || got.Effective != inForce {
		t.Fatalf("InForce = %q, %v, want 2026-10-03, true: a version that has not started is not in force", got.Effective, ok)
	}
}

// A version starts at midnight UTC on its date, the reading
// legalnotice.leavesNotice gives its 30 days. In New York that is the
// evening before.
func TestAVersionStartsAtMidnightUTCOnItsDate(t *testing.T) {
	start := time.Date(2026, 11, 1, 0, 0, 0, 0, time.UTC)
	if got, _ := history.InForce(start.Add(-time.Nanosecond)); got.Effective != inForce {
		t.Fatalf("InForce just before midnight UTC = %q, want 2026-10-03", got.Effective)
	}
	if got, _ := history.InForce(start); got.Effective != coming {
		t.Fatalf("InForce at midnight UTC = %q, want 2026-11-01", got.Effective)
	}
	newYork, err := time.LoadLocation("America/New_York")
	if err != nil {
		t.Fatal(err)
	}
	if got, _ := history.InForce(time.Date(2026, 10, 31, 21, 0, 0, 0, newYork)); got.Effective != coming {
		t.Fatalf("InForce at 9 p.m. in New York on October 31 = %q, want 2026-11-01", got.Effective)
	}
}

func TestNothingIsInForceBeforeTheFirstVersion(t *testing.T) {
	if got, ok := history.InForce(time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC)); ok {
		t.Fatalf("InForce = %q, true, want no version", got.Effective)
	}
}

func TestComingIsEveryVersionThatHasNotStarted(t *testing.T) {
	got := history.Coming(time.Date(2026, 10, 20, 0, 0, 0, 0, time.UTC))
	if len(got) != 1 || got[0].Effective != coming {
		t.Fatalf("Coming = %+v, want only 2026-11-01", got)
	}
	if got := history.Coming(time.Date(2026, 11, 1, 0, 0, 0, 0, time.UTC)); len(got) != 0 {
		t.Fatalf("Coming on the day it starts = %+v, want none", got)
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
			// Signup records the version in force, so one must be: a
			// first version dated in the future would leave a new
			// Practice nothing to agree to.
			if _, ok := doc.InForce(time.Now()); !ok {
				t.Fatal("no version is in force today")
			}
		})
	}
}

func TestEachDocumentLivesAtItsOwnAddress(t *testing.T) {
	if legal.Terms.Path != "/terms" || legal.Privacy.Path != "/privacy" {
		t.Fatalf("paths = %q, %q", legal.Terms.Path, legal.Privacy.Path)
	}
}
