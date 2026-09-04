/*
 * The pending Engagement Requests inbox, as the continuum check sees it
 * (#595).
 *
 * `clientName` and `requestedByName` are the table's two free-text
 * columns -- #530's URL and #537's hyphenated double-barrelled name,
 * the same values `DataTable`'s own `max-inline-size`/`overflow-wrap`
 * fix (#542) was written against.
 *
 * Two Requests, not one (#596): a birth ask carries a due date and a
 * postpartum ask does not, so an inbox of one kind never shows the
 * "Not given" cell a real inbox always has in it, and #537 is the
 * argument that measuring such a screen measures nothing.
 */
import { jsonResponse } from '#lib/testResponse.js';
import type { PendingRequestItem } from '#lib/engagementRequest.js';
import type { RouteFixture } from '../../../routeFixture.js';
import Page from './+page.svelte';

export const requests: PendingRequestItem[] = [
	{
		requestId: 'request-1',
		clientId: 'client-1',
		clientName: 'https://portal.highland-midwifery-group.example.org/referrals/2027/persephone?source=intake',
		kind: 'birth',
		dueDate: '2027-03-01',
		requestedByName: 'Anne-Marie Ochieng-Whitfield',
		requestedAt: '2026-08-01T10:00:00Z'
	},
	{
		// A postpartum ask names no due date, so its Due cell says so
		// rather than going blank. Its Client is named for the Practice
		// that registered it, which is what a Practice types into a free
		// text column as often as it types a person's name.
		requestId: 'request-2',
		clientId: 'client-2',
		clientName: 'Riverside Doula Collective',
		kind: 'postpartum',
		requestedByName: 'Persephone Ochieng-Whitfield',
		requestedAt: '2026-08-02T10:00:00Z'
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
