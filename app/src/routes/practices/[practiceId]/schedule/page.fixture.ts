/*
 * The Practice-wide schedule, as the continuum check sees it (#263).
 * Mirrors contracts/page.fixture.ts's shape and its hostile-content rule
 * (#537/#596): the widest thing a Practice types is a person's name, so
 * one row carries a double-barreled Client name beside a Doula with a
 * full professional suffix, and one carries a pasted URL where a name
 * should be. Both rows are scheduled -- an unscheduled Visit cannot
 * appear on this screen at all, which is what the endpoint's own
 * predicate decides, not this component.
 */
import type { CursorPage } from '#lib/paginatedList.svelte.js';
import type { ScheduledVisit } from '#lib/visitSchedule.js';
import type { SchedulePageData } from './+page.js';
import type { RouteFixture, RouteVariant } from '../../../routeFixture.js';
import { nth, TYPICAL_CLIENTS, TYPICAL_DOULAS } from '../../../typicalPilot.js';
import type { RouteParams as RouteParameters } from './$types';
import Page from './ScheduleScreen.svelte';

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

/*
 * Typical pilot data (#1981), the screen this route's table floor is
 * measured on (`typicalPilot.ts` says where each value comes from): a
 * month of a 14-doula agency's Visits, each Client with her own Doula, and
 * every Doula in the filter.
 */
export const typical: RouteVariant<RouteParameters> = {
	name: 'The Practice-wide schedule, with typical pilot data',
	typical: true,
	props: {
		data: {
			...data,
			page: {
				items: TYPICAL_CLIENTS.slice(0, 8).map((person, index) => {
					const doula = nth(TYPICAL_DOULAS, index + 2);
					return {
						visitId: `visit-${index + 1}`,
						engagementId: `eng-${index + 1}`,
						clientName: person.name,
						staffId: doula.staffId,
						staffName: doula.name,
						scheduledAt: `2026-09-${String(8 + index * 3).padStart(2, '0')}T14:30:00Z`
					};
				}),
				hasMore: false
			},
			doulas: TYPICAL_DOULAS.map((doula) => ({ value: doula.staffId, label: doula.name }))
		} satisfies SchedulePageData
	}
};

export const fixture: RouteFixture<RouteParameters> = {
	name: 'The Practice-wide schedule',
	variants: [typical],
	component: Page,
	params: { practiceId: 'practice-1' },
	url: 'https://example.test/practices/practice-1/schedule',
	props: { data },
	// The route takes its first page from `load` rather than from a fetch,
	// so the screen is on the page as soon as it renders; the heading is
	// still what proves it rendered the screen and not an error state.
	readyText: 'Schedule'
};
