/*
 * Spending a recovery code, as the continuum check sees it (#595, #694).
 *
 * The screen fetches nothing on mount -- everybody who reaches it is
 * signed out -- so `respond` answers the one call it can ever make, the
 * spend itself, and never runs during a sweep.
 */
import { jsonResponse } from '#lib/testResponse.js';
import type { RouteFixture } from '../../routeFixture.js';
import Page from './+page.svelte';

export const fixture: RouteFixture = {
	name: 'Use a recovery code',
	component: Page,
	params: {},
	url: 'https://example.test/recovery-code',
	respond: () => jsonResponse(undefined, 204),
	readyText: 'Use a recovery code'
};
