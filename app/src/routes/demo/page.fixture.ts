/*
 * The dev-only demo hub, as the continuum check sees it (#595).
 *
 * One static link, no fetch, no Practice-typed content -- this route
 * exists for local/e2e poking, not for anything a Practice ever sees.
 * #595 also gave it the level-1 heading it was missing (see the route's
 * own file), which is what lets it join this sweep at all.
 */
import type { RouteFixture } from '../routeFixture.js';
import Page from './+page.svelte';

export const fixture: RouteFixture = {
	name: 'The dev-only demo hub',
	component: Page,
	params: {},
	url: 'https://example.test/demo',
	readyText: 'Demo'
};
