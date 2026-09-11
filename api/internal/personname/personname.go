// Package personname holds the two rules for turning a Client's stored
// name columns into the name a reader is shown. They live here, below
// every package that spells one, rather than in client, because
// activitypage has to spell a Client actor's name exactly as client
// does and cannot import client without a cycle (client reads its own
// history through activitypage). client re-exports both, so every
// existing caller still reads client.PreferredName.
package personname

// Legal is the document name ADR-0017's read table gives Stripe
// invoicing and the Contract Template's client_name merge field:
// given_name plus family_name when she has one, given_name alone when
// she doesn't -- family_name is the only optional half, so there's never
// a trailing separator to trim.
func Legal(givenName, familyName string) string {
	if familyName == "" {
		return givenName
	}
	return givenName + " " + familyName
}

// Preferred is the conversation name every screen, the Clients sort, and
// the Message thread read: preferred_name when she has one, falling back
// to given_name -- the one column that's never empty.
func Preferred(givenName, preferredName string) string {
	if preferredName == "" {
		return givenName
	}
	return preferredName
}
