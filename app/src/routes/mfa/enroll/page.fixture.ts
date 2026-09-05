/*
 * TOTP enrolment, as the continuum check sees it (#595). On mount this
 * screen probes `/api/staff/session` for the email to re-authenticate
 * with (its own header comment explains why) -- the fixture answers that
 * probe rather than leaving the screen on a step it can never reach.
 */
import { jsonResponse } from '#lib/testResponse.js';
import type { SessionInfo } from '#lib/landing.js';
import type { RouteFixture } from '../../routeFixture.js';
import Page from './+page.svelte';

const session: SessionInfo = {
	memberships: [{ practiceId: 'practice-1', practiceName: 'Riverside Doulas', roles: ['owner'] }],
	lastPracticeId: undefined,
	staffId: 'staff-1',
	name: 'Anne-Marie Ochieng-Whitfield',
	email: 'anne-marie@example.test',
	workState: 'NY',
	workStateReportedAt: '2026-01-01T00:00:00Z',
	secondFactor: false
};

export const fixture: RouteFixture = {
	name: 'TOTP enrolment',
	component: Page,
	params: {},
	url: 'https://example.test/mfa/enroll',
	respond: () => jsonResponse(session),
	readyText: 'Set up two-factor authentication'
};
