/*
 * The Client-portal login screen, as the continuum check sees it (#595).
 *
 * Deliberately identical in shape to the Staff login's own fixture: the
 * on-mount probe (#283) finds a live session with more than one
 * Engagement and shows the picker, whose only free text is the Practice's
 * own name -- #530's URL, carried rather than invented.
 */
import { jsonResponse } from '#lib/testResponse.js';
import type { PortalSessionInfo } from '#lib/portalLanding.js';
import type { RouteFixture } from '../../../routeFixture.js';
import Page from './+page.svelte';

export const session: PortalSessionInfo = {
	engagements: [
		{
			engagementId: 'engagement-1',
			practiceName: 'https://portal.highland-midwifery-group.example.org/referrals/2027/persephone?source=intake',
			status: 'active'
		},
		{ engagementId: 'engagement-2', practiceName: 'Anne-Marie Ochieng-Whitfield Doula Care', status: 'active' }
	]
};

export const fixture: RouteFixture = {
	name: 'The Client-portal login screen',
	component: Page,
	params: {},
	url: 'https://example.test/portal/(signed-out)/login',
	respond: () => jsonResponse(session),
	readyText: 'Log in'
};
