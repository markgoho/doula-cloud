/*
 * Owner vouching, as the continuum check sees it (#595, #694).
 *
 * This screen makes two calls on mount and reads a different thing out of
 * each -- the roster, for the Staff member this page is about, and the
 * signed-in person's own session, for the address the code will actually
 * arrive at. So `respond` forks on the path rather than answering one
 * body to both.
 *
 * The content is hostile on the two axes that decide this screen's width
 * (#537): the Staff member's name is the repo's hyphenated
 * double-barrelled one, and the Owner's own address is a long one, since
 * every sentence here embeds one or the other and the warning embeds
 * both.
 */
import { jsonResponse } from '#lib/testResponse.js';
import type { SessionInfo } from '#lib/landing.js';
import type { RouteFixture } from '../../../../../routeFixture.js';
import Page from './+page.svelte';

export const roster = {
	members: [
		{
			staffId: 'staff-2',
			name: 'Persephone Ochieng-Whitfield',
			email: 'persephone.ochieng-whitfield@highland-midwifery-group.example.org',
			roles: ['doula'],
			employmentType: 'contractor',
			workState: 'CA',
			workStateReportedAt: '2025-05-04T12:00:00Z'
		}
	],
	invitations: { items: [], hasMore: false }
};

export const session: SessionInfo = {
	memberships: [
		{ practiceId: 'practice-1', practiceName: 'Riverside Doula Collective', roles: ['owner'] }
	],
	lastPracticeId: 'practice-1',
	staffId: 'staff-1',
	name: 'Anne-Marie Ochieng-Whitfield',
	email: 'anne-marie.ochieng-whitfield@riverside-doula-collective.example.org',
	workState: 'NY',
	workStateReportedAt: '2026-01-01T00:00:00Z',
	secondFactor: true,
	soleOwner: false
};

export const fixture: RouteFixture = {
	name: 'Owner vouching for a locked-out Staff member',
	component: Page,
	params: { practiceId: 'practice-1', staffId: 'staff-2' },
	url: 'https://example.test/practices/practice-1/staff/staff-2/mfa-recovery',
	pageData: {
		session: {
			practiceId: 'practice-1',
			staffId: 'staff-1',
			practiceName: 'Riverside Doula Collective',
			roles: ['owner'],
			isContractor: false
		}
	},
	respond: (path: string) =>
		jsonResponse(path.startsWith('/api/staff/session') ? session : roster),
	readyText: 'Help Persephone Ochieng-Whitfield sign in again'
};
