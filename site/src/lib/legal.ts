/**
 * The version history of the Terms of Service and the Privacy Policy, for
 * the pages at /terms and /privacy to print.
 *
 * A version is the date it takes effect (ADR-0053). `material` is true
 * when the version changes what a Practice pays, what she gets, or what
 * occurs with her data, and `change` is one line of what changed.
 *
 * `api/internal/legal` holds the same histories for the BFF, which
 * records the version an Owner agreed to. The two must agree, and
 * `legal.spec.ts` beside this file fails when they do not: a new version
 * is one entry here and the same entry there, in the same commit as the
 * new text.
 */
export interface LegalVersion {
	effective: string;
	material: boolean;
	change: string;
}

export interface LegalDocument {
	name: string;
	path: string;
	// Oldest first.
	versions: LegalVersion[];
}

export const TERMS: LegalDocument = {
	name: 'Terms of Service',
	path: '/terms',
	versions: [
		{ effective: '2026-09-29', material: false, change: 'First version.' },
		{ effective: '2026-10-03', material: false, change: 'Writes the name of the product as one word, DoulaCloud.' }
	]
};

export const PRIVACY: LegalDocument = {
	name: 'Privacy Policy',
	path: '/privacy',
	versions: [
		{ effective: '2026-09-29', material: false, change: 'First version.' },
		{
			effective: '2026-10-02',
			material: false,
			change: 'Says how long Feedback is kept, what erases it, and that GitHub keeps a note of each piece.'
		},
		{ effective: '2026-10-03', material: false, change: 'Writes the name of the product as one word, DoulaCloud.' }
	]
};

// The version in force: the newest one.
export function currentVersion(legalDocument: LegalDocument): LegalVersion {
	return legalDocument.versions.at(-1)!;
}
