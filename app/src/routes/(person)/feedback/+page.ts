import { apiFetch } from '#lib/api.js';
import { feedbackListPath, type FeedbackPage } from '#lib/founderFeedback.js';
import { refuseFounderRead } from './founderRead.js';
import type { PageLoad } from './$types';

export interface FeedbackListData {
	/**
	The pieces whose issue open has failed -- printed first.
	*/
	unopened: FeedbackPage;
	all: FeedbackPage;
}

/**
 * The first page of each of the founder's two lists (#1526). `apiFetch`,
 * not `apiFetchWithSession`: that helper's 401 handling calls `goto()`,
 * the wrong tool mid-`load` (#471's rule); `refuseFounderRead` does the
 * same hand-offs with `redirect`.
 */
export const load: PageLoad = async ({ url }): Promise<FeedbackListData> => {
	const [unopenedResponse, allResponse] = await Promise.all([
		apiFetch(feedbackListPath(undefined, true)),
		apiFetch(feedbackListPath())
	]);
	await refuseFounderRead(unopenedResponse, url.pathname);
	await refuseFounderRead(allResponse, url.pathname);

	return { unopened: await unopenedResponse.json(), all: await allResponse.json() };
};
