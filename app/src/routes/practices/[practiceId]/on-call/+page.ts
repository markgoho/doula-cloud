import type { RangeErrors } from '#lib/dayRange.js';
import {
	onCallViewFromParameters,
	resolveOnCallRead,
	rosterPath,
	rosterRangeErrors,
	type OnCallFilters,
	type OnCallView,
	type Roster
} from '#lib/onCall.js';
import { canNarrowToMine, isOwnerOrAdmin } from '#lib/roles.js';
import { streamRead, type StreamedRead } from '#lib/streamedRead.svelte.js';
import type { PageLoad } from './$types';

export const _ON_CALL_READ = 'app:on-call';

export interface OnCallPageData {
	/**
	The births, or none when the address named days that are not a range and nothing was read.
	*/
	roster?: Roster;
	/**
	What the address said.
	*/
	view: OnCallView;
	/**
	The days and Doula the births were read under.
	*/
	filters?: OnCallFilters;
	/**
	The instant the days were worked out from, so a heading's "Tonight" and the days cannot disagree.
	*/
	now: Date;
	/**
	Why the address's days are not a range, per field: this screen's own check first, the BFF's refusal second.
	*/
	rangeErrors?: RangeErrors;
}

/**
 * Loads through SvelteKit's `load`, like the Practice-wide schedule (#263),
 * and hands the read over unawaited, so a click changes screen at once
 * (#2249). The narrowing lives in the address, so a link a colleague is sent
 * opens on the same births, and a reload paints the same roster.
 *
 * Days that are not a range are answered here, in place, with no read; a
 * range the BFF still refuses is mapped onto the same fields from its
 * `details`. A refusal of any other kind is the screen's to say, beside
 * "Try again".
 *
 * This read is the same for every Staff member, and what comes back is
 * not: the BFF gives a contractor Doula only the births she holds a
 * granted Attachment for. Nothing here narrows anything, and nothing
 * here may -- a hidden control is not a refusal.
 */
export const load: PageLoad = async ({
	params,
	url,
	parent,
	fetch,
	depends
}): Promise<{ read: Promise<StreamedRead<OnCallPageData>> }> => {
	depends(_ON_CALL_READ);
	const { session } = await parent();
	const view = onCallViewFromParameters(
		url.searchParams,
		canNarrowToMine(session),
		isOwnerOrAdmin(session) ? undefined : { staffId: session.staffId }
	);
	const now = new Date();
	const { filters, errors, isEmpty } = resolveOnCallRead(view, session.timezone, session.staffId, now);
	if (!filters) return { read: Promise.resolve({ isLoaded: true, value: { view, now, rangeErrors: errors } }) };
	const read = async (): Promise<StreamedRead<OnCallPageData>> => {
		const landed = await streamRead<Roster>(rosterPath(params.practiceId, filters), fetch);
		if (landed.isLoaded) {
			const roster = isEmpty ? { ...landed.value, windows: [], noWindow: [] } : landed.value;
			return { isLoaded: true, value: { roster, view, filters, now } };
		}
		const rangeErrors = landed.response ? await rosterRangeErrors(landed.response) : undefined;
		return rangeErrors ? { isLoaded: true, value: { view, filters, now, rangeErrors } } : landed;
	};
	return { read: read() };
};
