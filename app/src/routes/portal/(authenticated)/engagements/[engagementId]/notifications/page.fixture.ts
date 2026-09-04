/*
 * The Client-portal Notifications settings screen (#303), as the
 * continuum check sees it (#595).
 */
import { jsonResponse } from '#lib/testResponse.js';
import type { RouteFixture } from '../../../../../routeFixture.js';
import Page from './+page.svelte';

export const fixture: RouteFixture = {
	name: 'The Client-portal Notifications settings screen',
	component: Page,
	params: { engagementId: 'engagement-1' },
	url: 'https://example.test/portal/engagements/engagement-1/notifications',
	pageData: { practiceName: 'Riverside Doula Collective' },
	respond: () => jsonResponse({ enabled: false }),
	readyText: 'Notifications'
};
