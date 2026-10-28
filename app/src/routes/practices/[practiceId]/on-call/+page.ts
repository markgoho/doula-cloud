import { mapRead, streamRead, type StreamedRead } from '#lib/streamedRead.svelte.js';
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
 * (#263), and hands the read over unawaited, so a click changes screen at
 * once (#2249). A refusal is drawn by the screen as the page
 * `practices/+error.svelte` would draw it.
 *
 * The range lives in the URL, so a link a colleague is sent opens on the
 * same days, and a reload paints the same roster. Its default is today
 * through the next 30 days (#2026), so a first visit shows the births
 * coming up rather than an empty day.
 *
 * This read is the same for every Staff member, and what comes back is
 * not: the BFF gives a contractor Doula only the births she holds a
 * granted Attachment for. Nothing here narrows anything, and nothing
 * here may -- a hidden control is not a refusal.
 */
export const load: PageLoad = ({
	params,
	url,
}): { read: Promise<StreamedRead<OnCallPageData>> } => {
	const range = rangeFromParameters(url.searchParams);
	return {
		read: mapRead(streamRead<Roster>(rosterPath(params.practiceId, range)), (roster) => ({
			roster,
			range,
		})),
	};
};
