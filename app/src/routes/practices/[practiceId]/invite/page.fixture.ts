/*
 * Inviting a Staff member, as the continuum check sees it (#595).
 *
 * A blank form with no fetch on mount -- the address a Practice types in
 * never round-trips back onto this screen (only the confirmation notice
 * echoes it, after a submit the sweep never simulates).
 */
import type { RouteFixture } from '../../../routeFixture.js';
import Page from './+page.svelte';

export const fixture: RouteFixture = {
	name: 'Inviting a Staff member',
	component: Page,
	params: { practiceId: 'practice-1' },
	url: 'https://example.test/practices/practice-1/invite',
	readyText: 'Invite a Staff member'
};
