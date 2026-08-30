import { redirect, error } from '@sveltejs/kit';
import { resolve } from '$app/paths';
import { apiFetch, apiErrorMessage } from '#lib/api.js';
import type { LayoutLoad } from './$types';

export interface EngagementIdentity {
	practiceName: string;
	clientName: string;
}

/**
 * Loads through SvelteKit's `load`, not the app's usual onMount-fetch,
 * specifically so the Practice's name -- the portal's own identity (#431)
 * -- is known before first paint. Every route under here can read
 * `page.data.practiceName` (via `$app/state`) to build a title that says
 * the same thing the chrome does, rather than one that starts generic and
 * flips once an onMount fetch resolves (#487). `apiFetch`, not
 * `apiFetchWithSession`: that helper's 401 handling calls `goto()`, which
 * is the wrong tool mid-`load` (#471's rule, same as `billing/+page.ts`).
 *
 * This does not replace the hub page's own fetch of the same endpoint for
 * `status`/`createdAt` -- that duplication predates this ticket and is out
 * of scope here.
 */
export const load: LayoutLoad = async ({ params }): Promise<EngagementIdentity> => {
	const response = await apiFetch(`/api/portal/engagements/${params.engagementId}`);

	if (response.status === 401) {
		redirect(303, `${resolve('/portal/(signed-out)/login')}?sessionEnded=true`);
	} else if (!response.ok) {
		error(response.status, await apiErrorMessage(response));
	}

	return response.json();
};
