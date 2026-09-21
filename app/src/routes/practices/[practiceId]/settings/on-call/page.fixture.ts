/*
 * The on-call rule screen, as the continuum check sees it (#1093).
 *
 * It opens on the gestational-week rule, which is the branch that
 * renders the most: the week field only exists under that rule, so the
 * other branch is a strict subset of this one and realizes nothing the
 * sweep has not already measured. The hint lines are the widest content
 * on the screen, and they are the sentences the product actually says.
 */
import { jsonResponse } from '#lib/testResponse.js';
import type { OnCallSettings } from '#lib/onCall.js';
import type { RouteFixture } from '../../../../routeFixture.js';
import Page from './+page.svelte';

export const settings: OnCallSettings = {
	startRule: 'gestational_week',
	startWeek: 37,
	graceDays: 14
};

export const fixture: RouteFixture = {
	name: 'The on-call rule',
	component: Page,
	params: { practiceId: 'practice-1' },
	url: 'https://example.test/practices/practice-1/settings/on-call',
	respond: () => jsonResponse(settings),
	readyText: 'On call'
};
