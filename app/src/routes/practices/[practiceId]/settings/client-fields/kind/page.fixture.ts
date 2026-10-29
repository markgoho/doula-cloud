/*
 * The kind-of-answer question a new Client field starts with (#2193). It
 * reads nothing, so it answers no fetch.
 */
import { practiceSession, type RouteFixture } from '../../../../../routeFixture.js';
import Page from './+page.svelte';

export const fixture: RouteFixture = {
	name: 'The kind of answer for a new Client field',
	component: Page,
	params: { practiceId: 'practice-1' },
	url: 'https://example.test/practices/practice-1/settings/client-fields/kind',
	pageData: practiceSession(['owner']),
	readyText: 'What kind of answer do you need?'
};
