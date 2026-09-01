import { error, redirect } from '@sveltejs/kit';
import { resolve } from '$app/paths';
import { apiFetch, apiErrorMessage } from '#lib/api.js';
import { practiceInvoicesPath, type PracticeInvoicePage } from '#lib/invoice.js';
import type { PageLoad } from './$types';

/**
 * Loads through SvelteKit's `load` rather than the app's usual
 * onMount-fetch, for the reason the Billing route's `load` gives (#471):
 * this read is Owner/Admin-only (ADR-0006's money row, carried forward by
 * ADR-0008), so a Doula who types this URL gets a 403 from the BFF and
 * that refusal has to reach `practices/+error.svelte` rather than sit in
 * a local error string this page owns. `apiFetch`, not
 * `apiFetchWithSession`: that helper's 401 handling calls `goto()`, which
 * is the wrong tool mid-`load`.
 */
export const load: PageLoad = async ({ params }): Promise<PracticeInvoicePage> => {
	const response = await apiFetch(practiceInvoicesPath(params.practiceId));

	if (response.status === 401) {
		redirect(303, `${resolve('/(signed-out)/login')}?sessionEnded=true`);
	} else if (response.status === 403) {
		error(403, 'not permitted to read this');
	} else if (!response.ok) {
		error(response.status, await apiErrorMessage(response));
	}

	return response.json();
};
