/*
 * Your account, as the continuum check sees it (#595).
 *
 * The fieldset legend echoes the signed-in Staff member's own name, so
 * this carries #537's vocabulary the same way the two existing fixtures
 * do -- a hyphenated double-barrelled name, since that is what a person
 * actually types into a name field, unlike the URL vocabulary that fits
 * a Practice's own free-text fields.
 */
import { jsonResponse } from '#lib/testResponse.js';
import type { SessionInfo } from '#lib/landing.js';
import type { RouteFixture } from '../routeFixture.js';
import Page from './+page.svelte';

export const session: SessionInfo = {
	memberships: [{ practiceId: 'practice-1', practiceName: 'Riverside Doula Collective', roles: ['owner'] }],
	lastPracticeId: 'practice-1',
	staffId: 'staff-1',
	name: 'Anne-Marie Ochieng-Whitfield',
	email: 'anne-marie@example.test',
	workState: 'NY',
	workStateReportedAt: '2026-01-01T00:00:00Z',
	secondFactor: false
};

export const fixture: RouteFixture = {
	name: 'Your account',
	component: Page,
	params: {},
	url: 'https://example.test/account',
	respond: () => jsonResponse(session),
	readyText: 'Your account'
};
