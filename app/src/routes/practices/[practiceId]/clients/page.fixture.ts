/*
 * The Clients list, as the continuum check sees it (#595).
 *
 * `isContractor: false` from `+page.ts`'s own load keeps the "Find or
 * add a Client" action and the search-door paragraph both on screen.
 * `name` and `email` are the table's two free-text columns -- #530's URL
 * and #537's hyphenated double-barrelled name, `DataTable`'s own #542
 * fix measured again on a different screen.
 */
import { jsonResponse } from '#lib/testResponse.js';
import type { ClientListItem } from '#lib/client.js';
import type { RouteFixture } from '../../../routeFixture.js';
import Page from './+page.svelte';

export const clients: ClientListItem[] = [
	{
		clientId: 'client-1',
		name: 'https://portal.highland-midwifery-group.example.org/referrals/2027/persephone?source=intake',
		email: 'anne-marie.ochieng-whitfield@example.test',
		hasWork: true,
		portalInviteStatus: 'sent',
		pendingRequestKinds: ['birth'],
		/*
		 * #264: both ends of the rollup on one Client -- ADR-0017's "two
		 * concurrent open Engagements, neither dropped" (the second row's
		 * own two extremes above already sits at "zero"), one line
		 * carrying every field populated (Contract, Doula, Invoice/money),
		 * the other carrying every optional field absent (no Contract yet,
		 * no Doula attached, no Invoice).
		 */
		openEngagements: [
			{
				engagementId: 'engagement-1',
				engagementStatus: 'active',
				contractStatus: 'sent',
				doulaName: 'Yolanda Okonkwo-Fitzgerald',
				invoiceStatus: 'open',
				invoiceAmountCents: 450_000
			},
			{
				engagementId: 'engagement-2',
				engagementStatus: 'intake'
			}
		]
	},
	/*
	 * A second Client, because a list of one is a list whose every column
	 * holds the same answer (#596). This one is the other end of all three
	 * of them -- no work yet, never invited, no pending Request -- so the
	 * Work column, the invite column and the empty Requests cell are all
	 * on screen for the sweep to measure and for a spec to assert on
	 * without inventing a row of its own.
	 */
	{
		clientId: 'client-2',
		name: 'Anne-Marie Ochieng-Whitfield',
		email: 'persephone@example.test',
		hasWork: false,
		pendingRequestKinds: []
	}
];

export const fixture: RouteFixture = {
	name: 'The Clients list',
	component: Page,
	params: { practiceId: 'practice-1' },
	url: 'https://example.test/practices/practice-1/clients',
	props: { data: { isContractor: false } },
	respond: () => jsonResponse({ items: clients, hasMore: false }),
	readyText: 'Clients'
};
