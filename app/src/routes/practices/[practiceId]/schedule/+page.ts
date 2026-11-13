import { canNarrowToMine, isOwnerOrAdmin, type LabeledValue } from '#lib/roles.js';
import { doulaOptions, doulasOf, staffPath, type Roster } from '#lib/staff.js';
import { streamRead, type StreamedRead } from '#lib/streamedRead.svelte.js';
import {
	practiceSchedulePath,
	scheduleDaysOf,
	scheduleStaffId,
	scheduleViewFromParameters,
	type ResolvedScheduleFilters,
	type ScheduledVisit,
	type ScheduleRangeErrors,
	type ScheduleView
} from '#lib/visitSchedule.js';
import type { CursorPage } from '#lib/paginatedList.svelte.js';
import type { PageLoad } from './$types';

export const _SCHEDULE_READ = 'app:schedule';

export interface SchedulePageData {
	page: CursorPage<ScheduledVisit>;
	/**
	What the address said.
	*/
	view: ScheduleView;
	/**
	 * The days and Doula the rows were read under, which "Load more"
	 * continues. The days are empty when the address named days that are not
	 * a range, and nothing was read.
	 */
	filters: ResolvedScheduleFilters;
	/**
	The instant the days were worked out from, so a heading's "Today" and the days cannot disagree.
	*/
	now: Date;
	/**
	Why the address's days are not a range, per field.
	*/
	rangeErrors?: ScheduleRangeErrors;
	/**
	 * The Doulas the filter offers, empty when this Staff member cannot be
	 * offered one at all. `GET .../staff` is Owner/Admin on the BFF, which
	 * is not an oversight -- ADR-0006 grants an Admin the roster precisely
	 * so she can do scheduling -- so a Doula reading her own Practice's
	 * schedule gets the days and no Doula list, rather than a control
	 * whose options the endpoint would refuse to hand over.
	 */
	doulas: LabeledValue[];
	/**
	The roster read failed: the list stands, and the Doula filter waits.
	*/
	isRosterUnavailable: boolean;
}

const NOTHING: CursorPage<ScheduledVisit> = { items: [], hasMore: false };

/**
 * Loads through SvelteKit's `load`, mirroring the Practice-wide Contract
 * and Invoice lists (#273, #265), and hands the reads over unawaited, so
 * a click changes screen at once (#2249). The schedule and the roster are
 * read at the same time, and the roster failing leaves the list standing.
 *
 * The narrowing lives in the URL and is read here, so a shared link works
 * on a cold load. Days that are not a range, and "Mine" beside a different
 * Doula, are answered here with an empty list and no read: the BFF is
 * never sent a range that can never match.
 */
export const load: PageLoad = async ({
	params,
	url,
	parent,
	fetch,
	depends
}): Promise<{ read: Promise<StreamedRead<SchedulePageData>> }> => {
	// The screen's "Try again" runs this read again.
	depends(_SCHEDULE_READ);
	const { session } = await parent();
	const view = scheduleViewFromParameters(
		url.searchParams,
		canNarrowToMine(session),
		isOwnerOrAdmin(session) ? undefined : { staffId: session.staffId }
	);
	const now = new Date();
	const read = scheduleDaysOf(view, now);
	const whose = scheduleStaffId(view, session.staffId);
	const filters: ResolvedScheduleFilters = read.days
		? { ...read.days, staffId: whose.staffId }
		: { from: '', to: '' };
	const isReadable = read.days !== undefined && !whose.isEmpty;
	const schedule: Promise<StreamedRead<CursorPage<ScheduledVisit>>> = isReadable
		? streamRead<CursorPage<ScheduledVisit>>(practiceSchedulePath(params.practiceId, filters), fetch)
		: Promise.resolve({ isLoaded: true, value: NOTHING });
	const roster = isOwnerOrAdmin(session)
		? loadDoulaOptions(params.practiceId, fetch)
		: Promise.resolve({ doulas: [], isRosterUnavailable: false });
	const joined = async (): Promise<StreamedRead<SchedulePageData>> => {
		const [landed, doulas] = await Promise.all([schedule, roster]);
		if (!landed.isLoaded) return landed;
		return { isLoaded: true, value: { page: landed.value, view, filters, now, rangeErrors: read.errors, ...doulas } };
	};
	return { read: joined() };
};

/**
 * The Staff roster, reduced to the people a Visit can be assigned to.
 * A Visit can only ever be assigned to a holder of the doula role
 * (api/internal/visit's `requireEligibleAssignee`), so anybody else would
 * never appear on a schedule row and would only narrow the list to
 * nothing. `staff.ts` owns the reduction (#268) -- this screen's filter
 * and the Engagement page's Visit pickers offer the same people.
 */
async function loadDoulaOptions(
	practiceId: string,
	fetcher: typeof globalThis.fetch
): Promise<{ doulas: LabeledValue[]; isRosterUnavailable: boolean }> {
	const read = await streamRead<Roster>(staffPath(practiceId), fetcher);
	if (!read.isLoaded) return { doulas: [], isRosterUnavailable: true };
	return { doulas: doulaOptions(doulasOf(read.value)), isRosterUnavailable: false };
}
