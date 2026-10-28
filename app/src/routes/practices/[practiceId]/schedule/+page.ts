import { isOwnerOrAdmin, type LabeledValue } from '#lib/roles.js';
import { doulaOptions, doulasOf, staffPath, type Roster } from '#lib/staff.js';
import { bothRead, mapRead, streamRead, type StreamedRead } from '#lib/streamedRead.svelte.js';
import {
	practiceSchedulePath,
	scheduleFiltersFromParameters,
	type ResolvedScheduleFilters,
	type ScheduledVisit
} from '#lib/visitSchedule.js';
import type { CursorPage } from '#lib/paginatedList.svelte.js';
import type { PageLoad } from './$types';

export interface SchedulePageData {
	page: CursorPage<ScheduledVisit>;
	filters: ResolvedScheduleFilters;
	/**
	 * The Doulas the filter offers, empty when this Staff member cannot be
	 * offered one at all. `GET .../staff` is Owner/Admin on the BFF, which
	 * is not an oversight -- ADR-0006 grants an Admin the roster precisely
	 * so she can do scheduling -- so a Doula reading her own Practice's
	 * schedule gets the date range and no Doula list, rather than a control
	 * whose options the endpoint would refuse to hand over. The schedule
	 * read itself accepts `staffId` from any role; only the picker is
	 * gated, and only because its data source is.
	 */
	doulas: LabeledValue[];
}

/**
 * Loads through SvelteKit's `load`, mirroring the Practice-wide Contract
 * and Invoice lists (#273, #265), and hands the reads over unawaited, so
 * a click changes screen at once (#2249). A refusal is drawn by the screen
 * as the page `practices/+error.svelte` would draw it. The schedule and
 * the roster are read at the same time.
 *
 * Unlike those two, this list is narrowed, and the narrowing lives in the
 * URL. Reading it here rather than in the component is what makes a
 * shared link work on a cold load: the first page the screen paints is
 * already the narrowed one, and a reload of the same URL paints the same
 * rows.
 *
 * Reads the Membership `practices/[practiceId]/+layout.ts` already
 * resolved (#835) through `parent()` rather than a session fetch of its
 * own.
 */
export const load: PageLoad = async ({
	params,
	url,
	parent
}): Promise<{ read: Promise<StreamedRead<SchedulePageData>> }> => {
	const { session } = await parent();
	const filters = scheduleFiltersFromParameters(url.searchParams);
	const schedule = streamRead<CursorPage<ScheduledVisit>>(practiceSchedulePath(params.practiceId, filters));
	const doulas = isOwnerOrAdmin(session) ? loadDoulaOptions(params.practiceId) : noDoulas();
	return {
		read: mapRead(bothRead(schedule, doulas), ([page, doulas]) => ({ page, filters, doulas }))
	};
};

/**
 * The Staff roster, reduced to the people a Visit can be assigned to.
 * A Visit can only ever be assigned to a holder of the doula role
 * (api/internal/visit's `requireEligibleAssignee`), so anybody else would
 * never appear on a schedule row and would only narrow the list to
 * nothing. `staff.ts` owns the reduction (#268) -- this screen's filter
 * and the Engagement page's Visit pickers offer the same people.
 */
function loadDoulaOptions(practiceId: string): Promise<StreamedRead<LabeledValue[]>> {
	return mapRead(streamRead<Roster>(staffPath(practiceId)), (roster) =>
		doulaOptions(doulasOf(roster))
	);
}

async function noDoulas(): Promise<StreamedRead<LabeledValue[]>> {
	return { isLoaded: true, value: [] };
}
