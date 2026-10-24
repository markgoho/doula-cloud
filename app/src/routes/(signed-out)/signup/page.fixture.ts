/*
 * Signing up a new Practice, as the continuum check sees it (#595).
 *
 * Every field starts blank -- what a Practice eventually types here (its
 * own name) never round-trips back onto this screen, so there is no free
 * text to carry. The one thing the load hands it is the list of coming
 * versions of the Terms and the Privacy Policy (#1824), and the fixture
 * holds the busiest one: two versions of the Terms and one of the
 * Privacy Policy, so the line under the agreement sentence is as long as
 * it gets. The line's absence, outside a notice period, is a strict
 * subset of this tree, so it is not declared as a variant.
 */
import type { RouteFixture } from '../../routeFixture.js';
import type { SignupData } from './+page.js';
import Page from './+page.svelte';

export const data: SignupData = {
	coming: [
		{ document: 'terms', name: 'Terms of Service', version: '2026-11-30' },
		{ document: 'terms', name: 'Terms of Service', version: '2026-12-15' },
		{ document: 'privacy', name: 'Privacy Policy', version: '2026-12-01' }
	]
};

export const fixture: RouteFixture = {
	name: 'Signing up a new Practice',
	component: Page,
	params: {},
	url: 'https://example.test/(signed-out)/signup',
	props: { data },
	readyText: 'Sign up your Practice'
};
