/*
 * Signing up a new Practice, as the continuum check sees it (#595).
 *
 * Every field starts blank and nothing fetches on mount -- what a
 * Practice eventually types here (its own name) never round-trips back
 * onto this screen, so there is no free text to carry.
 */
import type { RouteFixture } from '../../routeFixture.js';
import Page from './+page.svelte';

export const fixture: RouteFixture = {
	name: 'Signing up a new Practice',
	component: Page,
	params: {},
	url: 'https://example.test/(signed-out)/signup',
	readyText: 'Sign up your Practice'
};
