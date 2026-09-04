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
 */
import { jsonResponse } from '#lib/testResponse.js';
import type { ClientDetail } from '#lib/clientDetail.js';
import type { RouteFixture } from '../../../../routeFixture.js';
import Page from './+page.svelte';

const detail: ClientDetail = {
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
	engagements: [{ engagementId: 'engagement-1', kind: 'birth', status: 'active', createdAt: '2026-08-01T00:00:00Z' }],
	history: [
		{
			type: 'engagement_request',
			at: '2026-08-01T00:00:00Z',
			engagementRequest: {
				requestId: 'request-1',
				kind: 'birth',
				state: 'pending',
				requestedBy: 'staff-2',
				requestedByName: 'Anne-Marie Ochieng-Whitfield',
				requestedAt: '2026-08-01T00:00:00Z'
			}
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
