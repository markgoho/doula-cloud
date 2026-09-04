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
import type { RouteFixture } from '../../../../routeFixture.js';
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
		}
	]
};

export const fixture: RouteFixture = {
	name: 'The Client detail hub',
	component: Page,
	params: { practiceId: 'practice-1', clientId: 'client-1' },
	url: 'https://example.test/practices/practice-1/clients/client-1',
	props: { data: { isContractor: false } },
	respond: (path) => {
		if (path.endsWith('/api/staff/session')) return jsonResponse({ staffId: 'staff-1' });
		return jsonResponse(detail);
	},
	readyText: 'Persephone Ochieng-Whitfield'
};
