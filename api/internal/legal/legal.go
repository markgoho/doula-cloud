// Package legal holds the version history of the two documents a
// Practice agrees to when its Owner creates it: the Terms of Service and
// the Privacy Policy (ADR-0053). The text lives on the marketing site, at
// each Document's Path; what the BFF needs is which version is current,
// whether each version was a material change, and one line of what
// changed, because the agreement recorded at signup (#1547), the email
// before a material change (#1548) and the screen after it (#1549) read
// those facts and must not invent them.
//
// site/src/lib/legal.ts carries the same histories for the pages to
// print, and the two have to agree: site/src/lib/legal.spec.ts reads this
// file and fails when they differ, the same kind of mirrored pair as
// website.SiteBaseURL and the site's SITE_ORIGIN. That spec reads the
// Version literals below with a pattern, so each stays on one line with
// its fields in this order.
package legal

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
}

// Document is one of the two documents, with its versions oldest first.
type Document struct {
	Name     string
	Path     string
	Versions []Version
}

// Current is the version in force: the newest one.
func (d Document) Current() Version {
	return d.Versions[len(d.Versions)-1]
}

// Terms is the Terms of Service, at doula.cloud/terms.
var Terms = Document{
	Name: "Terms of Service",
	Path: "/terms",
	Versions: []Version{
		{Effective: "2026-09-29", Material: false, Change: "First version."},
	},
}

// Privacy is the Privacy Policy, at doula.cloud/privacy.
var Privacy = Document{
	Name: "Privacy Policy",
	Path: "/privacy",
	Versions: []Version{
		{Effective: "2026-09-29", Material: false, Change: "First version."},
	},
}
