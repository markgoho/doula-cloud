import { error, redirect } from '@sveltejs/kit';
import { resolve } from '$app/paths';
import { apiFetch, apiErrorMessage } from '#lib/api.js';
import { isOwnerOrAdmin, type LabeledValue } from '#lib/roles.js';
import { loadStaff } from '#lib/staff.js';
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
 * and Invoice lists (#273, #265): the refusal has to reach
 * `practices/+error.svelte` rather than sit in a local error string this
 * page owns. `apiFetch`, not `apiFetchWithSession`: that helper's 401
 * handling calls `goto()`, which is the wrong tool mid-`load`.
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
export const load: PageLoad = async ({ params, url, parent }): Promise<SchedulePageData> => {
	const { session } = await parent();
	const filters = scheduleFiltersFromParameters(url.searchParams);

	const response = await apiFetch(practiceSchedulePath(params.practiceId, filters));

	if (response.status === 401) {
		redirect(303, `${resolve('/(signed-out)/login')}?sessionEnded=true`);
	} else if (response.status === 403) {
		error(403, 'not permitted to read this');
	} else if (!response.ok) {
		error(response.status, await apiErrorMessage(response));
	}

	const page: CursorPage<ScheduledVisit> = await response.json();
	const doulas = isOwnerOrAdmin(session) ? await loadDoulaOptions(params.practiceId) : [];
	return { page, filters, doulas };
};

/**
 * The Staff roster, reduced to the people a Visit can be assigned to.
 * `requireDoula` gates every Visit write on the BFF, so a Staff member
 * without the doula role can never appear on a schedule row and would
 * only ever narrow the list to nothing.
 */
async function loadDoulaOptions(practiceId: string): Promise<LabeledValue[]> {
	const roster = await loadStaff(apiFetch, practiceId);
	return roster.members
		.filter((member) => member.roles.includes('doula'))
		.map((member) => ({ value: member.staffId, label: member.name }));
}
