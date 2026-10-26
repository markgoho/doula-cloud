/*
 * Accepting a Client-portal invite, as the continuum check sees it
 * (#595).
 *
 * A token in the URL reaches the Continue button -- the only state that
 * mounts, since spending the invite is behind a click the sweep never
 * simulates (#617's GET-then-POST shape: a GET spends nothing). Nothing
 * here fetches on mount.
 */
import type { RouteFixture, RouteVariant } from '../../../routeFixture.js';
import Page from './+page.svelte';

/*
 * #2025: the same screen from a link with no token in it -- one plain
 * sentence and the next step, in place of the lede and Continue.
 */
export const linkNotComplete: RouteVariant = {
	name: 'Accepting a Client-portal invite, from a link that is not complete',
	url: 'https://example.test/portal/(signed-out)/accept-invite'
};

export const fixture: RouteFixture = {
	name: 'Accepting a Client-portal invite',
	component: Page,
	params: {},
	url: 'https://example.test/portal/(signed-out)/accept-invite?token=invite-token-1',
	readyText: 'Accept your portal invite',
	variants: [linkNotComplete]
};
