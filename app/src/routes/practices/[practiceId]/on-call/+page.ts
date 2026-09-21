import { error, redirect } from '@sveltejs/kit';
import { apiFetch, apiErrorMessage } from '#lib/api.js';
import { refuseRead } from '#lib/errorPage.js';
import { staffLoginAfterSessionEnded } from '#lib/sessionEnded.js';
import {
	rangeFromParameters,
	rosterPath,
	type Roster,
	type RosterRange,
} from '#lib/onCall.js';
import type { PageLoad } from './$types';

export interface OnCallPageData {
	roster: Roster;
	range: RosterRange;
}

/**
 * Loads through SvelteKit's `load`, like the Practice-wide schedule
 * (#263): a refusal has to reach `practices/+error.svelte` rather than
 * sit in a local error string this page owns. `apiFetch`, not
 * `apiFetchWithSession`, whose 401 handling calls `goto()` -- the wrong
 * tool mid-`load`.
 *
 * The range lives in the URL, so a link a colleague is sent opens on the
 * same days, and a reload paints the same roster. Its default is today,
 * which is the question the screen exists for.
 *
 * This read is the same for every Staff member, and what comes back is
 * not: the BFF gives a contractor Doula only the births she holds a
 * granted Attachment for. Nothing here narrows anything, and nothing
 * here may -- a hidden control is not a refusal.
 */
export const load: PageLoad = async ({
	params,
	url,
}): Promise<OnCallPageData> => {
	const range = rangeFromParameters(url.searchParams);
	const response = await apiFetch(rosterPath(params.practiceId, range));

	if (response.status === 401) {
		redirect(303, staffLoginAfterSessionEnded());
	} else if (response.status === 403) {
		await refuseRead(response);
	} else if (!response.ok) {
		error(response.status, await apiErrorMessage(response));
	}

	return { roster: await response.json(), range };
};
