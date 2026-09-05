/*
 * Redeeming a Client sign-in link, as the continuum check sees it (#595).
 *
 * A token in the URL reaches the Continue button -- the only state that
 * mounts, since spending the link is behind a click the sweep never
 * simulates (#617's GET-then-POST shape: a GET spends nothing). Nothing
 * here fetches on mount.
 */
import type { RouteFixture } from '../../../routeFixture.js';
import Page from './+page.svelte';

export const fixture: RouteFixture = {
	name: 'Redeeming a Client sign-in link',
	component: Page,
	params: {},
	url: 'https://example.test/portal/(signed-out)/sign-in?token=magic-link-token-1',
	readyText: 'Sign in'
};
