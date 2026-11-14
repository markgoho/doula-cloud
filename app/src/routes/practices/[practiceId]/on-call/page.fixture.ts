/*
 * The on-call roster as the continuum check sees it (#1093, #2748).
 *
 * The ticket names the load this screen owns: fourteen Doulas and a full
 * month of births, at 320px. So this fixture carries exactly that, and
 * the content is hostile rather than polite (ADR-0025, #537): a
 * double-barreled Client name beside a Doula with a professional suffix,
 * a pasted URL where a name should be, a Doula on two births at once, a
 * gap nobody covers, a gap with a covering Doula, a birth with no Doula
 * yet, a birth with its own start, and a run of days nobody is on call for.
 * Today is Saturday November 14, 2026 in the Practice's zone.
 */
import type { OnCallPageData } from './+page.js';
import type { CoverageGap, Roster, RosterDoula, RosterOnCall, RosterWindow, Tonight } from '#lib/onCall.js';
import { practiceSession, type RouteFixture, type RouteVariant } from '../../../routeFixture.js';
import { nth, TYPICAL_CLIENTS, TYPICAL_DOULAS } from '../../../typicalPilot.js';
import type { RouteParams as RouteParameters } from './$types';
import Page from './OnCallScreen.svelte';

export const NOW = new Date('2026-11-14T15:00:00Z');
const TODAY = '2026-11-14';
const LAST_DAY = '2026-12-13';

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
	'Ida Bell'
];

const staffIdOf = (index: number) => `staff-${index + 1}`;

function onCall(index: number, from: string, to: string, extra: Partial<RosterOnCall> = {}): RosterOnCall {
	return { staffId: staffIdOf(index), name: doulaNames[index]!, from, to, narrowed: false, ...extra };
}

function gap(overrides: Partial<CoverageGap> & Pick<CoverageGap, 'id' | 'engagementId'>): CoverageGap {
	return {
		staffId: staffIdOf(0),
		staffName: doulaNames[0]!,
		startsAt: '2026-11-14T23:00:00Z',
		endsAt: '2026-11-15T14:00:00Z',
		...overrides
	};
}

const holder = (index: number, coveringFor?: number) => ({
	staffId: staffIdOf(index),
	name: doulaNames[index]!,
	coveringFor: coveringFor === undefined ? undefined : doulaNames[coveringFor]
});

const windows: RosterWindow[] = [
	{
		engagementId: 'eng-1',
		clientName: 'Anne-Marie Ochieng-Whitfield',
		dueDate: '2026-11-25',
		window: { start: '2026-11-04', end: '2026-12-09' },
		onCall: [
			onCall(0, '2026-11-04', '2026-12-09', {
				atOnce: [{ engagementId: 'eng-2', clientName: 'Bea Okafor', days: { start: TODAY, end: '2026-11-22' } }]
			})
		],
		gaps: [gap({ id: 'gap-1', engagementId: 'eng-1' })],
		unstaffedDays: [],
		uncovered: true,
		tonight: { holders: [], isNobody: true }
	},
	{
		engagementId: 'eng-2',
		clientName: 'Bea Okafor',
		dueDate: '2026-11-20',
		window: { start: '2026-11-01', end: '2026-12-04' },
		onCall: [
			onCall(0, '2026-11-01', '2026-11-22', {
				narrowed: true,
				atOnce: [
					{
						engagementId: 'eng-1',
						clientName: 'Anne-Marie Ochieng-Whitfield',
						days: { start: TODAY, end: '2026-11-22' }
					}
				]
			}),
			onCall(1, '2026-11-23', '2026-12-04', { narrowed: true })
		],
		gaps: [
			gap({
				id: 'gap-2',
				engagementId: 'eng-2',
				staffId: staffIdOf(1),
				staffName: doulaNames[1]!,
				startsAt: '2026-11-24T22:00:00Z',
				endsAt: '2026-11-25T10:00:00Z',
				coveringStaffId: staffIdOf(2),
				coveringStaffName: doulaNames[2]!
			})
		],
		unstaffedDays: [],
		uncovered: false,
		tonight: { holders: [holder(0)], isNobody: false }
	},
	{
		engagementId: 'eng-3',
		clientName: 'https://portal.highland-midwifery-group.example.org/referrals/2027/persephone?source=intake',
		dueDate: '2026-12-02',
		window: { start: '2026-11-11', end: '2026-12-16' },
		noDoula: true,
		onCall: [],
		gaps: [],
		unstaffedDays: [{ start: TODAY, end: '2026-12-16' }],
		uncovered: true,
		tonight: { holders: [], isNobody: true }
	},
	{
		engagementId: 'eng-4',
		clientName: 'Cleo Marsh',
		dueDate: '2026-11-26',
		window: { start: '2026-11-05', end: '2026-12-10' },
		ownStart: { startRule: 'gestational_week', startWeek: 38 },
		onCall: [onCall(3, '2026-11-05', '2026-12-10')],
		gaps: [
			gap({
				id: 'gap-3',
				engagementId: 'eng-4',
				staffId: staffIdOf(3),
				staffName: doulaNames[3]!,
				coveringStaffId: staffIdOf(5),
				coveringStaffName: doulaNames[5]!
			})
		],
		unstaffedDays: [],
		uncovered: false,
		tonight: { holders: [holder(5, 3)], isNobody: false }
	},
	{
		engagementId: 'eng-6',
		clientName: 'Casey Estrada',
		dueDate: '2026-12-12',
		window: { start: '2026-11-10', end: '2026-12-26' },
		onCall: [onCall(6, '2026-11-10', '2026-12-26')],
		gaps: [],
		unstaffedDays: [],
		uncovered: false,
		tonight: { holders: [holder(6)], isNobody: false }
	},
	{
		engagementId: 'eng-5',
		clientName: 'Dot Nakamura',
		dueDate: '2026-12-20',
		window: { start: '2026-11-29', end: '2027-01-03' },
		ownStart: { startRule: 'attachment_granted' },
		onCall: [onCall(4, '2026-11-29', '2027-01-03')],
		gaps: [],
		unstaffedDays: [{ start: '2026-11-29', end: '2026-12-03' }],
		uncovered: true
	}
];

const doulas: RosterDoula[] = doulaNames.map((name, index) => ({
	staffId: staffIdOf(index),
	name,
	births: [],
	unreachable: [],
	isOnCallTonight: false
}));
doulas[0] = {
	...doulas[0]!,
	births: [
		{ engagementId: 'eng-1', clientName: 'Anne-Marie Ochieng-Whitfield', covering: false },
		{ engagementId: 'eng-2', clientName: 'Bea Okafor', covering: false }
	],
	atOnce: [{ days: { start: TODAY, end: '2026-11-22' }, engagementIds: ['eng-1', 'eng-2'] }],
	unreachable: [
		{
			gapId: 'gap-1',
			engagementId: 'eng-1',
			clientName: 'Anne-Marie Ochieng-Whitfield',
			startsAt: '2026-11-14T23:00:00Z',
			endsAt: '2026-11-15T14:00:00Z',
			isCovered: false
		}
	],
	isOnCallTonight: true
};
doulas[2] = { ...doulas[2]!, births: [{ engagementId: 'eng-2', clientName: 'Bea Okafor', covering: true }] };

export const roster: Roster = {
	from: TODAY,
	to: LAST_DAY,
	today: TODAY,
	rule: { startRule: 'gestational_week', startWeek: 37, graceDays: 14 },
	windows,
	noWindow: [
		{ engagementId: 'eng-no-due', clientName: 'Dot Nakamura', reason: 'no_due_date', doulas: [{ staffId: staffIdOf(3), name: doulaNames[3]! }] },
		{
			engagementId: 'eng-no-due-no-doula',
			clientName: 'Wilhelmina Featherstonehaugh-Pemberton',
			reason: 'no_due_date_no_doula',
			doulas: []
		}
	],
	doulas
};

export const data: OnCallPageData = {
	roster,
	view: { preset: 30, isMine: false, doula: '', needsCover: false, order: '' },
	filters: { from: TODAY, to: LAST_DAY },
	now: NOW
};

/*
 * Typical pilot data (#1981), the screen this route's table floor is
 * measured on (`typicalPilot.ts` says where each value comes from). A
 * 14-doula agency with 32 births spread over six months has five or six
 * due in a month: most have one Doula on call for the whole window, one is
 * covered by a backup, one has a Doula who cannot be reached tonight with
 * another covering, and one has days nobody is on call for.
 */
const typicalStaff = TYPICAL_DOULAS.map((doula) => ({ staffId: doula.staffId, name: doula.name }));

type Typical = { staffId: string; name: string };

function tonightOf(index: number, primary: Typical, backup: Typical, isUncovered: boolean): Tonight {
	if (isUncovered) return { holders: [], isNobody: true };
	if (index === 0) return { holders: [{ ...backup, coveringFor: primary.name }], isNobody: false };
	return { holders: [primary], isNobody: false };
}

const typicalWindows: RosterWindow[] = TYPICAL_CLIENTS.slice(0, 6).map((person, index) => {
	const primary = nth(typicalStaff, index + 2);
	const backup = nth(typicalStaff, index + 3);
	const day = String(index * 4 + 14).padStart(2, '0');
	const isTonight = index < 3;
	const isUncovered = index === 1;
	return {
		engagementId: `eng-${index + 1}`,
		clientName: person.name,
		dueDate: `2026-11-${String(index * 4 + 20).padStart(2, '0')}`,
		window: { start: `2026-11-${isTonight ? '04' : day}`, end: `2026-12-${String(index + 10).padStart(2, '0')}` },
		onCall: [{ ...primary, from: '2026-11-04', to: '2026-12-10', narrowed: false }],
		gaps:
			index === 0
				? [
						gap({
							id: 'gap-1',
							engagementId: 'eng-1',
							staffId: primary.staffId,
							staffName: primary.name,
							coveringStaffId: backup.staffId,
							coveringStaffName: backup.name
						})
					]
				: [],
		unstaffedDays: isUncovered ? [{ start: TODAY, end: '2026-11-18' }] : [],
		uncovered: isUncovered,
		tonight: isTonight ? tonightOf(index, primary, backup, isUncovered) : undefined
	};
});

export const typical: RouteVariant<RouteParameters> = {
	name: 'The on-call roster, with typical pilot data',
	typical: true,
	props: {
		data: {
			...data,
			roster: {
				...roster,
				windows: typicalWindows,
				noWindow: [{ engagementId: 'eng-no-due', clientName: nth(TYPICAL_CLIENTS, 10).name, reason: 'no_due_date', doulas: [] }],
				doulas: typicalStaff.map((doula) => ({ ...doula, births: [], unreachable: [], isOnCallTonight: false }))
			}
		} satisfies OnCallPageData
	}
};

/*
 * #2255: the screen while its read runs -- the days the URL names, and a
 * row skeleton where the births will land.
 */
export const loading: RouteVariant<RouteParameters> = {
	name: 'The on-call roster, while its read runs',
	props: {}
};

/*
 * An employed Doula reads the whole Practice but is offered neither the
 * Doula picker nor "Needing cover", and no link to the rule; a contractor
 * is offered "Order" alone and sees only the births she is on, with no
 * colleague's overlap. Both draw the roster's one session-dependent branch
 * the Owner's tree does not.
 */
export const asDoula: RouteVariant<RouteParameters> = {
	name: 'The on-call roster, as an employed Doula',
	pageData: { session: { ...practiceSession(['doula']).session, hasOtherDoulas: true } }
};

export const asContractor: RouteVariant<RouteParameters> = {
	name: 'The on-call roster, as a contractor Doula',
	pageData: { session: { ...practiceSession(['doula']).session, isContractor: true } },
	props: {
		data: {
			...data,
			roster: {
				...roster,
				windows: windows.slice(0, 2).map((row) => ({ ...row, onCall: row.onCall.map(({ atOnce, ...rest }) => (void atOnce, rest)) })),
				noWindow: [],
				doulas: doulas.slice(0, 2).map(({ atOnce, ...rest }) => (void atOnce, rest))
			}
		} satisfies OnCallPageData
	}
};

export const fixture: RouteFixture<RouteParameters> = {
	name: 'The on-call roster',
	variants: [typical, loading, asDoula, asContractor],
	component: Page,
	params: { practiceId: 'practice-1' },
	url: 'https://example.test/practices/practice-1/on-call',
	props: { data },
	pageData: practiceSession(['owner']),
	// The route takes its roster from `load` rather than from a fetch, so
	// the screen is on the page as soon as it renders; the heading is
	// what proves it rendered the screen and not an error state.
	readyText: 'On call'
};
