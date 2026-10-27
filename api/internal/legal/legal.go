// Package legal holds the version history of the two documents a
// Practice agrees to when its Owner creates it: the Terms of Service and
// the Privacy Policy (ADR-0053). The text lives on the marketing site, at
// each Document's Path; what the BFF needs is which version is in force,
// whether each version was a material change, one line of what changed,
// and the reason for a rise in the price of a Credit, because the
// agreement recorded at signup (#1547), the email before a material
// change (#1548) and the screen after it (#1549) read those facts and
// must not invent them.
//
// site/src/lib/legal.ts carries the same histories for the pages to
// print, and the two have to agree: site/src/lib/legal.spec.ts reads this
// file and fails when they differ, the same kind of mirrored pair as
// website.SiteBaseURL and the site's SITE_ORIGIN. That spec reads the
// Version literals below with a pattern, so each stays on one line with
// its fields in this order: CreditPriceCents after Change on a version of
// the Terms, and PriceRise, when a version has one, last.
package legal

import "time"

// Version is one version of a document.
type Version struct {
	// Effective is the date the version takes effect, in the form
	// 2026-10-15. It is the version's identity (ADR-0053).
	Effective string
	// Material is true when the version changes what a Practice pays,
	// what she gets, or what occurs with her data. A material version
	// emails each Owner 30 days or more before Effective, and asks an
	// Owner to agree at her next sign-in after it.
	Material bool
	// Change is one line that says what changed.
	Change string
	// CreditPriceCents is the price of one Credit, in cents before sales
	// tax, that a version of the Terms states ("A Credit costs $20.00").
	// Checkout charges the price of the version in force (#1834), so a
	// new price starts on the date of its version, not when someone
	// moves a Stripe Price. Every version of the Terms sets it; the
	// Privacy Policy says nothing about price and leaves it out.
	CreditPriceCents int64
	// PriceRise is empty unless the version raises the price of a
	// Credit, and then it is the reason, one line the email before the
	// version prints. The Terms promise "If the price of a Credit ever
	// rises, we email each Owner at least 60 days before, with the
	// reason" (ADR-0042), so a rise is published 60 days or more before
	// Effective, not 30, and a rise is always Material (#1817). It is
	// the last field of a literal, and a literal that is not a rise
	// leaves it out.
	PriceRise string
}

// Document is one of the two documents, with its versions oldest first.
type Document struct {
	Name     string
	Path     string
	Versions []Version
}

// InForce is the version in force at the instant at: the newest one that
// has started. A version starts at midnight UTC on its date, the same
// reading legalnotice gives its 30 days of notice, so in the United
// States it starts on the evening before. The newest version is not
// always in force: a material one is in the history 30 days or more
// before it starts (#1816).
//
// ok is false when no version has started at at. legal_test.go holds
// that each shipped document has one in force today.
func (d Document) InForce(at time.Time) (version Version, ok bool) {
	for _, v := range d.Versions {
		if !started(v, at) {
			break
		}
		version, ok = v, true
	}
	return version, ok
}

// Coming is every version that has not started at the instant at, oldest
// first: the ones a Practice has been told of, or is being told of, and
// that do not apply to it yet.
func (d Document) Coming(at time.Time) []Version {
	var coming []Version
	for _, v := range d.Versions {
		if !started(v, at) {
			coming = append(coming, v)
		}
	}
	return coming
}

// started compares dates as text: Effective is YYYY-MM-DD, which sorts
// the way the dates do, and at's date in UTC is the day that has begun.
func started(v Version, at time.Time) bool {
	return v.Effective <= at.UTC().Format(time.DateOnly)
}

// Terms is the Terms of Service, at doula.cloud/terms.
var Terms = Document{
	Name: "Terms of Service",
	Path: "/terms",
	Versions: []Version{
		{Effective: "2026-09-29", Material: false, Change: "First version.", CreditPriceCents: 2000},
		{Effective: "2026-10-03", Material: false, Change: "Writes the name of the product as one word, DoulaCloud.", CreditPriceCents: 2000},
	},
}

// Privacy is the Privacy Policy, at doula.cloud/privacy.
var Privacy = Document{
	Name: "Privacy Policy",
	Path: "/privacy",
	Versions: []Version{
		{Effective: "2026-09-29", Material: false, Change: "First version."},
		{Effective: "2026-10-02", Material: false, Change: "Says how long Feedback is kept, what erases it, and that GitHub keeps a note of each piece."},
		{Effective: "2026-10-03", Material: false, Change: "Writes the name of the product as one word, DoulaCloud."},
		{Effective: "2026-10-22", Material: false, Change: "Names Buttondown, which keeps the waitlist, and Pirsch, which counts visits to the home page without a cookie."},
		{Effective: "2026-10-27", Material: false, Change: "Says the waitlist keeps whether you asked to be in the pilot."},
	},
}
