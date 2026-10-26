/*
 * The Staff roster, as the continuum check sees it (#595).
 *
 * `name` and `email` are the table's two free-text columns -- #537's
 * hyphenated double-barreled name and #530's own URL, the shape
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
import type { RouteFixture, RouteVariant } from '../../../routeFixture.js';
import { TYPICAL_STAFF } from '../../../typicalPilot.js';
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

/*
 * Typical pilot data (#1981), the screen a table's floor is measured on
 * (`typicalPilot.ts` says where each value comes from): the fifteen Staff
 * of a 14-doula agency and one live invitation to the next Doula. The
 * long values stay on the hostile rows above.
 */
const typicalRoster = {
	members: TYPICAL_STAFF.map((member) => ({
		...member,
		roles: [...member.roles],
		workStateReportedAt: '2026-09-08T15:00:00Z'
	})),
	invitations: {
		items: [
			{
				invitationId: 'invitation-1',
				address: 'sofia.ortiz@gmail.com',
				roles: ['doula'],
				employmentType: 'employee',
				expiresAt: '2026-10-29T15:00:00Z',
				expired: false,
				deliveryFailed: false
			}
		],
		hasMore: false
	}
};

export const typical: RouteVariant = {
	name: 'The Staff roster, with typical pilot data',
	typical: true,
	respond: () => jsonResponse(typicalRoster)
};

export const fixture: RouteFixture = {
	name: 'The Staff roster',
	variants: [typical],
	component: Page,
	params: { practiceId: 'practice-1' },
	url: 'https://example.test/practices/practice-1/staff',
	/*
	 * An Owner. Since #2041 the roster reads no session predicate at all
	 * -- the one row action gated on the role, "Send a recovery code",
	 * moved to the Staff member's own page -- so an Owner and an Admin,
	 * the only two sessions the roster endpoint answers, draw one tree,
	 * and there is no variant for an Admin to name.
	 */
	pageData: {
		session: {
			practiceId: 'practice-1',
			staffId: 'staff-1',
			practiceName: 'Riverside Doula Collective',
			roles: ['owner'],
			isContractor: false
		}
	},
	// The roster is the only read: the histories are the Staff member's
	// page's since #2041 (`[staffId]/page.fixture.ts`).
	respond: () => jsonResponse(roster),
	readyText: 'Staff'
};
