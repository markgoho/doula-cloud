/*
 * The Staff login screen, as the continuum check sees it (#595).
 *
 * On mount it probes for a live session (#283) and, finding one with
 * more than one Membership, shows the same Practice picker `/` renders
 * -- so the fixture answers that probe rather than leaving the form
 * alone, which is the only way this route ever puts a Practice's own
 * free text (`practiceName`) on screen. #530's own URL, not this
 * ticket's invention.
 */
import { jsonResponse } from '#lib/testResponse.js';
import type { SessionInfo } from '#lib/landing.js';
import type { RouteFixture } from '../../routeFixture.js';
import Page from './+page.svelte';

const session: SessionInfo = {
	memberships: [
		{
			practiceId: 'practice-1',
			practiceName: 'https://portal.highland-midwifery-group.example.org/referrals/2027/persephone?source=intake',
			roles: ['owner']
		},
		{ practiceId: 'practice-2', practiceName: 'Anne-Marie Ochieng-Whitfield Doula Care', roles: ['doula'] }
	],
	lastPracticeId: undefined,
	staffId: 'staff-1',
	name: 'Anne-Marie Ochieng-Whitfield',
	email: 'anne-marie@example.test',
	workState: 'NY',
	workStateReportedAt: '2026-01-01T00:00:00Z'
};

export const fixture: RouteFixture = {
	name: 'The Staff login screen',
	component: Page,
	params: {},
	url: 'https://example.test/(signed-out)/login',
	respond: () => jsonResponse(session),
	readyText: 'Log in'
};
