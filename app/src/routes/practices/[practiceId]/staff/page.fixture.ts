/*
 * The Staff roster, as the continuum check sees it (#595).
 *
 * `name` and `email` are the table's two free-text columns -- #537's
 * hyphenated double-barrelled name and #530's own URL, the shape
 * `DataTable`'s #542 fix was written against, now measured on the one
 * screen that lists every Staff member at once.
 *
 * Two Members and two pending Invitations, not one Member and none
 * (#596): a roster holding a single Owner and nothing pending is a
 * screen no Practice ever looks at, and #537 is the argument that a
 * fixture measuring such a screen measures nothing. The second Member
 * is a contractor holding no roles yet, and the two Invitations are one
 * live and one lapsed-and-undeliverable, because those are the rows a
 * real roster carries and the ones whose per-row flags and actions have
 * to fit beside everything else.
 */
import { jsonResponse } from '#lib/testResponse.js';
import type { RouteFixture } from '../../../routeFixture.js';
import Page from './+page.svelte';

export const roster = {
	members: [
		{
			staffId: 'staff-1',
			name: 'Anne-Marie Ochieng-Whitfield',
			email: 'https://portal.highland-midwifery-group.example.org/referrals/2027/persephone?source=intake',
			roles: ['owner', 'admin'],
			employmentType: 'employee',
			workState: 'NY',
			workStateReportedAt: '2026-01-01T00:00:00Z'
		},
		{
			// A contractor doula who has joined but holds no roles yet, and
			// whose work state was asserted at another Practice a year
			// before this Membership (#459).
			staffId: 'staff-2',
			name: 'Persephone Ochieng-Whitfield',
			email: 'persephone@example.test',
			roles: [],
			employmentType: 'contractor',
			workState: 'CA',
			workStateReportedAt: '2025-05-04T12:00:00Z'
		}
	],
	invitations: {
		items: [
			{
				invitationId: 'invitation-1',
				address: 'anne-marie.ochieng-whitfield@example.test',
				roles: ['doula'],
				employmentType: 'contractor',
				expiresAt: '2026-09-01T00:00:00Z',
				expired: false,
				deliveryFailed: false
			},
			{
				// Lapsed and dead-lettered at once (#291, #339): the row
				// carries two flags beside its actions, which is the widest
				// an Invitation row ever gets.
				invitationId: 'invitation-2',
				address: 'anne-marie@example.test',
				roles: ['admin'],
				employmentType: 'employee',
				expiresAt: '2026-09-02T00:00:00Z',
				expired: true,
				deliveryFailed: true
			}
		],
		hasMore: false
	}
};

export const fixture: RouteFixture = {
	name: 'The Staff roster',
	component: Page,
	params: { practiceId: 'practice-1' },
	url: 'https://example.test/practices/practice-1/staff',
	respond: () => jsonResponse(roster),
	readyText: 'Staff'
};
