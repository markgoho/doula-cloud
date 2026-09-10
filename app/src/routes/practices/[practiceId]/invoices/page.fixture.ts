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
import type { PracticeInvoiceListData } from '#lib/invoice.js';
import type { RouteFixture, RouteVariant } from '../../../routeFixture.js';
import type { RouteParams as RouteParameters } from './$types';
import Page from './+page.svelte';

/*
 * A Client's name is the one free-text column a Practice types itself, so
 * it carries #537's vocabulary: the hyphenated double-barreled name, and
 * the URL that has no break opportunity a browser will take.
 */
export const data: PracticeInvoiceListData = {
	items: [
		{
			id: 'inv-1',
			engagementId: 'eng-1',
			contractId: 'contract-1',
			clientName: 'Anne-Marie Ochieng-Whitfield',
			status: 'open',
			amountCents: 450_000,
			currency: 'usd',
			createdAt: '2026-08-01T00:00:00Z',
			// #271: the by-hand rail's own reference, on the row already
			// carrying #537's hostile client-name value.
			reference: 'INV-0042',
			billingMode: 'by_hand',
			// #768: overdue since well before this fixture's own "now", so the
			// "Payment due" cell renders its longest form -- a date plus a day
			// count -- at every width the continuum sweeps.
			dueAt: '2026-08-31T00:00:00Z'
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
			paidAt: '2026-07-04T00:00:00Z',
			// #271: the Stripe rail's own reference, so this fixture's two
			// rows realize both billing-mode states the "Billed via" column
			// renders.
			reference: 'DC-0092',
			billingMode: 'stripe',
			// Paid, so its due date reads plainly however long ago it passed.
			dueAt: '2026-07-31T00:00:00Z'
		}
	],
	hasMore: false,
	outstandingCents: 450_000,
	outstandingCount: 1,
	paidCents: 250_000,
	overdueCents: 450_000,
	overdueCount: 1,
	clientsCanPay: true,
	// #768: the unnarrowed list, so the fixture sweeps the "Payment due"
	// column and both filter links in the state a Practice meets first.
	isNarrowedToOverdue: false
};

/*
 * #768: the narrowed screen is a different tree, not a shorter one -- the
 * other filter link reads as current, and an empty narrowed list says
 * something the unnarrowed one never says. It is swept as its own subject
 * so the state a Practice reaches by pressing "Overdue" is measured too.
 */
export const narrowedToOverdue: RouteVariant<RouteParameters> = {
	name: 'The Practice-wide invoice list, narrowed to overdue',
	url: 'https://example.test/practices/practice-1/invoices?overdue=true',
	props: { data: { ...data, items: [data.items[0]], isNarrowedToOverdue: true } }
};

export const fixture: RouteFixture<RouteParameters> = {
	name: 'The Practice-wide invoice list',
	variants: [narrowedToOverdue],
	component: Page,
	params: { practiceId: 'practice-1' },
	url: 'https://example.test/practices/practice-1/invoices',
	props: { data },
	// The route takes its first page from `load` rather than from a fetch,
	// so the screen is on the page as soon as it renders; the heading is
	// still what proves it rendered the screen and not an error state.
	readyText: 'Invoices'
};
