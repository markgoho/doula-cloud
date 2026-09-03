/*
 * Accepting a Client-portal invite, as the continuum check sees it
 * (#595).
 *
 * A token in the URL reaches the credentials form -- the only state that
 * mounts, since everything else on this route is behind a submit the
 * sweep never simulates. Nothing here fetches on mount.
 */
import type { RouteFixture } from '../../../routeFixture.js';
import Page from './+page.svelte';

export const fixture: RouteFixture = {
	name: 'Accepting a Client-portal invite',
	component: Page,
	params: {},
	url: 'https://example.test/portal/(signed-out)/accept-invite?token=invite-token-1',
	readyText: 'Accept your portal invite'
};
