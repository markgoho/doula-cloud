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
	// #909: required, unlike `pendingDeletion` below -- that flag is
	// conditional, but every session has a caller, so a route may read
	// this one without a fallback. The Add-a-Visit picker reads it to
	// find which roster entry is the signed-in person.
	staffId: string;
	practiceName: string;
	roles: string[];
	isContractor: boolean;
	// #871: optional, not carried by every route fixture's `pageData.session`
	// -- true only while this Practice is between initiate and finalize.
	// settings/delete reads it to tell a non-Owner why she landed here,
	// rather than the generic "Only a Practice Owner..." notice every
	// other Owner-only setting shows.
	pendingDeletion?: boolean;
}

// The one route the deletion lockout leaves open regardless of role
// (staffauth.Middleware's isPracticeSessionRoute) -- this load redirects
// straight there once pendingDeletion comes back true, so an Owner lands
// on the restore screen and every other role lands on that same screen's
// own "Only a Practice Owner..." notice, instead of every other route
// under this Practice refusing one 403 at a time.
const deleteSettingsPath = (practiceId: string) =>
	resolve('/practices/[practiceId]/settings/delete', { practiceId });

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
 * #871: `pendingDeletion` on a successful response is a live Membership
 * at a Practice mid-deletion, not a refusal at all -- `.../session` is
 * the one route `staffauth.Middleware` leaves open to every role while
 * a Practice is locked, precisely so this `load` can read the flag and
 * send everyone to `settings/delete`: an Owner to the restore screen,
 * everyone else to that same screen's own locked notice (`pendingDeletion`
 * rides along on the returned session for exactly that), rather than
 * each of this Practice's other routes refusing her one 403 at a time.
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

	const body: {
		staffId: string;
		practiceName: string;
		roles: string[];
		isContractor: boolean;
		pendingDeletion: boolean;
	} = await response.json();

	if (body.pendingDeletion && url.pathname !== deleteSettingsPath(params.practiceId)) {
		redirect(303, deleteSettingsPath(params.practiceId));
	}

	return {
		session: {
			practiceId: params.practiceId,
			staffId: body.staffId,
			practiceName: body.practiceName,
			roles: body.roles,
			isContractor: body.isContractor,
			pendingDeletion: body.pendingDeletion
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
			redirect(303, resolve('/(signed-out)'));
		}
	}

	redirect(303, resolve('/(signed-out)/no-practice'));
}
