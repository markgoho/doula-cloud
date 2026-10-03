/*
 * The door to a Client's details journey (#1610). Her record and her
 * Engagement's page link here; it sends the reader to the first
 * question.
 *
 * ## Why the draft is cleared here
 *
 * The same reason intake's own door gives: this URL is where a visit
 * starts, never somewhere a reader passes through in the middle of one.
 * The draft is mirrored per tab, so without this a second visit would
 * open what the first left half-typed rather than what her record now
 * holds -- and the Edit form may have changed it in between.
 */
import { redirect } from '@sveltejs/kit';
import { clientDetails } from '#lib/clientDetailsFlow.svelte.js';
import { engagementOrigin } from '#lib/clientDetailsJourney.js';
import { detailsBasePath } from './details.js';
import type { PageLoad } from './$types';

export const load: PageLoad = ({ params, url }) => {
	clientDetails.open(params.clientId, engagementOrigin(url.searchParams));
	redirect(307, `${detailsBasePath(params.practiceId, params.clientId)}/date-of-birth`);
};
