/*
 * Your account's work-state question (#2044), as the continuum check
 * sees it. The select fills from the summary's session; its width is
 * set by its longest option, "District of Columbia", not by the answer.
 */
import { jsonResponse } from '#lib/testResponse.js';
import type { RouteFixture } from '../../../routeFixture.js';
import { session } from '../page.fixture.js';
import Page from './+page.svelte';

export const fixture: RouteFixture = {
	name: 'Your account, changing where you work',
	component: Page,
	params: {},
	url: 'https://example.test/account/work-state',
	respond: () => jsonResponse(session),
	readyText: 'Which state do you work from?'
};
