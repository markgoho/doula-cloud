/*
 * The Practice-wide Invoice list, as the continuum check sees it (#570).
 *
 * This is the screen the reach trial on #551 shipped, and the trial's
 * finding was that telling whether it laid out correctly took a
 * hand-written throwaway harness -- `research/reach-trial-551-harness` --
 * because the repo's own check could not sweep it. This fixture is that
 * harness's content, moved into the check.
 *
 * The trial measured it holding at every width from 1400px down to 280px
 * including a URL in its only free-text column, and it holds because #542
 * gave `DataTable`'s cells `max-inline-size: var(--measure)` and
 * `overflow-wrap: anywhere`. So this entry is not only a subject: it is
 * the regression test for #542, on a real screen rather than a demo.
 */
import type { PracticeInvoicePage } from '#lib/invoice.js';
import type { RouteFixture } from '../../../routeFixture.js';
import type { RouteParams as RouteParameters } from './$types';
import Page from './+page.svelte';

/*
 * A Client's name is the one free-text column a Practice types itself, so
 * it carries #537's vocabulary: the hyphenated double-barrelled name, and
 * the URL that has no break opportunity a browser will take.
 */
export const data: PracticeInvoicePage = {
	items: [
		{
			id: 'inv-1',
			engagementId: 'eng-1',
			contractId: 'contract-1',
			clientName: 'Anne-Marie Ochieng-Whitfield',
			status: 'open',
			amountCents: 450_000,
			currency: 'usd',
			createdAt: '2026-08-01T00:00:00Z'
		},
		{
			id: 'inv-2',
			engagementId: 'eng-2',
			contractId: 'contract-2',
			clientName:
				'https://portal.highland-midwifery-group.example.org/referrals/2027/persephone?source=intake',
			status: 'paid',
			amountCents: 250_000,
			currency: 'usd',
			createdAt: '2026-07-01T00:00:00Z',
			paidAt: '2026-07-04T00:00:00Z'
		}
	],
	hasMore: false,
	outstandingCents: 450_000,
	outstandingCount: 1,
	paidCents: 250_000
};

export const fixture: RouteFixture<RouteParameters> = {
	name: 'The Practice-wide invoice list',
	component: Page,
	params: { practiceId: 'practice-1' },
	url: 'https://example.test/practices/practice-1/invoices',
	props: { data },
	// The route takes its first page from `load` rather than from a fetch,
	// so the screen is on the page as soon as it renders; the heading is
	// still what proves it rendered the screen and not an error state.
	readyText: 'Invoices'
};
