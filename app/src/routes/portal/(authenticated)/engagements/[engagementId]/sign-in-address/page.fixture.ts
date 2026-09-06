/*
 * The Client-portal sign-in-address change screen (#619), as the
 * continuum check sees it (#595).
 *
 * The address is hostile on purpose (#537): a long local part at a long
 * domain is what a real person's work address looks like, and it is the
 * value that decides whether the intro line wraps or overflows at 320px.
 */
import { jsonResponse } from '#lib/testResponse.js';
import type { RouteFixture } from '../../../../../routeFixture.js';
import Page from './+page.svelte';

export const session = {
	clientId: 'client-1',
	signInAddress: 'margaretha.vandenberghe@northshore-midwifery-collective.example',
	engagements: []
};

export const fixture: RouteFixture = {
	name: 'The Client-portal sign-in address screen',
	component: Page,
	params: { engagementId: 'engagement-1' },
	url: 'https://example.test/portal/engagements/engagement-1/sign-in-address',
	pageData: { practiceName: 'Riverside Doula Collective' },
	respond: () => jsonResponse(session),
	readyText: 'Sign-in address'
};
