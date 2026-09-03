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

const clients: ClientListItem[] = [
	{
		clientId: 'client-1',
		name: 'https://portal.highland-midwifery-group.example.org/referrals/2027/persephone?source=intake',
		email: 'anne-marie.ochieng-whitfield@example.test',
		hasWork: true,
		portalInviteStatus: 'sent',
		pendingRequestKinds: ['birth']
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
