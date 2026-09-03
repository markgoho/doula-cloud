/*
 * The Settings hub, as the continuum check sees it (#595).
 *
 * A fixed five-item nav (see the route's own header comment: "a way in,
 * not a settings design") -- no fetch, and no Practice-typed content;
 * every label and description is this repo's own copy.
 */
import type { RouteFixture } from '../../../routeFixture.js';
import Page from './+page.svelte';

export const fixture: RouteFixture = {
	name: 'The Settings hub',
	component: Page,
	params: { practiceId: 'practice-1' },
	url: 'https://example.test/practices/practice-1/settings',
	readyText: 'Settings'
};
