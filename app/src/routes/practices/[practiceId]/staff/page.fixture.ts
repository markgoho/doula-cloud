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
import type { MembershipHistory, WorkStateHistory } from '#lib/staff.js';
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

/*
 * What each row's two disclosures answer with (#1126).
 *
 * These were the spec's own until the sweep could reach them. A
 * disclosure that fetches on open was measured on the word `Loading...`,
 * so the fixture had nothing to answer and said so in a comment; now that
 * the check opens each one in preparation and waits for what arrives, the
 * roster's fixture owes these reads the same answer it owes the roster
 * itself, and the route's own spec imports them from here rather than
 * describing the screen a second time (#596).
 *
 * The row set follows ADR-0025's rule, applied to what the entry sentence
 * renders DIFFERENTLY rather than to how many rows there are. A move
 * ("Changed from ... to ...") and a first assertion ("Reported ...") are
 * two different sentences, not one sentence with a blank in it; an entry
 * older than the Membership carries the "(before joining this practice)"
 * line and one newer does not; and `hasMore` is what puts the "Show older
 * changes" button in the tree beside them. The Owner's row holds all four
 * at once, which is the busiest this disclosure ever gets. The Member with
 * nothing recorded and the page that failed are the two states left out on
 * purpose: both are swept on `HistoryDisclosure`'s own style-guide page,
 * where they are the whole subject rather than one row of a roster.
 *
 * The values are the longest the type allows, not representative ones
 * (#537): `District of Columbia` is the widest of the 51 names
 * `workStates.ts` carries and `North Carolina` the next, so the widest
 * sentence this screen can render is a move between those two, inside a
 * table cell at 320px.
 */
export const workStateHistories: Record<string, WorkStateHistory> = {
	'staff-1': {
		memberSince: '2025-12-01T00:00:00Z',
		items: [
			{
				eventId: 'event-3',
				previousWorkState: 'DC',
				workState: 'NY',
				createdAt: '2026-01-01T00:00:00Z'
			},
			// The widest sentence this screen can render: the two longest of
			// the 51 names `workStates.ts` carries, on either side of a move,
			// inside a table cell at 320px.
			{
				eventId: 'event-2',
				previousWorkState: 'NC',
				workState: 'DC',
				createdAt: '2025-12-15T00:00:00Z'
			},
			// Her first assertion, made before this Practice existed to be
			// joined -- so the row that carries no previous value is also the
			// row that carries the "(before joining this practice)" line.
			{ eventId: 'event-1', workState: 'NC', createdAt: '2025-06-01T00:00:00Z' }
		],
		hasMore: true,
		nextCursor: 'cursor-1'
	},
	'staff-2': {
		// Asserted at another Practice, a year before this Membership (#459):
		// a contractor doula carries her earlier Practice's rows in with her,
		// and the screen must not read as though she said it here.
		memberSince: '2026-08-01T00:00:00Z',
		items: [{ eventId: 'event-4', workState: 'CA', createdAt: '2025-05-04T12:00:00Z' }],
		hasMore: false
	}
};

/*
 * The other history behind the same row (#872). Every value here is the
 * STORED one -- `owner`, `contractor` -- which is what the BFF sends and
 * what the screen maps to the team's words through `roles.ts` (#262), so a
 * sentence reading "Contractor" on screen is the mapping's doing rather
 * than an echo of this file.
 *
 * The Owner's row again holds every sentence `membershipChangeSentence`
 * builds differently: a role change with all three roles on both sides,
 * which is the longest line this disclosure ever renders; an employment
 * type change; a joining; and an action that names no before or after at
 * all. The contractor's holds one, and it is the one the other row's
 * `roles_changed` is not -- a promotion rather than a full house.
 */
export const membershipHistories: Record<string, MembershipHistory> = {
	'staff-1': {
		items: [
			{
				eventId: 'membership-event-4',
				action: 'roles_changed',
				actorName: 'Anne-Marie Ochieng-Whitfield',
				previousRoles: ['owner', 'admin'],
				roles: ['owner', 'admin', 'doula'],
				createdAt: '2027-02-02T12:00:00Z'
			},
			{
				eventId: 'membership-event-3',
				action: 'sessions_ended',
				actorName: 'Anne-Marie Ochieng-Whitfield',
				createdAt: '2027-01-20T09:00:00Z'
			},
			{
				eventId: 'membership-event-2',
				action: 'employment_type_changed',
				actorName: 'Renata Alvarez',
				previousEmploymentType: 'employee',
				employmentType: 'contractor',
				createdAt: '2027-01-06T11:00:00Z'
			},
			{
				eventId: 'membership-event-1',
				action: 'joined',
				actorName: 'Renata Alvarez',
				roles: ['owner', 'admin', 'doula'],
				employmentType: 'employee',
				createdAt: '2026-08-01T00:00:00Z'
			}
		],
		hasMore: true,
		nextCursor: 'cursor-1'
	},
	'staff-2': {
		items: [
			{
				eventId: 'membership-event-5',
				action: 'roles_changed',
				actorName: 'Renata Alvarez',
				previousRoles: ['doula'],
				roles: ['admin', 'doula'],
				createdAt: '2026-11-11T11:00:00Z'
			}
		],
		hasMore: false
	}
};

/*
 * Whose history a path is asking for. Both rows fetch both histories, so
 * the answer turns on the segment and the staff id together -- and the
 * cursor query, when a second page is asked for, rides on the last segment
 * rather than this one.
 */
export function subjectOf(path: string): string {
	return path.split('/').at(-2) ?? '';
}

export const fixture: RouteFixture = {
	name: 'The Staff roster',
	component: Page,
	params: { practiceId: 'practice-1' },
	url: 'https://example.test/practices/practice-1/staff',
	/*
	 * An Owner, because that is the widest this screen ever gets: #694's
	 * "Send a recovery code" is the one row action gated on the role, and
	 * an Admin's tree is that tree with one link removed. A variant for
	 * her is deliberately not declared -- a strict subset realizes nothing
	 * the sweep has not already measured (svelte-tests.md).
	 *
	 * Re-read against the whole `{#if}` chain on #928 and left as it
	 * stands. `isOwner` is the only session predicate this route reads,
	 * and the roster endpoint is Owner-and-Admin, so an Owner and an Admin
	 * are the only two sessions that reach the screen at all -- there is
	 * no third tree here for a variant to name.
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
	/*
	 * The roster, and the two histories a row's disclosures ask for once
	 * they are opened (#1126). Before that ticket the check never opened
	 * one in a state where its handler could run, so a fixture answering
	 * the roster for every path went unnoticed: a history read got the
	 * roster back, failed to parse as a page of entries, and the screen the
	 * sweep measured was an error notice.
	 */
	respond: (path) => {
		if (path.includes('/work-state-history')) {
			return jsonResponse(workStateHistories[subjectOf(path)]);
		}
		if (path.includes('/membership-history')) {
			return jsonResponse(membershipHistories[subjectOf(path)]);
		}
		return jsonResponse(roster);
	},
	readyText: 'Staff'
};
