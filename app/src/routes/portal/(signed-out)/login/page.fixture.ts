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
import type { RouteFixture, RouteVariant } from '../../../routeFixture.js';
import Page from './+page.svelte';

export const session: PortalSessionInfo = {
	engagements: [
		{
			engagementId: 'engagement-1',
			practiceName: 'https://portal.highland-midwifery-group.example.org/referrals/2027/persephone?source=intake',
			status: 'active',
			createdAt: '2026-01-15T20:00:00Z'
		},
		{
			engagementId: 'engagement-2',
			practiceName: 'Anne-Marie Ochieng-Whitfield Doula Care',
			status: 'active',
			createdAt: '2026-03-12T20:00:00Z'
		}
	]
};

/*
 * #757: the same screen after her session ended, which is a different
 * tree -- the notice above a form the base fixture never shows, since
 * the base answers the probe with a live session and lands her instead.
 * Deliberately identical in shape to the Staff login's own variant.
 */
export const afterSessionEnded: RouteVariant = {
	name: 'The Client-portal login screen, after a session ended',
	url: 'https://example.test/portal/(signed-out)/login?sessionEnded=true',
	respond: () => jsonResponse('no matching portal session', 404)
};

export const fixture: RouteFixture = {
	name: 'The Client-portal login screen',
	component: Page,
	params: {},
	url: 'https://example.test/portal/(signed-out)/login',
	respond: () => jsonResponse(session),
	readyText: 'Log in',
	variants: [afterSessionEnded]
};
