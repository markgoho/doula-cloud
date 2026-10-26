// Package personname holds the rules for turning a Client's stored name
// columns into the name a reader is shown. They live here, below every
// package that spells one, rather than in client, because activitypage
// has to spell a Client actor's name exactly as every other reader does
// and cannot import client without a cycle (client reads its own history
// through activitypage). Every caller reaches the rules here directly.
//
// There are three names and one fallback (#2111). Legal is the document
// name. Preferred is her first name. Full is that first name followed by
// the family name. With no preferred name on the record the given name
// stands in for it, in Preferred and in Full alike, so one Client never
// reads "Amara Okafor" on one screen and "Amara" as the same first name on
// the next. The brief's Voice amendments of 2026-10-26 (#2095, #2111) say
// which surface reads which; app/src/lib/personName.ts is the app's
// spelling of the same rules.
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

// Full is the name a surface reads where it sets Clients side by side --
// the Clients list, the Overview's "Clients waiting on a reply" table,
// the Practice-wide activity feed -- and the title of a screen about one
// Client: the first name (Preferred) followed by family_name when she
// has one. Two Clients at one Practice can share a first name, and
// "Visit added for Sarah" then does not say which Sarah (#2095).
func Full(givenName, preferredName, familyName string) string {
	return Legal(Preferred(givenName, preferredName), familyName)
}

// Preferred is her first name, the conversation name a sentence about one
// Client and the Message thread read: preferred_name when she has one,
// falling back to given_name -- the one column that's never empty.
func Preferred(givenName, preferredName string) string {
	if preferredName == "" {
		return givenName
	}
	return preferredName
}
