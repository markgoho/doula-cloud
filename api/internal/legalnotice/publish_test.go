package legalnotice_test

import (
	"testing"
	"time"

	"doula-cloud/api/internal/legalnotice"
)

// A version starts at midnight UTC on its date, so 30 days of notice
// from publishedAt (October 23, 2026, 15:00 UTC) reach November 22 at
// 15:00, and a version of November 23 is the first that leaves them.
func TestLeavesNotice_CountsFromMidnightUTCOnTheDate(t *testing.T) {
	for effective, want := range map[string]bool{"2026-11-22": false, "2026-11-23": true} {
		if got := legalnotice.LeavesNotice(effective, publishedAt, legalnotice.NoticeLead); got != want {
			t.Errorf("LeavesNotice(%s) = %v, want %v", effective, got, want)
		}
	}
	if legalnotice.LeavesNotice("2026-12-07", publishedAt, legalnotice.PriceRiseLead) {
		t.Error("a rise 45 days out leaves the 60 days a rise is owed")
	}
	if !legalnotice.LeavesNotice("2026-12-07", publishedAt.Add(-16*24*time.Hour), legalnotice.PriceRiseLead) {
		t.Error("a rise 60 days out does not leave the 60 days a rise is owed")
	}
}
