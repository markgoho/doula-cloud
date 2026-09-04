/*
 * The Client-portal Engagement hub, as the continuum check sees it
 * (#595).
 *
 * `practiceName` reaches this screen two ways at once: through the
 * ancestor `+layout.ts`'s `page.data` (the title, `RecordDetail`'s
 * `serviceName`) and through this route's own `onMount` fetch (the
 * summary's facts). Both carry #530's own URL, since a Practice's
 * registered name is exactly the value that broke a grid track there.
 */
import { jsonResponse } from '#lib/testResponse.js';
import type { RouteFixture } from '../../../../routeFixture.js';
import Page from './+page.svelte';

export const practiceName =
	'https://portal.highland-midwifery-group.example.org/referrals/2027/persephone?source=intake';

export const detail = {
	engagementId: 'engagement-1',
	practiceName,
	clientName: 'Anne-Marie Ochieng-Whitfield',
	status: 'active',
	dueDate: '2027-03-01'
};

export const fixture: RouteFixture = {
	name: 'The Client-portal Engagement hub',
	component: Page,
	params: { engagementId: 'engagement-1' },
	url: 'https://example.test/portal/engagements/engagement-1',
	pageData: { practiceName },
	// #486: the Activity ledger's own read shares this route's one mocked
	// fetcher, and needs the cursor-list envelope docs/api-design.md
	// section 4 asks for -- the bare `detail` shape above has no `items`,
	// which is exactly what crashed DataTable when this fixture answered
	// every path with it alike.
	respond: (path) =>
		jsonResponse(path.includes('/activity') ? { items: [], hasMore: false } : detail),
	readyText: `Welcome to ${practiceName}`
};
