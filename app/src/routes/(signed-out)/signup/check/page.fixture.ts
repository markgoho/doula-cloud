/*
 * The end of setting up a Practice, as the continuum check sees it (#595,
 * #2049): every answer at its longest, and the busiest agreement.
 *
 * The answers are the journey's own draft, which the pages before this
 * one fill. The sweep mounts this route with nothing in front of it, so
 * the fixture fills the draft at import, the way `detailsFixture.ts`
 * does for a Client's details journey.
 *
 * The load hands it the coming versions of the Terms and the Privacy
 * Policy (#1824), and the fixture holds the busiest list: two versions
 * of the Terms and one of the Privacy Policy. Outside a notice period
 * there is no list, a strict subset of this tree, so it is not a variant.
 */
import type { RouteFixture } from '../../../routeFixture.js';
import { signupDraft } from '../signupJourney.svelte.js';
import type { SignupData } from './+page.js';
import Page from './+page.svelte';

export const answers = {
	firstName: 'Anne-Marie',
	lastName: 'Ochieng-Whitfield',
	email: 'anne-marie.ochieng-whitfield@highland-midwifery-group.example.org',
	practiceName: 'Highland Midwifery and Birth Doula Collective of Monroe County',
	workStateName: 'District of Columbia',
	timezone: 'America/Phoenix'
};

/**
Fills the draft the way the three pages before this one would.
*/
export function seedSignup(): void {
	signupDraft.clear();
	signupDraft.update(answers);
	signupDraft.password = 'correct horse battery staple';
	for (const id of ['you', 'practice', 'where'] as const) signupDraft.visit(id);
}

// eslint-disable-next-line unicorn/no-top-level-side-effects -- installing state IS what a fixture does: the sweep mounts this route without the pages that fill it, and the module has no other moment to do it in.
seedSignup();

export const data: SignupData = {
	coming: [
		{ document: 'terms', name: 'Terms of Service', version: '2026-11-30' },
		{ document: 'terms', name: 'Terms of Service', version: '2026-12-15' },
		{ document: 'privacy', name: 'Privacy Policy', version: '2026-12-01' }
	]
};

export const fixture: RouteFixture = {
	name: 'Setting up a Practice: check your answers',
	component: Page,
	params: {},
	url: 'https://example.test/signup/check',
	props: { data },
	readyText: 'Check your answers'
};
