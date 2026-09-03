/*
 * Finding a Client, as the continuum check sees it (#595).
 *
 * `isContractor: false` from `+page.ts`'s own load takes the real search
 * form rather than the contractor-Doula explain-only door. No fetch
 * fires until a search is submitted, which the sweep never simulates, so
 * there is no Practice-typed free text on screen at mount.
 */
import type { RouteFixture } from '../../../../routeFixture.js';
import Page from './+page.svelte';

export const fixture: RouteFixture = {
	name: 'Finding a Client',
	component: Page,
	params: { practiceId: 'practice-1' },
	url: 'https://example.test/practices/practice-1/clients/search',
	props: { data: { isContractor: false } },
	readyText: 'Find a Client'
};
