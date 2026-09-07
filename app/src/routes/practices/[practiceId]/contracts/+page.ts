import { error, redirect } from '@sveltejs/kit';
import { resolve } from '$app/paths';
import { apiFetch, apiErrorMessage } from '#lib/api.js';
import { practiceAwaitingContractsPath } from '#lib/contract.js';
import type { CursorPage } from '#lib/paginatedList.svelte.js';
import type { AwaitingContract } from '#lib/contract.js';
import type { PageLoad } from './$types';

/**
 * Loads through SvelteKit's `load`, mirroring the Practice-wide Invoice
 * list's own `+page.ts` (#265): this read is Owner/Admin-only, the same
 * declaration the endpoint's own doc comment argues against ADR-0008, so
 * a Doula who types this URL gets a 403 from the BFF and that refusal has
 * to reach `practices/+error.svelte` rather than sit in a local error
 * string this page owns. `apiFetch`, not `apiFetchWithSession`: that
 * helper's 401 handling calls `goto()`, which is the wrong tool
 * mid-`load`.
 */
export const load: PageLoad = async ({ params }): Promise<CursorPage<AwaitingContract>> => {
	const response = await apiFetch(practiceAwaitingContractsPath(params.practiceId));

	if (response.status === 401) {
		redirect(303, `${resolve('/(signed-out)/login')}?sessionEnded=true`);
	} else if (response.status === 403) {
		error(403, 'not permitted to read this');
	} else if (!response.ok) {
		error(response.status, await apiErrorMessage(response));
	}

	return response.json();
};
