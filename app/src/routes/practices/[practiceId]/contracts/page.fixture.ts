/*
 * The Practice-wide "Contracts awaiting signature" and "void requests
 * waiting" lists, as the continuum check sees it (#273, #971). Mirrors
 * invoices/page.fixture.ts's shape and its two-row rule (#537/#596): the
 * Contracts table's two rows realize Draft with a hostile,
 * double-barreled Client name and Sent with a URL as the Client's name
 * -- the two states its Status column ever renders. The Void requests
 * table has no such second render branch (every row shows the same three
 * plain-text fields the same way), so one hostile row -- an unbroken URL
 * as the reason -- is what #537's rule asks for here; its own empty
 * state is exercised by the spec's own departure, the same way the
 * Contracts table's is.
 */
import type { AwaitingContract, AwaitingVoidRequest } from '#lib/contract.js';
import type { CursorPage } from '#lib/paginatedList.svelte.js';
import type { RouteFixture, RouteVariant } from '../../../routeFixture.js';
import { nth, TYPICAL_CLIENTS, TYPICAL_DOULAS } from '../../../typicalPilot.js';
import type { RouteParams as RouteParameters } from './$types';
import type { ContractsPageData } from './+page.js';
import Page from './+page.svelte';

export const contractsData: CursorPage<AwaitingContract> = {
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

export const voidRequestsData: CursorPage<AwaitingVoidRequest> = {
	items: [
		{
			requestId: 'request-1',
			engagementId: 'eng-3',
			clientId: 'client-3',
			clientName: 'Priya Okonkwo-Fitzgerald',
			requestedByName: 'Jamie Doula',
			reason:
				'https://portal.highland-midwifery-group.example.org/notes/2027/reschedule?source=client-call',
			createdAt: '2026-06-01T00:00:00Z'
		}
	],
	hasMore: false
};

export const data: ContractsPageData = { contracts: contractsData, voidRequests: voidRequestsData };

/*
 * Typical pilot data (#1981), the screen this route's two table floors are
 * measured on (`typicalPilot.ts` says where each value comes from): three
 * Contracts a 14-doula agency is waiting on, Drafts and Sent ones, and one
 * void request from a Doula with the reason in a sentence she would type.
 * The pasted URL as a name and as a reason stays on the hostile screen.
 */
export const typical: RouteVariant<RouteParameters> = {
	name: 'The Practice-wide Contracts awaiting signature list, with typical pilot data',
	typical: true,
	props: {
		data: {
			contracts: {
				items: [4, 5, 6].map((index) => {
					const person = nth(TYPICAL_CLIENTS, index);
					return {
						engagementId: `eng-${index}`,
						contractId: `contract-${index}`,
						clientId: person.clientId,
						clientName: person.name,
						status: index === 4 ? 'draft' : 'sent',
						createdAt: `2026-10-0${index}T15:00:00Z`
					};
				}),
				hasMore: false
			},
			voidRequests: {
				items: [
					{
						requestId: 'request-1',
						engagementId: 'eng-7',
						clientId: nth(TYPICAL_CLIENTS, 7).clientId,
						clientName: nth(TYPICAL_CLIENTS, 7).name,
						requestedByName: nth(TYPICAL_DOULAS, 2).name,
						reason: 'Her due date moved, so the fee changed',
						createdAt: '2026-10-08T15:00:00Z'
					}
				],
				hasMore: false
			}
		} satisfies ContractsPageData
	}
};

export const fixture: RouteFixture<RouteParameters> = {
	name: 'The Practice-wide Contracts awaiting signature list',
	variants: [typical],
	component: Page,
	params: { practiceId: 'practice-1' },
	url: 'https://example.test/practices/practice-1/contracts',
	props: { data },
	// The route takes its first page from `load` rather than from a fetch,
	// so the screen is on the page as soon as it renders; the heading is
	// still what proves it rendered the screen and not an error state.
	readyText: 'Contracts'
};
