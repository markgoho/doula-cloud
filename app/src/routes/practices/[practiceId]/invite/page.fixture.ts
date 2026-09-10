/*
 * Inviting a Staff member, as the continuum check sees it (#595).
 *
 * A blank form with no fetch on mount -- the address a Practice types in
 * never round-trips back onto this screen (only the confirmation notice
 * echoes it, after a submit the sweep never simulates).
 *
 * One session is all this screen has (#928). It reads no session at all:
 * `roles.js` is never imported here and the fixture carries no `pageData`,
 * so the same form is drawn for whoever reaches it and there is nothing
 * for a variant to vary. Who may reach it is the invitation endpoint's
 * own refusal, which happens after a submit and off this screen.
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
