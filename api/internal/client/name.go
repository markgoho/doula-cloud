// Package client is the Staff-side BFF write surface for a Client record:
// search, lookup-before-insert create, edit (with the match-query block
// and the invite revoke), the detail read, and the Client-shaped list.
// ADR-0017 (docs/adr/0017-twelve-columns-a-practice-defined-layer-and-an-
// engagement-that-is-asked-for.md). Saving a Client here is free and
// starts nothing -- an Engagement comes only from an approved Engagement
// Request, built elsewhere.
package client

import "doula-cloud/api/internal/personname"

// LegalName and PreferredName are where every caller reaches the two
// naming rules, and both now delegate to personname, which holds the
// rules themselves. The rules moved down a layer in #1150: activitypage
// resolves a Client actor's own name on every subject-scoped read of the
// activity table, this package reads its own history through
// activitypage, and a package cannot import the package that imports it.
// The alternative was a second spelling of "preferred_name, else
// given_name" one import away from this one, which is the drift #1150
// exists to remove.

// LegalName is the document name ADR-0017's read table gives Stripe
// invoicing and the Contract Template's client_name merge field.
func LegalName(givenName, familyName string) string {
	return personname.Legal(givenName, familyName)
}

// PreferredName is the conversation name every screen, the Clients sort,
// and the Message thread read.
func PreferredName(givenName, preferredName string) string {
	return personname.Preferred(givenName, preferredName)
}
