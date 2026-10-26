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
import type { RouteFixture, RouteVariant } from '../../routeFixture.js';
import Page from './+page.svelte';

/*
 * #2025: the same screen from a link with no token in it -- one plain
 * sentence and the next step, in place of the credentials form.
 */
export const linkNotComplete: RouteVariant = {
	name: 'Accepting a Staff invite, from a link that is not complete',
	url: 'https://example.test/(signed-out)/accept-invite'
};

export const fixture: RouteFixture = {
	name: 'Accepting a Staff invite',
	component: Page,
	params: {},
	url: 'https://example.test/(signed-out)/accept-invite?token=invite-token-1',
	readyText: 'Accept your Staff invite',
	variants: [linkNotComplete]
};
