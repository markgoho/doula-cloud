/*
 * The Clients list, as the continuum check sees it (#595).
 *
 * `isContractor: false` from `+page.ts`'s own load keeps the "Find or
 * add a Client" action and the search-door paragraph both on screen.
 * `name` is the table's free-text column -- #530's URL and #537's
 * hyphenated double-barreled name, `DataTable`'s own #542 fix measured
 * again on a different screen. Email and the portal invite left the list
 * for the Client hub on #2022; they stay on the rows because they are
 * the list's wire shape.
 */
import { jsonResponse } from '#lib/testResponse.js';
import type { ClientListItem } from '#lib/client.js';
import type { RouteFixture, RouteVariant } from '../../../routeFixture.js';
import { nth, TYPICAL_CLIENTS, TYPICAL_DOULAS } from '../../../typicalPilot.js';
import Page from './+page.svelte';

// A calendar day, `YYYY-MM-DD`, this many days before today: a live birth's postpartum words
// count from now, so the fixture holds the day that makes them the same words whenever it runs.
function daysAgo(days: number): string {
	const day = new Date();
	day.setDate(day.getDate() - days);
	return `${day.getFullYear()}-${String(day.getMonth() + 1).padStart(2, '0')}-${String(day.getDate()).padStart(2, '0')}`;
}

export const clients: ClientListItem[] = [
	{
		clientId: 'client-1',
		name: 'https://portal.highland-midwifery-group.example.org/referrals/2027/persephone?source=intake',
		email: 'anne-marie.ochieng-whitfield@example.test',
		hasWork: true,
		portalInviteStatus: 'invited',
		portalInviteDelivery: 'sent',
		pendingRequestKinds: ['birth'],
		/*
		 * #264: both ends of the rollup on one Client -- ADR-0017's "two
		 * concurrent open Engagements, neither dropped" (the second row's
		 * own two extremes above already sits at "zero"), one line
		 * carrying every field populated (Contract, Doula, Invoice/money),
		 * the other carrying every optional field absent (no Contract yet,
		 * no Doula attached, no Invoice). A third line (#741) covers the
		 * one further render-distinct branch neither extreme reaches: an
		 * `invoiceStatus` other than `open` now renders as "Invoice:
		 * <status>" rather than "Owes", a different template, not
		 * merely a different word through the same one -- so it needs its
		 * own line, the same reasoning ADR-0025 gives for adding a third
		 * row only where a field's own render branches a third way.
		 */
		openEngagements: [
			{
				engagementId: 'engagement-1',
				kind: 'birth',
				engagementStatus: 'active',
				dueDate: '2027-01-10',
				contractStatus: 'sent',
				// #2232: two Doulas, the longest a line's Doula text gets.
				doulaNames: ['Yolanda Okonkwo-Fitzgerald', 'Anne-Marie Ochieng-Whitfield'],
				invoiceStatus: 'open',
				invoiceAmountCents: 450_000,
				/*
				 * #2130: late, so it reads "Overdue $4,500.00", the longer
				 * of the two words an open Invoice can take ("Owes" is on
				 * the typical rows below).
				 */
				daysOverdue: 12
			},
			/*
			 * The third optional field: her own fee, which draws in the Money
			 * column as "Your fee $1,200.00" and is render-distinct from an
			 * Invoice's amount and state. A fixture need not be
			 * role-consistent: no single reader sees both, and this row
			 * carries both on purpose, because the sweep's question is how
			 * much room the widest possible line needs.
			 */
			{
				engagementId: 'engagement-2',
				kind: 'postpartum',
				engagementStatus: 'intake',
				feeCents: 120_000
			},
			/*
			 * #741: the settled-Contract branch -- a deposit Invoice paid
			 * off alongside its balance, so nothing is outstanding any
			 * longer and the line reads the old "Invoice: Paid" wording
			 * rather than "Owes".
			 */
			{
				engagementId: 'engagement-3',
				kind: 'birth',
				engagementStatus: 'active',
				dueDate: '2027-02-21',
				invoiceStatus: 'paid',
				invoiceAmountCents: 100_000
			}
		]
	},
	/*
	 * A second Client, because a list of one is a list whose every column
	 * holds the same answer (#596). This one is the other end of all three
	 * of them -- not yet in care, never invited, no pending Request -- so the
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
	},
	/*
	 * A third Client for the two columns whose longest real value neither
	 * row above holds (#1945): the Pending request column with a Request
	 * of each kind ("Birth & Postpartum request pending"), and the Portal
	 * invite column in its blocked branch, the longest invite words there
	 * are plus the "Blocked email addresses" link (#785) -- a branch that
	 * renders differently, so it earns a row of its own.
	 */
	{
		clientId: 'client-3',
		name: 'Persephone Adebayo-Lindqvist',
		email: 'persephone.adebayo-lindqvist@example.test',
		hasWork: true,
		portalInviteStatus: 'invited',
		portalInviteDelivery: 'bounced',
		emailSuppressed: true,
		pendingRequestKinds: ['birth', 'postpartum']
	},
	/*
	 * A fourth Client for the Due column's two ended states, which no row
	 * above reaches: once a pregnancy has ended the cell reads "26 weeks postpartum"
	 * for a live birth and the neutral "Ended <day>" for a loss or an outcome
	 * nobody learned, in place of a due date or the dash. One row holds both,
	 * as two lines, so each carries its kind's prefix, the longest a Due line
	 * gets. A loss and an unknown outcome render the same words, so only the
	 * loss is here. The name is hostile in the way #537 asks for: long, and
	 * hyphenated on both sides.
	 */
	{
		clientId: 'client-4',
		name: 'Wilhelmina Featherstonehaugh-Abernathy-Villiers',
		email: 'wilhelmina.featherstonehaugh-abernathy@example.test',
		hasWork: true,
		pendingRequestKinds: [],
		openEngagements: [
			{
				engagementId: 'engagement-4',
				kind: 'postpartum',
				engagementStatus: 'active',
				birthOutcome: 'live_birth',
				pregnancyEndedOn: daysAgo(26 * 7 + 3)
			},
			{
				engagementId: 'engagement-5',
				kind: 'birth',
				engagementStatus: 'active',
				birthOutcome: 'loss',
				pregnancyEndedOn: '2026-11-30'
			}
		]
	}
];

/*
 * "Also show Clients not yet in care" on (#458, #1945): the Work column renders only here, so
 * without this variant the sweep never measured the widest the table
 * gets. The same rows as the base, through the base's own `respond`.
 */
export const withEveryone: RouteVariant = {
	name: 'The Clients list, with Also show anyone not yet in care on',
	url: 'https://example.test/practices/practice-1/clients?all=true'
};

/*
 * The second axis, and the one no row set can reach (#928): who is
 * looking, not what the data holds. `isAmbientContractor` arrives through
 * `+page.ts`'s own `load` as `data.isContractor`, so this restates
 * `props` -- setting a session in `pageData` here would mount the base
 * tree a second time and the sweep would stay green about it.
 *
 * Her list is empty on purpose, and that is the whole reason this variant
 * exists. A contractor Doula holding attached Clients reads the base's
 * table with the "Find or add a Client" action taken away -- a strict
 * subset, realizing nothing the sweep has not measured. A contractor
 * Doula holding none reads two sentences no other session can reach:
 * ADR-0017's "Work reaches you as an Offer, so there are no Clients here
 * yet." in place of "No Clients yet.", and the search door put back
 * underneath as "How to add Clients of your own" -- the link #539 had to
 * restore after taking the header action away from her.
 *
 * Both of those are this repo's own copy and neither holds a
 * Practice-typed value, so there is no hostile content to choose for this
 * branch: what it renders that the base does not is two fixed strings.
 */
export const asContractorWithNoClients: RouteVariant = {
	name: 'The Clients list, as a contractor Doula with nobody attached',
	props: { data: { isContractor: true } },
	respond: () => jsonResponse({ items: [], hasMore: false })
};

/*
 * Typical pilot data (#1981), the screen this route's table floor is
 * measured on (`typicalPilot.ts` says where each value comes from). An
 * Owner's first page of a 14-doula agency's book: most Clients carry one
 * open Engagement with its Contract signed, its Doula on it and an
 * Invoice outstanding, one has just been entered and has nothing yet, one
 * carries a pending birth Request, and the invites are the ordinary ones
 * (accepted, sent, never sent). The rare long branches -- a blocked
 * invite, a Request of each kind, two open Engagements -- stay on the
 * hostile rows above, where the sweep still holds them inside the frame.
 */
const typicalClients: ClientListItem[] = TYPICAL_CLIENTS.slice(0, 8).map((person, index) => {
	const doula = nth(TYPICAL_DOULAS, index + 2);
	const isNew = index === 3;
	const kind = index % 3 === 2 ? 'postpartum' : 'birth';
	return {
		clientId: person.clientId,
		name: person.name,
		email: person.email,
		hasWork: !isNew,
		portalInviteStatus: isNew ? undefined : (index % 4 === 1 ? 'invited' : 'accepted'),
		portalInviteDelivery: !isNew && index % 4 === 1 ? 'sent' : undefined,
		pendingRequestKinds: index === 5 ? ['birth'] : [],
		// The Client with a pending birth Request has no Engagement yet: the
		// Request is what asks for one (ADR-0017), so her row holds the one badge.
		openEngagements: isNew || index === 5
			? []
			: [
					{
						engagementId: `engagement-${index + 1}`,
						kind,
						engagementStatus: 'active',
						// Soonest first, as the BFF sends them.
						dueDate: kind === 'postpartum' ? undefined : `2027-0${index + 1}-14`,
						...(kind === 'postpartum' && { birthOutcome: 'live_birth', pregnancyEndedOn: daysAgo(22) }),
						contractStatus: 'signed',
						doulaNames: [doula.name],
						invoiceStatus: 'open',
						invoiceAmountCents: kind === 'birth' ? 150_000 : 90_000,
						// #2130: one Invoice on the page is late, as one at a
						// real agency is on most days.
						daysOverdue: index === 1 ? 12 : 0
					}
				]
	};
});

/*
 * The same page with what the demo Practice holds that a typical page does
 * not: two Doulas on one Engagement, a loss's "Ended" date, an overdue badge,
 * and one row with two Doulas and an overdue badge together. A swept typical variant (`typicalBusiest`) -- the continuum check
 * measures the typical page and the hostile rows; the Clients list spec holds
 * the rows to one height here.
 */
const twoDoulas = { doulaNames: ['Joss Adeyemi', 'Lena Vasquez'] };
const ended = { dueDate: undefined, birthOutcome: 'loss' as const, pregnancyEndedOn: '2026-11-02' };
const overdue = { daysOverdue: 12 };
const busiestLine = new Map<number, object>([
	[0, twoDoulas],
	[2, ended],
	[4, { ...twoDoulas, ...overdue }],
	// The demo's Imani: a Postpartum line, Active, Contract signed, and an overdue badge.
	[6, { kind: 'postpartum' as const, ...overdue, dueDate: undefined, birthOutcome: 'live_birth' as const, pregnancyEndedOn: daysAgo(45) }]
]);

export const seedLikeClients: ClientListItem[] = typicalClients.map((client, index) => {
	const [line] = client.openEngagements ?? [];
	const busiest = busiestLine.get(index);
	return line && busiest ? { ...client, openEngagements: [{ ...line, ...busiest }] } : client;
});

export const typical: RouteVariant = {
	name: 'The Clients list, with typical pilot data',
	typical: true,
	respond: () => jsonResponse({ items: typicalClients, hasMore: false })
};

/*
 * The demo Practice's busiest rows, swept as a typical screen too: the table's
 * floor is the width the longest Status line, a pair of Doulas and an "Ended"
 * date need together, since one container query serves every Client list.
 */
export const typicalBusiest: RouteVariant = {
	name: 'The Clients list, with the demo Practice’s busiest rows',
	typical: true,
	respond: () => jsonResponse({ items: seedLikeClients, hasMore: false })
};

export const typicalWithEveryone: RouteVariant = {
	...typical,
	name: 'The Clients list, with typical pilot data and Also show anyone not yet in care on',
	url: withEveryone.url
};

/*
 * #2624: a Doula who is not a contractor is also offered "Mine" in the
 * filter bar, beside the switch for anyone not yet in care.
 * With it on the bar draws one switch fewer, a strict subset of this
 * one, so that state is not declared.
 */
export const asEmployeeDoula: RouteVariant = {
	name: 'The Clients list, as an employee Doula',
	props: { data: { isContractor: false, isDoula: true } }
};

export const fixture: RouteFixture = {
	name: 'The Clients list',
	component: Page,
	params: { practiceId: 'practice-1' },
	url: 'https://example.test/practices/practice-1/clients',
	props: { data: { isContractor: false } },
	respond: () => jsonResponse({ items: clients, hasMore: false }),
	readyText: 'Clients',
	variants: [asContractorWithNoClients, withEveryone, typical, typicalBusiest, typicalWithEveryone, asEmployeeDoula]
};
