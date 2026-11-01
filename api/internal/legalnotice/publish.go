// Package legalnotice tells each Owner of each Practice, 30 days or more
// ahead, that a material version of the Terms of Service or the Privacy
// Policy is coming (ADR-0053, #1548), and 60 days or more ahead when it
// raises the price of a Credit (#1817). A change posted with no notice does
// not bind a Practice (Douglas v. Talk America), so the notice is the
// first half of how a material change takes effect; the screen an Owner
// agrees on after the date is the second (#1549).
//
// The version history stays in api/internal/legal. The act of publishing
// a version is ops's (#2486); this package holds the rules it publishes
// by and the outbox (ADR-0010) whose rows are the record of who was told.
package legalnotice

import (
	"strings"
	"time"

	"doula-cloud/api/internal/legal"
)

// NoticeLead is the least time between the emails and the date a
// material version takes effect. The Terms promise it: "we email each
// Owner at least 30 days before the new version takes effect".
const NoticeLead = 30 * 24 * time.Hour

// PriceRiseLead is the least time between the emails and the date a
// version that raises the price of a Credit takes effect. The Terms
// promise it: "If the price of a Credit ever rises, we email each Owner
// at least 60 days before, with the reason" (#1817).
const PriceRiseLead = 60 * 24 * time.Hour

// Key is how a document is named in the database and in an address: its
// path without the slash ("terms" or "privacy").
func Key(doc legal.Document) string {
	return strings.TrimPrefix(doc.Path, "/")
}

// Find is the version named by document and effective, in docs.
func Find(docs []legal.Document, document, effective string) (legal.Document, legal.Version, bool) {
	for _, doc := range docs {
		if Key(doc) != document {
			continue
		}
		for _, v := range doc.Versions {
			if v.Effective == effective {
				return doc, v, true
			}
		}
	}
	return legal.Document{}, legal.Version{}, false
}

// LeavesNotice answers whether a version that takes effect on effective
// leaves lead between now and its start. The version is a date with no
// zone, so it is taken to start at midnight UTC, which is before
// midnight anywhere in the United States: the reading that never gives
// an Owner less than the lead.
func LeavesNotice(effective string, now time.Time, lead time.Duration) bool {
	start, err := time.Parse(time.DateOnly, effective)
	// coverage:ignore reason: legal_test.go holds every version's Effective to a date, and a fixture is written the same way
	if err != nil {
		return false
	}
	return start.Sub(now) >= lead
}
