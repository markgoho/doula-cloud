/*
 * The on-call roster as the continuum check sees it (#1093).
 *
 * The ticket names the load this screen owns: fourteen Doulas and a full
 * month of births, at 320px. So this fixture carries exactly that, and
 * the content is hostile rather than polite (ADR-0025, #537): a
 * double-barreled Client name beside a Doula with a professional suffix,
 * a pasted URL where a name should be, a window with two narrowed
 * Doulas, a covered gap, an uncovered one, and a run of days nobody is
 * on call for.
 */
import type { OnCallPageData } from './+page.js';
import type {
	CoverageGap,
	Roster,
	RosterDoula,
	RosterWindow,
} from '#lib/onCall.js';
import type { RouteFixture } from '../../../routeFixture.js';
import type { RouteParams as RouteParameters } from './$types';
import Page from './+page.svelte';

const doulaNames = [
	'Persephone Vandermeulen-Achterberg, CD(DONA)',
	'Bo Ng',
	'Marguerite Okonkwo-Fitzgerald',
	'Ana Ruiz',
	'Tallulah Brightwater-Finch',
	'Jo Park',
	'Henrietta Oyelaran-Whitcombe',
	'Sam Lee',
	'Rosalind Achterberg',
	'Kim Tran',
	'Genevieve Castellanos-Ward',
	'Nia Cole',
	'Wilhelmina Featherstonehaugh',
	'Ida Bell',
];

const doulas: RosterDoula[] = doulaNames.map((name, index) => ({
	staffId: `staff-${index + 1}`,
	name,
	available: index % 5 !== 0,
	concurrentWindows: index === 0 ? 3 : index % 3,
}));

function gap(
	overrides: Partial<CoverageGap> & Pick<CoverageGap, 'id'>
): CoverageGap {
	return {
		engagementId: 'eng-1',
		staffId: 'staff-1',
		staffName: doulaNames[0]!,
		startsAt: '2026-10-17T22:00:00Z',
		endsAt: '2026-10-18T10:00:00Z',
		reason: undefined,
		coveringStaffId: undefined,
		coveringStaffName: undefined,
		...overrides,
	};
}

/* A month of births: one per day, each with a window around it, so the
	 table is asked to lay out a real month rather than three rows. */
const clientNames = [
	'Anne-Marie Ochieng-Whitfield',
	'https://portal.highland-midwifery-group.example.org/referrals/2027/persephone?source=intake',
	'Bea Okafor',
	'Cleo Marsh',
	'Dot Nakamura',
];

const windows: RosterWindow[] = Array.from({ length: 28 }, (_, index) => {
	const day = String((index % 28) + 1).padStart(2, '0');
	const primary = doulas[index % doulas.length]!;
	const backup = doulas[(index + 1) % doulas.length]!;
	const isUncovered = index % 4 === 0;
	return {
		engagementId: `eng-${index + 1}`,
		clientName: clientNames[index % clientNames.length]!,
		dueDate: `2026-11-${day}`,
		window: { start: `2026-10-${day}`, end: `2026-11-${day}` },
		onCall:
			index % 3 === 0
				? [
						{
							staffId: primary.staffId,
							name: primary.name,
							from: `2026-10-${day}`,
							to: '2026-10-22',
							narrowed: true,
						},
						{
							staffId: backup.staffId,
							name: backup.name,
							from: '2026-10-23',
							to: `2026-11-${day}`,
							narrowed: true,
						},
					]
				: [
						{
							staffId: primary.staffId,
							name: primary.name,
							from: `2026-10-${day}`,
							to: `2026-11-${day}`,
							narrowed: false,
						},
					],
		gaps:
			index === 0
				? [
						gap({ id: 'gap-1' }),
						gap({
							id: 'gap-2',
							staffId: 'staff-2',
							staffName: doulaNames[1]!,
							coveringStaffId: 'staff-3',
							coveringStaffName: doulaNames[2]!,
							reason: 'At a wedding four hours away',
						}),
					]
				: [],
		unstaffedDays: isUncovered
			? [{ start: '2026-10-23', end: '2026-10-24' }]
			: [],
		uncovered: isUncovered || index === 0,
	};
});

export const roster: Roster = {
	from: '2026-10-01',
	to: '2026-10-31',
	windows,
	noWindow: [
		{
			engagementId: 'eng-no-due',
			clientName: 'Dot Nakamura',
			reason: 'no_due_date',
		},
		{
			engagementId: 'eng-unknown-reason',
			clientName: 'Wilhelmina Featherstonehaugh-Pemberton',
			reason: 'something_the_bff_learned_later',
		},
	],
	doulas,
};

export const data: OnCallPageData = {
	roster,
	range: { from: '2026-10-01', to: '2026-10-31' },
};

export const fixture: RouteFixture<RouteParameters> = {
	name: 'The on-call roster',
	component: Page,
	params: { practiceId: 'practice-1' },
	url: 'https://example.test/practices/practice-1/on-call?from=2026-10-01&to=2026-10-31',
	props: { data },
	// The route takes its roster from `load` rather than from a fetch, so
	// the screen is on the page as soon as it renders; the heading is
	// what proves it rendered the screen and not an error state.
	readyText: 'On call',
};
