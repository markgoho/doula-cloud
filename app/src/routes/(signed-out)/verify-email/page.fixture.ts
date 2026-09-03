/*
 * Verifying an email address, as the continuum check sees it (#595).
 *
 * No token in the URL takes the "missing code" branch immediately, so
 * this route's own `fetch` (never mocked -- see the route's own code)
 * is never reached. The heading is static across every one of its three
 * states, so this is the only state worth measuring by hand.
 */
import type { RouteFixture } from '../../routeFixture.js';
import Page from './+page.svelte';

export const fixture: RouteFixture = {
	name: 'Verifying an email address',
	component: Page,
	params: {},
	url: 'https://example.test/(signed-out)/verify-email',
	readyText: 'Verify your email'
};
