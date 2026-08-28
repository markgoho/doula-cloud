// Package client is the Staff-side BFF write surface for a Client record:
// search, lookup-before-insert create, edit (with the match-query block
// and the invite revoke), the detail read, and the Client-shaped list.
// ADR-0017 (docs/adr/0017-twelve-columns-a-practice-defined-layer-and-an-
// engagement-that-is-asked-for.md). Saving a Client here is free and
// starts nothing -- an Engagement comes only from an approved Engagement
// Request, built elsewhere.
package client

// LegalName is the document name ADR-0017's read table gives Stripe
// invoicing and the Contract Template's client_name merge field:
// given_name plus family_name when she has one, given_name alone when she
// doesn't -- family_name is the only optional half, so there's never a
// trailing separator to trim.
func LegalName(givenName, familyName string) string {
	if familyName == "" {
		return givenName
	}
	return givenName + " " + familyName
}

// PreferredName is the conversation name every screen, the Clients sort,
// and the Message thread read: preferred_name when she has one, falling
// back to given_name -- the one column that's never empty.
func PreferredName(givenName, preferredName string) string {
	if preferredName == "" {
		return givenName
	}
	return preferredName
}
