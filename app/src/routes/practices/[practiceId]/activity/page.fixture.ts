/*
 * The Practice's whole activity ledger (#2006), as the continuum check
 * sees it.
 *
 * Three rows, one for each way the What column reads (#720): an
 * Engagement's row naming its Client, a Membership row naming the staff
 * member, and a Practice row naming a setting and no one. The names and
 * the actor are long on purpose (#537): the actor is #530's pasted URL,
 * and each name is the longest a Practice is likely to type. `hasMore`
 * is set, so the sweep measures the control that loads the next page.
 *
 * One session, the Owner's: a Doula's screen is the same tree with fewer
 * rows, which the BFF's per-row gate decides, so it realizes nothing the
 * sweep has not measured.
 */
import { jsonResponse } from '#lib/testResponse.js';
import type { ActivityEntry } from '#lib/activityLedger.js';
import { practiceSession, type RouteFixture } from '../../../routeFixture.js';
import Page from './+page.svelte';

export const items: ActivityEntry[] = [
	{
		subjectKind: 'engagement',
		subjectId: 'engagement-1',
		subjectName: 'Persephone Adeyemi-Wollstonecraft',
		action: 'contract_amount_repriced',
		actorKind: 'staff',
		actorName: 'https://portal.highland-midwifery-group.example.org/referrals/2027/persephone?source=intake',
		createdAt: '2026-08-01T00:00:00Z'
	},
	{
		subjectKind: 'membership',
		subjectId: 'staff-2',
		subjectName: 'Anne-Marie Ochieng-Whitfield',
		action: 'employment_type_changed',
		actorKind: 'staff',
		actorName: 'Marguerite Vandenberg-Whitfield',
		createdAt: '2026-07-30T00:00:00Z'
	},
	{
		subjectKind: 'practice',
		subjectId: 'practice-1',
		action: 'mfa_required_disabled',
		actorKind: 'staff',
		actorName: 'Marguerite Vandenberg-Whitfield',
		createdAt: '2026-07-29T00:00:00Z'
	}
];

export const fixture: RouteFixture = {
	name: "The Practice's whole activity ledger, as an Owner",
	component: Page,
	params: { practiceId: 'practice-1' },
	url: 'https://example.test/practices/practice-1/activity',
	pageData: practiceSession(['owner']),
	respond: () => jsonResponse({ items, hasMore: true, nextCursor: 'cursor-1' }),
	readyText: 'Activity'
};
