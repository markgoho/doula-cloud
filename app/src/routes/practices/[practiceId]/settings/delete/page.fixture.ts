/*
 * The Delete-this-Practice settings screen, as the continuum check sees
 * it (#595, #871). An Owner viewing the screen before she has started
 * anything: nothing pending, no unsettled Invoice in the way.
 */
import { jsonResponse } from '#lib/testResponse.js';
import type { RouteFixture } from '../../../../routeFixture.js';
import Page from './+page.svelte';

export const status = { pending: false, hasUnsettledInvoices: false };

export const fixture: RouteFixture = {
	name: 'The Delete-this-Practice settings screen',
	component: Page,
	params: { practiceId: 'practice-1' },
	url: 'https://example.test/practices/practice-1/settings/delete',
	pageData: {
		session: {
			practiceId: 'practice-1',
			practiceName: 'Riverside Doula Collective',
			roles: ['owner'],
			isContractor: false
		}
	},
	respond: () => jsonResponse(status),
	readyText: 'Delete this Practice'
};
