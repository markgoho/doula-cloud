/*
 * The Practice-wide schedule, as the continuum check sees it (#263).
 * Mirrors contracts/page.fixture.ts's shape and its hostile-content rule
 * (#537/#596): the widest thing a Practice types is a person's name, so
 * one row carries a double-barrelled Client name beside a Doula with a
 * full professional suffix, and one carries a pasted URL where a name
 * should be. Both rows are scheduled -- an unscheduled Visit cannot
 * appear on this screen at all, which is what the endpoint's own
 * predicate decides, not this component.
 */
import type { CursorPage } from '#lib/paginatedList.svelte.js';
import type { ScheduledVisit } from '#lib/visitSchedule.js';
import type { SchedulePageData } from './+page.js';
import type { RouteFixture } from '../../../routeFixture.js';
import type { RouteParams as RouteParameters } from './$types';
import Page from './+page.svelte';

export const schedulePage: CursorPage<ScheduledVisit> = {
	items: [
		{
			visitId: 'visit-1',
			engagementId: 'eng-1',
			clientName: 'Anne-Marie Ochieng-Whitfield',
			staffId: 'staff-1',
			staffName: 'Persephone Vandermeulen-Achterberg, CD(DONA)',
			scheduledAt: '2026-09-12T14:30:00Z'
		},
		{
			visitId: 'visit-2',
			engagementId: 'eng-2',
			clientName:
				'https://portal.highland-midwifery-group.example.org/referrals/2027/persephone?source=intake',
			staffId: 'staff-2',
			staffName: 'Bo Ng',
			scheduledAt: '2026-09-19T02:05:00Z'
		}
	],
	hasMore: false
};

export const data: SchedulePageData = {
	page: schedulePage,
	filters: { from: '2026-09-07', to: '2026-10-07' },
	doulas: [
		{ value: 'staff-1', label: 'Persephone Vandermeulen-Achterberg, CD(DONA)' },
		{ value: 'staff-2', label: 'Bo Ng' }
	]
};

export const fixture: RouteFixture<RouteParameters> = {
	name: 'The Practice-wide schedule',
	component: Page,
	params: { practiceId: 'practice-1' },
	url: 'https://example.test/practices/practice-1/schedule',
	props: { data },
	// The route takes its first page from `load` rather than from a fetch,
	// so the screen is on the page as soon as it renders; the heading is
	// still what proves it rendered the screen and not an error state.
	readyText: 'Schedule'
};
