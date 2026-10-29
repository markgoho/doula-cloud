/*
 * The kind-of-answer question a new Plan Template question starts with
 * (#2193). It reads nothing, so it answers no fetch; the six human names
 * are the whole of its content.
 */
import { practiceSession, type RouteFixture } from '../../../../../../routeFixture.js';
import Page from './+page.svelte';

export const fixture: RouteFixture = {
	name: 'The kind of answer for a new Plan Template question',
	component: Page,
	params: { practiceId: 'practice-1', planType: 'birth_plan' },
	url: 'https://example.test/practices/practice-1/settings/plan-templates/birth_plan/kind',
	pageData: practiceSession(['owner']),
	readyText: 'What kind of answer do you need?'
};
