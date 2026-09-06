import { redirect, error } from '@sveltejs/kit';
import { resolve } from '$app/paths';
import { apiFetch, apiErrorMessage, isMFARequired } from '#lib/api.js';
import { decideLanding, type SessionInfo } from '#lib/landing.js';
import type { LayoutLoad } from './$types';

/**
 * What one Practice is to the signed-in Staff member: her roles and
 * employment type there, resolved once by `staffauth.Middleware`
 * (api/internal/staffauth/middleware.go) and read back here. Every
 * route under `practices/[practiceId]` reads `page.data.session`
 * (via `#lib/appState.svelte.js`) instead of fetching this endpoint
 * itself -- #835's "resolve the Membership once per navigation."
 */
export interface PracticeSession {
	practiceId: string;
	practiceName: string;
	roles: string[];
	isContractor: boolean;
}

/**
 * Loads through SvelteKit's `load`, not the app's usual onMount-fetch,
 * for the same reason `engagements/[engagementId]/+layout.ts` does: every
 * descendant route reads `page.data.session` before first paint instead
 * of re-fetching `/api/practices/${practiceId}/session` itself. `apiFetch`,
 * not `apiFetchWithSession`: that helper's 401 handling calls `goto()`,
 * the wrong tool mid-`load` (#471's rule).
 *
 * #606: a 403 carrying `{code: "MFA_REQUIRED"}` is a live session barred
 * from *this* Practice only, not a stale Membership -- it routes to
 * enrolment instead, the same as `apiFetchWithSession` does for every
 * other fetch, carrying `returnTo` so enrolment can send her back here.
 * `apiFetch`, not `apiFetchWithSession`, does not run this check itself,
 * so this `load` runs it before the stale-Membership branch below can
 * misread an MFA refusal as one.
 *
 * #748: any other 403, or a 404, here means this session no longer
 * belongs to this Practice -- either she was removed from it, or her
 * Staff row is gone entirely. Either way `/api/staff/session` is what
 * says where she belongs now, decided through `decideLanding` -- the
 * same function `/` uses -- so this is the one place that decision
 * lives, not a copy per route: a Practice she still belongs to (another
 * Membership, or the same one back if this read raced a write) is where
 * she lands, and `/no-practice` is the fallback only once there truly is
 * nowhere else.
 */
export const load: LayoutLoad = async ({ params, url }): Promise<{ session: PracticeSession }> => {
	const response = await apiFetch(`/api/practices/${params.practiceId}/session`);

	if (response.status === 401) {
		redirect(303, `${resolve('/(signed-out)/login')}?sessionEnded=true`);
	} else if (await isMFARequired(response)) {
		redirect(303, `${resolve('/mfa/enroll')}?returnTo=${encodeURIComponent(url.pathname)}`);
	} else if (response.status === 403 || response.status === 404) {
		await redirectAwayFromStalePractice();
	} else if (!response.ok) {
		error(response.status, await apiErrorMessage(response));
	}

	const body: { practiceName: string; roles: string[]; isContractor: boolean } =
		await response.json();
	return {
		session: {
			practiceId: params.practiceId,
			practiceName: body.practiceName,
			roles: body.roles,
			isContractor: body.isContractor
		}
	};
};

/**
 * #748's landing decision for a session whose current Practice just
 * refused it. Always throws -- either a redirect to where she still
 * belongs, or to `/no-practice` once there is nowhere left.
 */
async function redirectAwayFromStalePractice(): Promise<never> {
	const staffResponse = await apiFetch('/api/staff/session');

	if (staffResponse.status === 401) {
		redirect(303, `${resolve('/(signed-out)/login')}?sessionEnded=true`);
	}

	if (staffResponse.ok) {
		const session: SessionInfo = await staffResponse.json();
		const landing = decideLanding(session);
		if (landing.type === 'redirect') {
			redirect(303, resolve('/practices/[practiceId]', { practiceId: landing.practiceId }));
		} else if (landing.type === 'picker') {
			// `/` re-probes and renders the picker itself (#357) -- reusing
			// its own load rather than a second copy of the same list here.
			redirect(303, resolve('/'));
		}
	}

	redirect(303, resolve('/(signed-out)/no-practice'));
}
