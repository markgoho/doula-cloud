/*
 * The Staff roster, as the continuum check sees it (#595).
 *
 * `name` and `email` are the table's two free-text columns -- #537's
 * hyphenated double-barrelled name and #530's own URL, the shape
 * `DataTable`'s #542 fix was written against, now measured on the one
 * screen that lists every Staff member at once.
 */
import { jsonResponse } from '#lib/testResponse.js';
import type { RouteFixture } from '../../../routeFixture.js';
import Page from './+page.svelte';

const roster = {
	members: [
		{
			staffId: 'staff-1',
			name: 'Anne-Marie Ochieng-Whitfield',
			email: 'https://portal.highland-midwifery-group.example.org/referrals/2027/persephone?source=intake',
			roles: ['owner', 'admin'],
			employmentType: 'employee',
			workState: 'NY',
			workStateReportedAt: '2026-01-01T00:00:00Z'
		}
	],
	invitations: { items: [], hasMore: false }
};

export const fixture: RouteFixture = {
	name: 'The Staff roster',
	component: Page,
	params: { practiceId: 'practice-1' },
	url: 'https://example.test/practices/practice-1/staff',
	respond: () => jsonResponse(roster),
	readyText: 'Staff'
};
