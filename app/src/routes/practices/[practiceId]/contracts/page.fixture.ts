/*
 * The Practice-wide "Contracts awaiting signature" list, as the continuum
 * check sees it (#273). Mirrors invoices/page.fixture.ts's shape and its
 * two-row rule (#537/#596): one row realizing Draft with a hostile,
 * double-barrelled Client name, one realizing Sent with a URL as the
 * Client's name -- the two states the Status column ever renders, each
 * carrying content a Practice types itself rather than a file.
 */
import type { AwaitingContract } from '#lib/contract.js';
import type { CursorPage } from '#lib/paginatedList.svelte.js';
import type { RouteFixture } from '../../../routeFixture.js';
import type { RouteParams as RouteParameters } from './$types';
import Page from './+page.svelte';

export const data: CursorPage<AwaitingContract> = {
	items: [
		{
			engagementId: 'eng-1',
			contractId: 'contract-1',
			clientId: 'client-1',
			clientName: 'Anne-Marie Ochieng-Whitfield',
			status: 'draft',
			createdAt: '2026-08-01T00:00:00Z'
		},
		{
			engagementId: 'eng-2',
			contractId: 'contract-2',
			clientId: 'client-2',
			clientName:
				'https://portal.highland-midwifery-group.example.org/referrals/2027/persephone?source=intake',
			status: 'sent',
			createdAt: '2026-07-01T00:00:00Z'
		}
	],
	hasMore: false
};

export const fixture: RouteFixture<RouteParameters> = {
	name: 'The Practice-wide Contracts awaiting signature list',
	component: Page,
	params: { practiceId: 'practice-1' },
	url: 'https://example.test/practices/practice-1/contracts',
	props: { data },
	// The route takes its first page from `load` rather than from a fetch,
	// so the screen is on the page as soon as it renders; the heading is
	// still what proves it rendered the screen and not an error state.
	readyText: 'Contracts'
};
