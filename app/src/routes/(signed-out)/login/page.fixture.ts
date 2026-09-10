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
import type { RouteFixture, RouteVariant } from '../../routeFixture.js';
import Page from './+page.svelte';

export const session: SessionInfo = {
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
	workStateReportedAt: '2026-01-01T00:00:00Z',
	secondFactor: false
};

/*
 * #757: the same screen, entered by someone whose session just ended.
 * A second tree, not a second route -- the notice renders above a form
 * the base fixture never shows, since the base answers the probe with a
 * live session and draws the picker instead. Both halves of the
 * continuum read this (ADR-0025): the sweep measures the notice and the
 * form together, and the drag surface can be shown the screen a person
 * actually lands on after a 401.
 */
export const afterSessionEnded: RouteVariant = {
	name: 'The Staff login screen, after a session ended',
	url: 'https://example.test/(signed-out)/login?sessionEnded=true',
	respond: () => jsonResponse('no matching staff session', 404)
};

export const fixture: RouteFixture = {
	name: 'The Staff login screen',
	component: Page,
	params: {},
	url: 'https://example.test/(signed-out)/login',
	respond: () => jsonResponse(session),
	readyText: 'Log in',
	variants: [afterSessionEnded]
};
