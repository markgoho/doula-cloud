// Package personname holds the two rules for turning a Client's stored
// name columns into the name a reader is shown. They live here, below
// every package that spells one, rather than in client, because
// activitypage has to spell a Client actor's name exactly as every
// other reader does and cannot import client without a cycle (client
// reads its own history through activitypage). Every caller reaches the
// rules here directly.
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

// Display is the name a Client's own record is titled with: her preferred
// name when she has one, otherwise her given and family names together.
// It is the app's `displayName` (clientDetail.ts), so the Client record and
// the Engagement page carry one title for one person (#2004); a bare given
// name ("Amara") reads as a first name only, not as whose record it is.
func Display(givenName, familyName, preferredName string) string {
	if preferredName != "" {
		return preferredName
	}
	return Legal(givenName, familyName)
}

// Full is the name a row reads where it names one Client among the
// Practice's others (#2095): the conversation name (Preferred) followed
// by family_name when she has one. The Practice-wide activity feed reads
// it, because two Clients at one Practice can share a first name and
// "Visit added for Sarah" then does not say which Sarah. The brief's
// Voice section, rule 2's amendment of 2026-10-26, is the rule; this is
// its one spelling.
func Full(givenName, preferredName, familyName string) string {
	return Legal(Preferred(givenName, preferredName), familyName)
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
