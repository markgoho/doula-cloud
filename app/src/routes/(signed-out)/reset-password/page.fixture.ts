/*
 * Reset-password, as the continuum check sees it (#595).
 *
 * A one-field form with no fetch on mount and no Practice-typed content:
 * the token it reads off the URL is never rendered.
 */
import type { RouteFixture } from '../../routeFixture.js';
import Page from './+page.svelte';

export const fixture: RouteFixture = {
	name: 'Reset your password',
	component: Page,
	params: {},
	url: 'https://example.test/(signed-out)/reset-password?token=reset-token-1',
	readyText: 'Reset your password'
};
