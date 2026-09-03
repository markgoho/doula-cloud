/*
 * The pre-account Offer read, as the continuum check sees it (#595).
 *
 * A token with no code yet shows only the six-digit access-code form --
 * `apiFetch` fires on submit, never on mount, so nothing here needs a
 * `respond`. The Offer's own free text (`terms`) only reaches the screen
 * after that submit, which the sweep never simulates (see the two
 * existing fixtures' own comments on measuring only what mounts).
 */
import type { RouteFixture } from '../../../routeFixture.js';
import Page from './+page.svelte';

export const fixture: RouteFixture = {
	name: 'The pre-account Offer read',
	component: Page,
	params: { offerId: 'offer-1' },
	url: 'https://example.test/(signed-out)/offers/offer-1?token=offer-token-1',
	readyText: 'An offer of work'
};
