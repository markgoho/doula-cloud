import { error, redirect } from '@sveltejs/kit';
import { resolve } from '$app/paths';
import { apiErrorMessage, apiFetch } from '#lib/api.js';
import { engagementURL, type EngagementSummary } from '#lib/engagementDetail.js';
import type { PageLoad } from './$types';

/**
 * The Engagement itself, loaded before the page renders (#695).
 *
 * Only this one section moves into `load`. The other six -- Visits,
 * Messages, both Plan types, the Contract, the Invoices, the Offers --
 * stay after mount on purpose: they arrive one at a time and each renders
 * as it lands, and awaiting all seven here would hold the whole page back
 * for the slowest of them.
 *
 * This one is different because without it there is no page. Loading it
 * here also puts a refusal in front of `practices/+error.svelte` rather
 * than in a local error string, the same move #471 made for billing --
 * which matters more here, since a Doula who is not attached to this
 * Engagement gets a 403 (ADR-0008, #350).
 *
 * `apiFetch`, not `apiFetchWithSession`: that helper's 401 handling calls
 * `goto()`, which is the wrong tool mid-load.
 */
export const load: PageLoad = async ({ params }): Promise<EngagementSummary> => {
	const response = await apiFetch(
		engagementURL({ practiceId: params.practiceId, engagementId: params.engagementId })
	);

	if (response.status === 401) {
		redirect(303, `${resolve('/(signed-out)/login')}?sessionEnded=true`);
	} else if (response.status === 403) {
		error(403, 'not permitted to read this');
	} else if (!response.ok) {
		error(response.status, await apiErrorMessage(response));
	}

	return response.json();
};
