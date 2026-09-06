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
import type { RouteFixture } from '../../../../../../routeFixture.js';
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

export const fixture: RouteFixture = {
	name: 'Requesting a new Engagement',
	component: Page,
	params: { practiceId: 'practice-1', clientId: 'client-1' },
	url: 'https://example.test/practices/practice-1/clients/client-1/engagement-requests/new',
	pageData: {
		session: {
			practiceId: 'practice-1',
			practiceName: 'Riverside Doula Collective',
			roles: ['doula'],
			isContractor: false
		}
	},
	respond: () => jsonResponse(detail),
	readyText: 'Ask to start work with Persephone Ochieng-Whitfield'
};
