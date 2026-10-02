import { apiFetch } from '#lib/api.js';
import { feedbackPiecePath, type FeedbackPiece } from '#lib/founderFeedback.js';
import { refuseFounderRead } from '../founderRead.js';
import type { PageLoad } from './$types';

/**
 * One piece of Feedback (#1526). This request is the read: the BFF
 * writes its `feedback_reads` row before it answers, every time this
 * `load` runs. So nothing may run it but an open of the page -- the
 * list's links opt out of SvelteKit's hover preload for that reason
 * (`feedback/+page.svelte`), and this page never re-fetches to refresh.
 */
export const load: PageLoad = async ({ params, url }): Promise<{ piece: FeedbackPiece }> => {
	const response = await apiFetch(feedbackPiecePath(params.feedbackId));
	await refuseFounderRead(response, url.pathname);

	return { piece: await response.json() };
};
