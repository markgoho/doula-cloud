/*
 * One Staff member's page, as the continuum check sees it (#2041).
 *
 * The page reads the roster and finds its person in it, then reads both
 * histories, so `respond` forks on the path. The roster is the roster
 * route's own (#596): the same two Members, so this page and that one
 * describe one Practice.
 *
 * Two screens, by ADR-0025's rule of states rather than counts. The base
 * is the Owner's own row, which holds every field's busiest state at once
 * -- three roles, the hyphenated double-barreled name (#537) and an
 * address that is a URL (#530), a work state history with a move between
 * the two longest state names, a first assertion and more to come, and a
 * Membership history with every sentence `membershipChangeSentence`
 * builds differently. The contractor holds the other states together: no
 * roles yet, a work state reported before joining this Practice (#459),
 * and one history entry with nothing older.
 *
 * As an Admin the page is the Owner's tree with the two Changes, the
 * recovery link and the Access section taken away -- a strict subset, so
 * it is left out on purpose (svelte-tests.md).
 */
import { jsonResponse } from '#lib/testResponse.js';
import type { MembershipHistory, WorkStateHistory } from '#lib/staff.js';
import type { RouteFixture, RouteVariant } from '../../../../routeFixture.js';
import { fixture as rosterFixture, roster } from '../page.fixture.js';
import Page from './+page.svelte';

/*
 * What each Member's work state history answers with (#459). The values
 * are the longest the type allows (#537): `District of Columbia` is the
 * widest of the 51 names `workStates.ts` carries and `North Carolina` the
 * next, so the widest sentence this page can render is a move between
 * those two.
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
			{
				eventId: 'event-2',
				previousWorkState: 'NC',
				workState: 'DC',
				createdAt: '2025-12-15T00:00:00Z'
			},
			// The first assertion, made before this Practice existed to be
			// joined -- so the row with no previous value also carries the
			// "(before joining this practice)" line.
			{ eventId: 'event-1', workState: 'NC', createdAt: '2025-06-01T00:00:00Z' }
		],
		hasMore: true,
		nextCursor: 'cursor-1'
	},
	'staff-2': {
		// Asserted at another Practice, a year before this Membership (#459).
		memberSince: '2026-08-01T00:00:00Z',
		items: [{ eventId: 'event-4', workState: 'CA', createdAt: '2025-05-04T12:00:00Z' }],
		hasMore: false
	}
};

/*
 * The Membership history (#872). Every value here is the STORED one --
 * `owner`, `contractor` -- which is what the BFF sends and what the page
 * maps to the team's words through `roles.ts` (#262).
 *
 * The Owner's row holds a role change with all three roles on both
 * sides, the longest line this table renders; an employment type change;
 * a joining; and an action that names no before or after at all.
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
 * Whose history a path asks for: the staff id is the segment before the
 * history's own, and a cursor query rides on the last segment.
 */
export function subjectOf(path: string): string {
	return path.split('/').at(-2) ?? '';
}

/**
 * The roster and both histories, by path.
 */
export function respond(path: string): Response {
	if (path.includes('/work-state-history')) {
		return jsonResponse(workStateHistories[subjectOf(path)]);
	}
	if (path.includes('/membership-history')) {
		return jsonResponse(membershipHistories[subjectOf(path)]);
	}
	return jsonResponse(roster);
}

const [owner, contractor] = roster.members;

export const asContractor: RouteVariant = {
	name: "A Staff member's page, for a contractor with no roles yet",
	params: { practiceId: 'practice-1', staffId: contractor!.staffId },
	url: `https://example.test/practices/practice-1/staff/${contractor!.staffId}`,
	readyText: contractor!.name
};

export const fixture: RouteFixture = {
	name: "A Staff member's page",
	component: Page,
	params: { practiceId: 'practice-1', staffId: owner!.staffId },
	url: `https://example.test/practices/practice-1/staff/${owner!.staffId}`,
	pageData: rosterFixture.pageData,
	variants: [asContractor],
	respond,
	readyText: owner!.name
};
