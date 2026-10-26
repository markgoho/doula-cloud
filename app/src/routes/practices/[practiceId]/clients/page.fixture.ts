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
				contractStatus: 'sent',
				doulaName: 'Yolanda Okonkwo-Fitzgerald',
				invoiceStatus: 'open',
				invoiceAmountCents: 450_000,
				/*
				 * #2130: late, so it reads "Overdue $4,500.00", the longer
				 * of the two words an open Invoice can take ("Owes" is on
				 * the typical rows below).
				 */
				daysOverdue: 12,
				/*
				 * The third optional field, and the one the first pass
				 * missed: a fee appends a whole ` · Your fee: $1,200.00`
				 * segment, which is render-distinct under ADR-0025's
				 * fixture rule and is the longest a rollup line ever gets.
				 * The spec reaches it through a spread override, which the
				 * rule allows for a spec -- but an override is invisible to
				 * the continuum sweep, so without it here the sweep
				 * measured this column and never its busiest state.
				 *
				 * A fixture need not be role-consistent: no single reader
				 * sees both an Invoice amount and her own fee, and this row
				 * carries both on purpose, because the sweep's question is
				 * how much room the widest possible line needs.
				 */
				feeCents: 120_000
			},
			{
				engagementId: 'engagement-2',
				kind: 'postpartum',
				engagementStatus: 'intake'
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
				invoiceStatus: 'paid',
				invoiceAmountCents: 100_000
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
		portalInviteStatus: 'bounced',
		emailSuppressed: true,
		pendingRequestKinds: ['birth', 'postpartum']
	}
];

/*
 * "See everyone" on (#458, #1945): the Work column renders only here, so
 * without this variant the sweep never measured the widest the table
 * gets. The same rows as the base, through the base's own `respond`.
 */
export const withEveryone: RouteVariant = {
	name: 'The Clients list, with See everyone on',
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
		portalInviteStatus: isNew ? undefined : (index % 4 === 1 ? 'sent' : 'accepted'),
		pendingRequestKinds: index === 5 ? ['birth'] : [],
		openEngagements: isNew
			? []
			: [
					{
						engagementId: `engagement-${index + 1}`,
						kind,
						engagementStatus: 'active',
						contractStatus: 'signed',
						doulaName: doula.name,
						invoiceStatus: 'open',
						invoiceAmountCents: kind === 'birth' ? 150_000 : 90_000,
						// #2130: one Invoice on the page is late, as one at a
						// real agency is on most days.
						daysOverdue: index === 1 ? 12 : 0
					}
				]
	};
});

export const typical: RouteVariant = {
	name: 'The Clients list, with typical pilot data',
	typical: true,
	respond: () => jsonResponse({ items: typicalClients, hasMore: false })
};

export const typicalWithEveryone: RouteVariant = {
	...typical,
	name: 'The Clients list, with typical pilot data and See everyone on',
	url: withEveryone.url
};

export const fixture: RouteFixture = {
	name: 'The Clients list',
	component: Page,
	params: { practiceId: 'practice-1' },
	url: 'https://example.test/practices/practice-1/clients',
	props: { data: { isContractor: false } },
	respond: () => jsonResponse({ items: clients, hasMore: false }),
	readyText: 'Clients',
	variants: [asContractorWithNoClients, withEveryone, typical, typicalWithEveryone]
};
