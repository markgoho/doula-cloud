import { apiErrorMessage, apiFetchWithSession } from '#lib/api.js';
import type { SessionInfo } from '#lib/landing.js';

export type AccountSessionResult = { ok: true; session: SessionInfo } | { ok: false; message: string };

/*
 * account/+layout.svelte and account/+page.svelte both need this endpoint --
 * the layout for the nav's memberships, the page for the form's
 * name/workState/reportedAt -- and they mount together on every visit to
 * /account. Memoizing the one in-flight request means whichever mounts
 * first pays for it and the other reuses the same result, rather than the
 * two of them racing the same GET (#474).
 */
let inFlight = $state<Promise<AccountSessionResult> | undefined>(undefined);

export function loadAccountSession(): Promise<AccountSessionResult> {
	if (!inFlight) {
		inFlight = (async () => {
			const response = await apiFetchWithSession('/api/staff/session');
			if (!response.ok) return { ok: false, message: await apiErrorMessage(response) };
			return { ok: true, session: (await response.json()) as SessionInfo };
		})();
	}
	return inFlight;
}

// Test-only: `inFlight` is module-level state, so a spec file with more
// than one render() needs a clean slate rather than replaying whatever the
// first test's fetch resolved to.
export function resetAccountSession(): void {
	inFlight = undefined;
}
