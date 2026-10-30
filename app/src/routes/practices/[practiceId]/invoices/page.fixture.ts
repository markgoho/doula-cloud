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
import { practiceSession, type RouteFixture, type RouteVariant } from '../../../routeFixture.js';
import { TYPICAL_CLIENTS } from '../../../typicalPilot.js';
import type { RouteParams as RouteParameters } from './$types';
import Page from './InvoicesScreen.svelte';

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
			raisedOn: '2026-08-01',
			// #271: the by-hand rail's own reference, on the row already
			// carrying #537's hostile client-name value.
			reference: 'INV-0042',
			billingMode: 'by_hand',
			// #768: overdue, so the "Payment due" cell renders its longest
			// form -- a date plus a day count -- at every width the continuum
			// sweeps. The count is the BFF's, in the Practice's zone (#1965).
			dueOn: '2026-08-31',
			daysOverdue: 56
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
			raisedOn: '2026-07-01',
			paidOn: '2026-07-04',
			// #271: the Stripe rail's own reference, so this fixture's two
			// rows realize both billing-mode states the "Billed via" column
			// renders.
			reference: 'DC-0092',
			billingMode: 'stripe',
			// Paid, so its due date reads plainly however long ago it passed:
			// the BFF counts no lateness on a settled Invoice.
			dueOn: '2026-07-31',
			daysOverdue: 0
		}
	],
	hasMore: false,
	outstandingCents: 450_000,
	outstandingCount: 1,
	paidCents: 250_000,
	refundedCents: 125_050,
	overdueCents: 450_000,
	overdueCount: 1,
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

/*
 * Typical pilot data (#1981), the screen this route's table floor is
 * measured on (`typicalPilot.ts` says where each value comes from): eight
 * Invoices from a 14-doula agency's book, a birth fee of $1,500.00 and a
 * postpartum one of $900.00, most open and within their terms, some paid,
 * one a month overdue, billed through Stripe or by hand. A
 * four-figure amount is the most a single Invoice here ever says.
 */
export const typical: RouteVariant<RouteParameters> = {
	name: 'The Practice-wide invoice list, with typical pilot data',
	typical: true,
	props: {
		data: {
			...data,
			items: TYPICAL_CLIENTS.slice(0, 8).map((person, index) => {
				const isPaid = index % 3 === 1;
				const isOverdue = index === 2;
				const isByHand = index % 4 === 3;
				return {
					id: `inv-${index + 1}`,
					engagementId: `eng-${index + 1}`,
					contractId: `contract-${index + 1}`,
					clientName: person.name,
					status: isPaid ? 'paid' : 'open',
					amountCents: index % 3 === 2 ? 90_000 : 150_000,
					currency: 'usd',
					createdAt: `2026-09-${String(1 + index * 3).padStart(2, '0')}T15:00:00Z`,
					raisedOn: `2026-09-${String(1 + index * 3).padStart(2, '0')}`,
					...(isPaid && { paidOn: `2026-09-${String(5 + index * 3).padStart(2, '0')}` }),
					reference: isByHand ? `INV-00${String(index + 10)}` : `DC-00${String(index + 10)}`,
					billingMode: isByHand ? 'by_hand' : 'stripe',
					dueOn: `2026-09-${String(15 + index * 2).padStart(2, '0')}`,
					daysOverdue: isOverdue ? 36 : 0
				};
			}),
			// The totals the eight rows above add up to.
			outstandingCents: 630_000,
			outstandingCount: 5,
			paidCents: 450_000,
			refundedCents: 0,
			overdueCents: 90_000,
			overdueCount: 1
		} satisfies PracticeInvoiceListData
	}
};

/*
 * #2255: the screen while its read runs -- the totals' labels with a
 * placeholder for each figure, the two links, and a row skeleton.
 */
export const loading: RouteVariant<RouteParameters> = {
	name: 'The Practice-wide invoice list, while its read runs',
	props: {}
};

const owner = practiceSession(['owner']).session;

/*
 * #2293: a Practice Clients cannot pay draws the Notice above the book,
 * from the session, both while the read runs and once it lands. It bills
 * by card: one that bills by hand draws no Notice (#2200), the base's
 * tree.
 */
export const clientsCannotPay: RouteVariant<RouteParameters> = {
	name: 'The Practice-wide invoice list, at a Practice Clients cannot pay',
	pageData: { session: { ...owner, clientsCanPay: false, billingMode: 'stripe' } }
};

export const clientsCannotPayWhileLoading: RouteVariant<RouteParameters> = {
	name: 'The Practice-wide invoice list, at a Practice Clients cannot pay, while its read runs',
	props: {},
	pageData: { session: { ...owner, clientsCanPay: false, billingMode: 'stripe' } }
};

export const fixture: RouteFixture<RouteParameters> = {
	name: 'The Practice-wide invoice list',
	variants: [narrowedToOverdue, typical, loading, clientsCannotPay, clientsCannotPayWhileLoading],
	component: Page,
	params: { practiceId: 'practice-1' },
	url: 'https://example.test/practices/practice-1/invoices',
	props: { data },
	pageData: { session: { ...owner, clientsCanPay: true } },
	// The route takes its first page from `load` rather than from a fetch,
	// so the screen is on the page as soon as it renders; the heading is
	// still what proves it rendered the screen and not an error state.
	readyText: 'Invoices'
};
