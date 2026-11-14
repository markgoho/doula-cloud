import { redirect, error } from '@sveltejs/kit';
import { apiFetch } from '#lib/api.js';
import { apiErrorMessage } from '#lib/apiErrorMessage.js';
import { PORTAL_ENGAGEMENT_DEPENDENCY, noteEngagementRead } from '#lib/portalEngagementRead.js';
import { portalLoginAfterRefusal } from '#lib/sessionEnded.js';
import type { LayoutLoad } from './$types';

export interface EngagementIdentity {
	practiceName: string;
	/** The IANA zone the Practice works in; every portal page reads it as
	 * `page.data.practiceTimezone`. */
	practiceTimezone: string;
	clientName: string;
	/** When the Engagement began (#310) -- the authenticated chrome's own
	 * way back to the portal root needs it to build the same
	 * `engagementLabel` the root list and the choosers use. */
	createdAt: string;
	/** portal.Detail's own resolved answer to ADR-0015's suppression
	 * question (#311) -- never the Engagement's kind itself, which
	 * GLOSSARY.md gives no Client word and which this DTO never carries.
	 * Every portal surface that offers, links to or announces a Birth
	 * Plan reads this one field. */
	offersBirthPlan: boolean;
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
 * The hub reads the rest of this same response from `page.data` (#2587)
 * instead of asking again; `createdAt` is read off it too, for the chrome's
 * own switcher label (#310). Through the `fetch` that `load` is passed, as
 * SvelteKit asks.
 */
export const load: LayoutLoad = async ({ params, url, untrack, fetch, depends }): Promise<EngagementIdentity> => {
	depends(PORTAL_ENGAGEMENT_DEPENDENCY);
	const response = await apiFetch(`/api/portal/engagements/${params.engagementId}`, {}, fetch);

	if (response.status === 401) {
		// Untracked, so moving between this Engagement's pages does not read it again.
		redirect(303, portalLoginAfterRefusal(untrack(() => `${url.pathname}${url.search}`)));
	} else if (!response.ok) {
		error(response.status, await apiErrorMessage(response));
	}

	noteEngagementRead();
	return response.json();
};
