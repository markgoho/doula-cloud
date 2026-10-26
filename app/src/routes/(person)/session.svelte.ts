import { apiFetchWithSession } from '#lib/api.js';
import { apiErrorMessage } from '#lib/apiErrorMessage.js';
import type { SessionInfo } from '#lib/landing.js';

export type AccountSessionResult = { ok: true; session: SessionInfo } | { ok: false; message: string };

/*
 * (person)/+layout.svelte and the account pages both need this endpoint --
 * the layout for the bar's name, the pages for the summary and the
 * question pages' answers -- and they mount together on every visit to
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

/*
 * Forgets the memoized read, so the next caller fetches again. A save on
 * one of Your account's question pages (#2044) calls it before it goes
 * back: `inFlight` outlives an SPA navigation, so without it the summary
 * would show the name she has just changed. The layout reads the memo in
 * an effect, so the bar's name follows the same new read.
 *
 * A spec file with more than one render() calls it too, for a clean slate
 * rather than whatever the first test's fetch resolved to.
 */
export function resetAccountSession(): void {
	inFlight = undefined;
}
