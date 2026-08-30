import { error, redirect } from '@sveltejs/kit';
import { resolve } from '$app/paths';
import { apiFetch, apiErrorMessage } from '#lib/api.js';
import { billingPath, type Balance } from '#lib/billing.js';
import type { PageLoad } from './$types';

/**
 * Loads through SvelteKit's `load` rather than the app's usual
 * onMount-fetch, specifically so a role refusal reaches `practices/
 * +error.svelte` (#471) -- billing reads are Owner/Admin-only
 * (ADR-0008), and a Doula who types this URL gets a 403 from the BFF.
 * `apiFetch`, not `apiFetchWithSession`: that helper's 401 handling
 * calls `goto()`, which is the wrong tool mid-`load`.
 */
export const load: PageLoad = async ({ params }): Promise<Balance> => {
	const response = await apiFetch(billingPath(params.practiceId));

	if (response.status === 401) {
		redirect(303, `${resolve('/(signed-out)/login')}?sessionEnded=true`);
	} else if (response.status === 403) {
		error(403, 'not permitted to read this');
	} else if (!response.ok) {
		error(response.status, await apiErrorMessage(response));
	}

	return response.json();
};
