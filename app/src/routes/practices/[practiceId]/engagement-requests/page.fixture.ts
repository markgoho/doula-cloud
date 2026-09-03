/*
 * The pending Engagement Requests inbox, as the continuum check sees it
 * (#595).
 *
 * `clientName` and `requestedByName` are the table's two free-text
 * columns -- #530's URL and #537's hyphenated double-barrelled name,
 * the same values `DataTable`'s own `max-inline-size`/`overflow-wrap`
 * fix (#542) was written against.
 */
import { jsonResponse } from '#lib/testResponse.js';
import type { PendingRequestItem } from '#lib/engagementRequest.js';
import type { RouteFixture } from '../../../routeFixture.js';
import Page from './+page.svelte';

const requests: PendingRequestItem[] = [
	{
		requestId: 'request-1',
		clientId: 'client-1',
		clientName: 'https://portal.highland-midwifery-group.example.org/referrals/2027/persephone?source=intake',
		kind: 'birth',
		dueDate: '2027-03-01',
		requestedByName: 'Anne-Marie Ochieng-Whitfield',
		requestedAt: '2026-08-01T10:00:00Z'
	}
];

export const fixture: RouteFixture = {
	name: 'The pending Engagement Requests inbox',
	component: Page,
	params: { practiceId: 'practice-1' },
	url: 'https://example.test/practices/practice-1/engagement-requests',
	respond: () => jsonResponse({ items: requests, hasMore: false }),
	readyText: 'Requests awaiting approval'
};
