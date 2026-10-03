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
import type { RequestDoulas } from '#lib/engagementRequest.js';
import { practiceSession, type RouteFixture, type RouteVariant } from '../../../../../../routeFixture.js';
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

const DOULAS_PATH = '/engagement-request-doulas';

/*
 * "Who is the Doula?" (#1596), in the three states the question renders
 * differently. Every session here is `staff-1`, `practiceSession`'s own
 * default, so the first entry of each list is the person at the form and
 * her label ends "(you)".
 *
 * `ownDoula` is what a plain Doula reads at an agency: herself and "No
 * Doula yet", nothing selected, because ADR-0008 gives her no roster and
 * she is not the only Doula. Her name is #537's hyphenated
 * double-barreled one, the widest label a two-option question gets.
 *
 * `agencyRoster` is the pilot agency (fourteen Doulas) as its Owner reads
 * it: fifteen options, nothing selected. It is the busy state, and the
 * one a 320px column has to hold. The names are a real roster's spread,
 * with the two shapes that wrap a radio label: the double-barreled
 * surname, and a long name with no hyphen to break on.
 *
 * `soloRoster` is the one case an answer is selected when the form
 * opens: the person at the form is the only Doula at the Practice.
 */
export const ownDoula: RequestDoulas = {
	items: [{ staffId: 'staff-1', name: 'Anne-Marie Ochieng-Whitfield' }],
	callerIsOnlyDoula: false
};

export const agencyRoster: RequestDoulas = {
	items: [
		{ staffId: 'staff-1', name: 'Anne-Marie Ochieng-Whitfield' },
		{ staffId: 'staff-2', name: 'Amara Okafor' },
		{ staffId: 'staff-3', name: 'Bernadette Vandenberghe-Castellanos' },
		{ staffId: 'staff-4', name: 'Chidinma Nwachukwu' },
		{ staffId: 'staff-5', name: 'Dolores Featherstonehaugh' },
		{ staffId: 'staff-6', name: 'Esperanza Villanueva' },
		{ staffId: 'staff-7', name: 'Hana Kim' },
		{ staffId: 'staff-8', name: 'Ines Abernathy' },
		{ staffId: 'staff-9', name: 'Jo Li' },
		{ staffId: 'staff-10', name: 'Marguerite Throckmorton-Balasubramanian' },
		{ staffId: 'staff-11', name: 'Priya Nair' },
		{ staffId: 'staff-12', name: 'Siobhan Fitzgerald' },
		{ staffId: 'staff-13', name: 'Wilhelmina Schwarzenberger' },
		{ staffId: 'staff-14', name: 'Zora Bell' }
	],
	callerIsOnlyDoula: false
};

export const soloRoster: RequestDoulas = {
	items: [{ staffId: 'staff-1', name: 'Anne-Marie Ochieng-Whitfield' }],
	callerIsOnlyDoula: true
};

/*
 * The approver's screen (#928), and the two things that make it a screen
 * of its own rather than the Doula's with a word changed.
 *
 * `isOwnerOrAdmin` decides both the verb -- "Start work with" against
 * "Ask to start work with", ADR-0017's solo-Practice collapse -- and
 * whether the sentence about the Credit and the balance after it (#1612)
 * is drawn above the form at all. That sentence is content the base fixture has no
 * way to reach, and it arrives through a read only an approver makes, so
 * this variant answers the balance endpoint as well as the Client. A
 * variant's `respond` replaces rather than merges, so the Client answer
 * is restated with it, and so is the Doula list: the approver reads the
 * whole roster, which is the fourteen-Doula agency here.
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
		if (path.includes(DOULAS_PATH)) return jsonResponse(agencyRoster);
		return jsonResponse(detail);
	},
	readyText: 'Start work with Persephone Ochieng-Whitfield'
};

/*
 * A solo Owner's screen (#1596): the approver's, with the one-Doula
 * roster. Its own variant because the question opens answered here and
 * nowhere else, and because it is the shortest form the screen has
 * beside the agency's longest. Its own `respond` for the reason
 * `asApprover` has one.
 *
 * Its balance is the signup bonus alone, which is a new solo Practice's,
 * so the Credit sentence is drawn in its "Welcome credits" form here and
 * in its "N Credits" form on `asApprover` (#1612).
 */
export const asSoloOwner: RouteVariant = {
	name: 'Requesting a new Engagement, as a solo Owner',
	pageData: practiceSession(['owner', 'admin', 'doula']),
	respond: (path) => {
		if (path.includes('/billing')) {
			return jsonResponse({
				balance: 3,
				ledger: {
					items: [{ origin: 'signup_bonus', quantity: 3, createdAt: '2026-10-01T00:00:00Z' }],
					hasMore: false
				}
			});
		}
		if (path.includes(DOULAS_PATH)) return jsonResponse(soloRoster);
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
	respond: (path) => jsonResponse(path.includes(DOULAS_PATH) ? ownDoula : detail),
	readyText: 'Ask to start work with Persephone Ochieng-Whitfield',
	variants: [asApprover, asSoloOwner]
};
