/**
 * A Client's two screen names, spelled once for the app (#2111). The same
 * rules are `personname.Preferred` and `personname.Full` in the BFF, and
 * the brief's Voice amendments of 2026-10-26 (#2095, #2111) say which
 * surface reads which.
 *
 * One fallback serves both: with no preferred name on the record, the
 * given name stands in for it.
 */

/** The name columns a Client's record carries. Only the given name is
 * ever demanded (ADR-0017), so the other two may be absent or empty. */
export interface PersonNameParts {
	givenName: string;
	preferredName?: string | null;
	familyName?: string | null;
}

/**
 * Her first name: the preferred name, else the given name.
 */
export function firstName({ givenName, preferredName }: PersonNameParts): string {
	return preferredName || givenName;
}

/** Her full name: the first name, then the family name when there is one.
 * A screen that sets Clients side by side reads it, and a screen about
 * one Client is titled with it. */
export function fullName(parts: PersonNameParts): string {
	return [firstName(parts), parts.familyName].filter(Boolean).join(' ');
}
