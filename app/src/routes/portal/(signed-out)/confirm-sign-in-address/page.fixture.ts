/*
 * Confirming a changed Client sign-in address (#619), as the continuum
 * check sees it (#595).
 *
 * A token in the URL reaches the Continue button -- the only state that
 * mounts, since spending the link is behind a click the sweep never
 * simulates (ADR-0026's GET-then-POST shape: a GET spends nothing).
 * Nothing here fetches on mount.
 */
import type { RouteFixture } from '../../../routeFixture.js';
import Page from './+page.svelte';

export const fixture: RouteFixture = {
	name: 'Confirming a changed Client sign-in address',
	component: Page,
	params: {},
	url: 'https://example.test/portal/(signed-out)/confirm-sign-in-address?token=address-change-token-1',
	readyText: 'Confirm your sign-in address'
};
