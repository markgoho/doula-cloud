/*
 * Forgot-password, as the continuum check sees it (#595).
 *
 * A one-field form with no fetch on mount and no Practice-typed content
 * anywhere on it -- every word on this screen is this repo's own copy.
 */
import type { RouteFixture } from '../../routeFixture.js';
import Page from './+page.svelte';

export const fixture: RouteFixture = {
	name: 'Forgot your password',
	component: Page,
	params: {},
	url: 'https://example.test/(signed-out)/forgot-password',
	readyText: 'Forgot your password?'
};
