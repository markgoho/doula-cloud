import { error, redirect } from '@sveltejs/kit';
import { resolve } from '$app/paths';
import { apiErrorMessage, apiFetch } from '#lib/api.js';
import { refuseRead } from '#lib/errorPage.js';
import { engagementURL, type EngagementSummary } from '#lib/engagementDetail.js';
import type { PageLoad } from './$types';

/**
 * The Birth Plan's own address on the Practice side (#280).
 *
 * Reads the same Engagement summary the hub's own `+page.ts` reads, one
 * level up -- not a second endpoint, and not `#lib/planInstance.js`'s own
 * read. Two reasons: a page opened cold has to name whose Client and
 * Engagement this is (this route's own AC), and the read's own refusal
 * already carries ADR-0008's attachment rule -- an unattached contractor
 * Doula 403s here exactly as she does on the hub, before the Birth Plan
 * read itself is ever asked. That read enforces the identical rule on its
 * own (api/internal/plans/instance_test.go's
 * TestGetInstanceHandler_ContractorWithoutAttachmentForbidden), so this
 * load is a second door onto the same room, not the only lock on it.
 *
 * The Birth Plan itself is not fetched here: it loads after mount, inside
 * `+page.svelte`, the same as every other section on the hub this route
 * split off from -- so naming the Client never waits on the plan render,
 * and a slow or failed plan fetch never blocks the page that says who it
 * belongs to.
 */
export const load: PageLoad = async ({ params }): Promise<EngagementSummary> => {
	const response = await apiFetch(
		engagementURL({ practiceId: params.practiceId, engagementId: params.engagementId })
	);

	if (response.status === 401) {
		redirect(303, `${resolve('/(signed-out)/login')}?sessionEnded=true`);
	} else if (response.status === 403) {
		await refuseRead(response);
	} else if (!response.ok) {
		error(response.status, await apiErrorMessage(response));
	}

	return response.json();
};
