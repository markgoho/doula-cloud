/*
 * Accepting a Staff invite, as the continuum check sees it (#595).
 *
 * Step one asks only for credentials -- nothing about who is inviting or
 * being invited is known yet (see the route's own header comment), so
 * there is no Practice-typed free text on screen until she submits. A
 * token in the URL is enough to reach step one rather than the "missing
 * token" refusal; no `respond` is needed since nothing here fetches on
 * mount.
 */
import type { RouteFixture } from '../../routeFixture.js';
import Page from './+page.svelte';

export const fixture: RouteFixture = {
	name: 'Accepting a Staff invite',
	component: Page,
	params: {},
	url: 'https://example.test/(signed-out)/accept-invite?token=invite-token-1',
	readyText: 'Accept your Staff invite'
};
