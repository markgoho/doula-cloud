import { error, redirect } from '@sveltejs/kit';
import { resolve } from '$app/paths';
import { apiFetch, apiErrorMessage } from '#lib/api.js';
import { practiceAwaitingContractsPath, practiceAwaitingVoidRequestsPath } from '#lib/contract.js';
import type { CursorPage } from '#lib/paginatedList.svelte.js';
import type { AwaitingContract, AwaitingVoidRequest } from '#lib/contract.js';
import type { PageLoad } from './$types';

/** The two Practice-wide, Owner/Admin-only roll-ups this route shows
 * (#273, #971) -- one screen, two work lists, since #971's own AC is
 * "sees the void requests waiting on them" beside the Contracts already
 * waiting on a signature, not a second address to remember. */
export interface ContractsPageData {
	contracts: CursorPage<AwaitingContract>;
	voidRequests: CursorPage<AwaitingVoidRequest>;
}

/**
 * Loads through SvelteKit's `load`, mirroring the Practice-wide Invoice
 * list's own `+page.ts` (#265): this read is Owner/Admin-only, the same
 * declaration the endpoint's own doc comment argues against ADR-0008, so
 * a Doula who types this URL gets a 403 from the BFF and that refusal has
 * to reach `practices/+error.svelte` rather than sit in a local error
 * string this page owns. `apiFetch`, not `apiFetchWithSession`: that
 * helper's 401 handling calls `goto()`, which is the wrong tool
 * mid-`load`. Both roll-ups load in parallel -- neither depends on the
 * other -- and share the same three status branches, since both endpoints
 * carry the same Owner/Admin declaration and the same session either
 * caller reads through.
 */
export const load: PageLoad = async ({ params }): Promise<ContractsPageData> => {
	const [contractsResponse, voidRequestsResponse] = await Promise.all([
		apiFetch(practiceAwaitingContractsPath(params.practiceId)),
		apiFetch(practiceAwaitingVoidRequestsPath(params.practiceId))
	]);

	for (const response of [contractsResponse, voidRequestsResponse]) {
		if (response.status === 401) {
			redirect(303, `${resolve('/(signed-out)/login')}?sessionEnded=true`);
		} else if (response.status === 403) {
			error(403, 'not permitted to read this');
		} else if (!response.ok) {
			error(response.status, await apiErrorMessage(response));
		}
	}

	return {
		contracts: await contractsResponse.json(),
		voidRequests: await voidRequestsResponse.json()
	};
};
