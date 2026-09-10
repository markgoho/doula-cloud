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
 * double-barreled one, and the Owner's own address is a long one, since
 * every sentence here embeds one or the other and the warning embeds
 * both.
 */
import { jsonResponse } from '#lib/testResponse.js';
import type { SessionInfo } from '#lib/landing.js';
import type { RouteFixture, RouteVariant } from '../../../../../routeFixture.js';
import Page from './+page.svelte';

/**
The Membership the Owner's own screen is drawn against.
*/
export const ownerSession = {
	practiceId: 'practice-1',
	staffId: 'staff-1',
	practiceName: 'Riverside Doula Collective',
	roles: ['owner'],
	isContractor: false
};

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

/*
 * The other screen this route renders (#913). An Admin is not the Owner's
 * tree with a button removed -- the whole page is replaced by a notice
 * saying who can do this, and nothing is fetched at all -- so it is a
 * branch the sweep has to measure rather than a strict subset it can
 * infer. `readyText` is the same heading, and it is the fallback wording,
 * because no roster read ever happens to name anybody.
 */
export const asAdmin: RouteVariant = {
	name: 'Owner vouching, as an Admin who cannot',
	pageData: { session: { ...ownerSession, roles: ['admin'] } },
	readyText: 'Help someone sign in again'
};

export const fixture: RouteFixture = {
	name: 'Owner vouching for a locked-out Staff member',
	component: Page,
	params: { practiceId: 'practice-1', staffId: 'staff-2' },
	url: 'https://example.test/practices/practice-1/staff/staff-2/mfa-recovery',
	pageData: { session: ownerSession },
	variants: [asAdmin],
	respond: (path: string) =>
		jsonResponse(path.startsWith('/api/staff/session') ? session : roster),
	readyText: 'Help Persephone Ochieng-Whitfield sign in again'
};
