/*
 * Requesting a new Engagement, as the continuum check sees it (#595).
 *
 * `title` starts as the static "Start new work" and becomes
 * `submitLabel` (`Ask to start work with ${displayName}` for a Doula)
 * once the Client loads -- the two strings differ, so `readyText`
 * genuinely gates on the fetch. A Doula role keeps the Owner/Admin-only
 * balance preview out of the mount cascade entirely.
 */
import { jsonResponse } from '#lib/testResponse.js';
import type { ClientDetail } from '#lib/clientDetail.js';
import type { RouteFixture, RouteVariant } from '../../../../../../routeFixture.js';
import { practiceSession } from '../../../../../../routeFixture.js';
import Page from './+page.svelte';

export const detail: ClientDetail = {
	id: 'client-1',
	givenName: 'Persephone',
	familyName: 'Ochieng-Whitfield',
	preferredName: '',
	email: 'persephone@example.test',
	phone: '585-555-0101',
	addressLine1: '100 Highland Ave',
	addressLine2: '',
	addressLocality: 'Rochester',
	addressRegion: 'NY',
	addressPostalCode: '14620',
	dateOfBirth: '1994-03-01',
	resolvedFields: [],
	engagements: [],
	history: []
};

/*
 * The approver's screen (#928), and the two things that make it a screen
 * of its own rather than the Doula's with a word changed.
 *
 * `isOwnerOrAdmin` decides both the verb -- "Start work with" against
 * "Ask to start work with", ADR-0017's solo-Practice collapse -- and
 * whether the Credit cost and the balance after it are drawn above the
 * form at all. That `DescriptionList` is content the base fixture has no
 * way to reach, and it arrives through a read only an approver makes, so
 * this variant answers the balance endpoint as well as the Client. A
 * variant's `respond` replaces rather than merges, so the Client answer
 * is restated with it.
 *
 * `readyText` moves with the verb: the heading IS `submitLabel`, so the
 * inherited one would never appear on this branch and the mount would
 * wait for a heading that is not coming.
 *
 * Owner rather than Admin because a name has to say one of them; the
 * predicate is `isOwnerOrAdmin`, so the two draw the same screen.
 */
export const asApprover: RouteVariant = {
	name: 'Requesting a new Engagement, as an Owner',
	pageData: practiceSession(['owner']),
	respond: (path) => {
		if (path.includes('/billing')) {
			return jsonResponse({ balance: 1284, ledger: { items: [], hasMore: false } });
		}
		return jsonResponse(detail);
	},
	readyText: 'Start work with Persephone Ochieng-Whitfield'
};

export const fixture: RouteFixture = {
	name: 'Requesting a new Engagement, as a Doula',
	component: Page,
	params: { practiceId: 'practice-1', clientId: 'client-1' },
	url: 'https://example.test/practices/practice-1/clients/client-1/engagement-requests/new',
	pageData: practiceSession(['doula']),
	respond: () => jsonResponse(detail),
	readyText: 'Ask to start work with Persephone Ochieng-Whitfield',
	variants: [asApprover]
};
