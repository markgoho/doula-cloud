/*
 * The Client detail hub, as the continuum check sees it (#595).
 *
 * `title` is `displayName(detail)`, gated on load the same way the
 * approval-screen fixture's is. Address and history rows carry #537's
 * vocabulary again -- a Client's own address line and a fellow Staff
 * member's name are exactly the free-text shapes this map keeps finding.
 * `isContractor: false` from `+page.ts`'s own load (#465) keeps "Start
 * new work with <name>" on screen, same rationale as the Clients list
 * fixture's own `data`.
 *
 * `resolvedFields` and `history` are each a row set the route's own
 * markup branches on, neither delegated to a component swept elsewhere
 * (#720): a `section_header` Field renders only a Heading while every
 * other type renders a DescriptionList row, so one of each is here, the
 * value row carrying an archived field's `note` too and #530's own URL
 * as the Practice's own question label. `history` merges two shapes at
 * once (`client_event`/`engagement_request`, see the route's own
 * `historyWho`/`historyWhat`) and ADR-0017 lets both kinds of Request be
 * pending together, so this holds one of each history type plus both
 * kinds of pending Request -- one requested by the signed-in Staff
 * member (`staff-1`, matching `respond`'s own session answer below, so
 * the Withdraw button this page conditions on `requestedBy === staffId`
 * actually renders) and one requested by somebody else.
 */
import { jsonResponse } from '#lib/testResponse.js';
import type { ClientDetail } from '#lib/clientDetail.js';
import type { EraseEligibility } from '#lib/clientErasure.js';
import type { RouteFixture, RouteVariant } from '../../../../routeFixture.js';
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
	resolvedFields: [
		{ fieldId: 'field-1', type: 'section_header', label: 'Birth history' },
		{
			fieldId: 'field-2',
			type: 'single_select',
			label: 'https://portal.highland-midwifery-group.example.org/referrals/2027/persephone?source=intake',
			value: 'Epidural, as early as it can be given',
			note: 'No longer collected'
		}
	],
	engagements: [{ engagementId: 'engagement-1', kind: 'birth', status: 'active', createdAt: '2026-08-01T00:00:00Z' }],
	history: [
		{
			type: 'engagement_request',
			at: '2026-08-01T00:00:00Z',
			engagementRequest: {
				requestId: 'request-1',
				kind: 'birth',
				state: 'pending',
				requestedBy: 'staff-1',
				requestedByName: 'Anne-Marie Ochieng-Whitfield',
				requestedAt: '2026-08-01T00:00:00Z'
			}
		},
		{
			type: 'engagement_request',
			at: '2026-08-01T00:00:00Z',
			engagementRequest: {
				requestId: 'request-2',
				kind: 'postpartum',
				state: 'pending',
				requestedBy: 'staff-2',
				requestedByName: 'Persephone Ochieng-Whitfield',
				requestedAt: '2026-08-01T00:00:00Z'
			}
		},
		{
			type: 'client_event',
			at: '2026-07-01T00:00:00Z',
			clientEvent: { eventType: 'created', diff: undefined, actorKind: 'system', createdAt: '2026-07-01T00:00:00Z' }
		},
		// An entry from a record absorbed into this one (#813). The
		// history renders it differently -- the What column carries the
		// "from another record" label -- so it earns its own row here,
		// per the fixture rule that a row set must hold every state a
		// field renders differently. Without it the sweep would measure a
		// History table whose widest cell is a state no merged Client's
		// screen actually shows, and the label is the longest one there
		// is.
		{
			type: 'client_event',
			at: '2026-06-02T00:00:00Z',
			fromMergedRecord: 'client-absorbed-1',
			clientEvent: {
				eventType: 'updated',
				diff: undefined,
				actorKind: 'staff',
				actorStaffId: 'staff-1',
				actorName: 'Anne-Marie Ochieng-Whitfield',
				createdAt: '2026-06-02T00:00:00Z'
			}
		},
		{
			type: 'client_event',
			at: '2026-06-01T00:00:00Z',
			fromMergedRecord: 'client-absorbed-1',
			clientEvent: { eventType: 'created', diff: undefined, actorKind: 'system', createdAt: '2026-06-01T00:00:00Z' }
		}
	],
	// The record that history came from, with the plaintext audit of the
	// act that absorbed it. One record rather than two: the summary's
	// length turns on the Staff name it carries, which is already this
	// fixture's longest, and a second row changes only a count.
	mergedFrom: [
		{
			clientId: 'client-absorbed-1',
			mergedAt: '2026-06-15T00:00:00Z',
			mergedByStaffId: 'staff-1',
			mergedByName: 'Anne-Marie Ochieng-Whitfield'
		}
	]
};

/*
 * What an Owner's own extra read answers (#691). Two invoices rather than
 * one, and one of them four figures, because the Notice this feeds is a
 * single generated sentence listing every one of them -- amount, status
 * and date apiece -- so its length is the list's length and #537's rule
 * lands on the row count here rather than on any one string.
 */
const eraseEligibility: EraseEligibility = {
	unsettledInvoices: [
		{
			invoiceId: 'invoice-1',
			status: 'open',
			amountCents: 450_000,
			currency: 'usd',
			createdAt: '2026-08-01T00:00:00Z'
		},
		{
			invoiceId: 'invoice-2',
			status: 'draft',
			amountCents: 92_500,
			currency: 'usd',
			createdAt: '2026-08-14T00:00:00Z'
		}
	]
};

/*
 * The Owner's screen (#928). `isOwner` arrives through `+page.ts`'s own
 * `load` rather than through `page.data`, so this restates `props`, not
 * `pageData` -- a variant that set the session here would mount the base
 * tree a second time and the sweep would stay green about it.
 *
 * What she reads that the base does not is #691's erasure block, and it
 * has an extra read behind it: `loadEligibility` runs only when `isOwner`
 * is true, so the base fixture never asks for `/erasure` and this variant
 * needs its own `respond`. A variant's `respond` replaces rather than
 * merges, so the session and detail answers below are restated with it.
 *
 * The unsettled-invoice branch, not the clear one: an Owner with nothing
 * outstanding gets one short destructive Button whose ConfirmDialog is
 * closed at rest and takes no box, while an Owner who is blocked reads a
 * sentence naming every unsettled invoice on the record. The blocked
 * branch is the one with something to measure.
 *
 * Not declared beside it: the ambient contractor's screen. `isContractor`
 * withholds "Start new work with <name>" and changes nothing else -- the
 * Edit link is unconditional (ADR-0017 gives a contractor Edit on her
 * attached Clients) and the erasure block is already absent from the base
 * -- so her tree is the base with one link removed, a strict subset that
 * realizes nothing the sweep has not already measured (ADR-0025).
 */
export const asOwner: RouteVariant = {
	name: 'The Client detail hub, as an Owner',
	props: { data: { isContractor: false, isOwner: true } },
	respond: (path) => {
		if (path.endsWith('/api/staff/session')) return jsonResponse({ staffId: 'staff-1' });
		if (path.endsWith('/erasure')) return jsonResponse(eraseEligibility);
		return jsonResponse(detail);
	}
};

export const fixture: RouteFixture = {
	name: 'The Client detail hub, as an employee Doula',
	component: Page,
	params: { practiceId: 'practice-1', clientId: 'client-1' },
	url: 'https://example.test/practices/practice-1/clients/client-1',
	props: { data: { isContractor: false } },
	respond: (path) => {
		if (path.endsWith('/api/staff/session')) return jsonResponse({ staffId: 'staff-1' });
		return jsonResponse(detail);
	},
	readyText: 'Persephone Ochieng-Whitfield',
	variants: [asOwner]
};
