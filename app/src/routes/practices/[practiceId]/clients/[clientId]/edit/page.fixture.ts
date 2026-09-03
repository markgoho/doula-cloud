/*
 * Editing a Client, as the continuum check sees it (#595).
 *
 * `title` starts as the static fallback "Edit Client" and becomes
 * `Edit ${displayName(detail)}` once the load resolves -- since the two
 * strings differ, the sweep's own `readyText` wait genuinely gates on
 * the fetch rather than racing it, the same mechanism the approval
 * screen's fixture relies on.
 */
import { jsonResponse } from '#lib/testResponse.js';
import type { ClientDetail } from '#lib/clientDetail.js';
import type { RouteFixture } from '../../../../../routeFixture.js';
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
	engagements: [],
	history: []
};

export const fixture: RouteFixture = {
	name: 'Editing a Client',
	component: Page,
	params: { practiceId: 'practice-1', clientId: 'client-1' },
	url: 'https://example.test/practices/practice-1/clients/client-1/edit',
	respond: () => jsonResponse(detail),
	readyText: 'Edit Persephone Ochieng-Whitfield'
};
