/*
 * The door to a Client's details journey (#1610). Her Engagement's page
 * links here and it sends the reader to the first question, her name.
 * Each Change on her record links here too, naming its question
 * (`?step=`), and it sends the reader to that one question, marked as
 * reached from the record (#2048): Back goes to the record, and the
 * question saves.
 *
 * ## Why the draft is cleared here
 *
 * The same reason intake's own door gives: this URL is where a visit
 * starts, never somewhere a reader passes through in the middle of one.
 * The draft is mirrored per tab, so without this a second visit would
 * open what the first left half-typed rather than what her record now
 * holds.
 */
import { redirect } from '@sveltejs/kit';
import { clientDetails } from '#lib/clientDetailsFlow.svelte.js';
import { engagementOrigin, stepOrigin } from '#lib/clientDetailsJourney.js';
import { RECORD_QUERY } from '#lib/intakeJourney.js';
import { detailsBasePath } from './details.js';
import type { PageLoad } from './$types';

export const load: PageLoad = ({ params, url }) => {
	clientDetails.open(params.clientId, engagementOrigin(url.searchParams));
	const base = detailsBasePath(params.practiceId, params.clientId);
	const step = stepOrigin(url.searchParams);
	redirect(307, step ? `${base}/${step}?${RECORD_QUERY}` : `${base}/name`);
};
