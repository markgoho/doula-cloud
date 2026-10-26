/*
 * The Client-portal sign-in address question (#2045), as the continuum
 * check sees it. It reads nothing: the question asks for a new address,
 * and the one she has is on the summary it came from.
 */
import type { RouteFixture } from '../../../../../../routeFixture.js';
import Page from './+page.svelte';

export const fixture: RouteFixture = {
	name: 'The Client-portal sign-in address question',
	component: Page,
	params: { engagementId: 'engagement-1' },
	url: 'https://example.test/portal/engagements/engagement-1/sign-in-address/change',
	pageData: { practiceName: 'Riverside Doula Collective' },
	readyText: 'What is your new sign-in address?'
};
