/**
 * The version history of the Terms of Service and the Privacy Policy, for
 * the pages at /terms and /privacy to print.
 *
 * A version is the date it takes effect (ADR-0053). `material` is true
 * when the version changes what a Practice pays, what she gets, or what
 * occurs with her data, and `change` is one line of what changed.
 * `creditPriceCents` is the price of a Credit a version of the Terms
 * states, in cents before tax; checkout charges the one in force (#1834),
 * and the Privacy Policy leaves it out.
 * `priceRise` is there only on a version that raises the price of a
 * Credit, and it is the reason the email before the version gives: the
 * Terms promise "at least 60 days before, with the reason" (#1817).
 *
 * `api/internal/legal` holds the same histories for the BFF, which
 * records the version an Owner agreed to. The two must agree, and
 * `legal.spec.ts` beside this file fails when they do not: a new version
 * is one entry here and the same entry there, in the same commit as the
 * new text, which is its own component in `legalTexts.ts`.
 */
export interface LegalVersion {
	effective: string;
	material: boolean;
	change: string;
	creditPriceCents?: number;
	priceRise?: string;
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
		{ effective: '2026-09-29', material: false, change: 'First version.', creditPriceCents: 2000 },
		{
			effective: '2026-10-03',
			material: false,
			change: 'Writes the name of the product as one word, DoulaCloud.',
			creditPriceCents: 2000
		}
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
		{ effective: '2026-10-03', material: false, change: 'Writes the name of the product as one word, DoulaCloud.' },
		{
			effective: '2026-10-22',
			material: false,
			change: 'Names Buttondown, which keeps the waitlist, and Pirsch, which counts visits to the home page without a cookie.'
		}
	]
};

/**
 * The version in force at `now`: the newest one that has started, which
 * is not always the newest one. A material version is in the history 30
 * days or more before it starts (#1816). A version starts at midnight UTC
 * on its date, the reading the BFF gives it too, so in the United States
 * it starts on the evening before.
 *
 * The site has no clock of its own: every page is HTML built ahead of
 * time and ships no JavaScript, so `now` is the moment of the build, and
 * a daily build moves /terms and /privacy to a new version within a day
 * of its start (`.github/workflows/firebase-hosting-merge.yml`). Every
 * sentence that carries a date says when a version takes effect, never
 * that it is or is not in force yet, so a page built the day before
 * still says what is true.
 */
export function versionInForce(legalDocument: LegalDocument, now: Date): LegalVersion {
	const today = now.toISOString().slice(0, 10);
	const version = legalDocument.versions.findLast(({ effective }) => effective <= today);
	if (!version) throw new Error(`no version of the ${legalDocument.name} is in force on ${today}`);
	return version;
}

// Every version after `version`, oldest first, by date rather than by
// identity, so a version that came through a load function is matched
// too. It reads no clock: a version's page names the versions after it
// whenever it was built.
export function laterVersions(legalDocument: LegalDocument, version: LegalVersion): LegalVersion[] {
	return legalDocument.versions.filter(({ effective }) => effective > version.effective);
}

// The address of one version's own text, such as /terms/2026-11-01. The
// email before a material change links it (api/internal/legalnotice), so
// it is the new text itself and not /terms, which shows the version in
// force until the date.
export function versionPath(legalDocument: LegalDocument, version: LegalVersion): string {
	return `${legalDocument.path}/${version.effective}`;
}
