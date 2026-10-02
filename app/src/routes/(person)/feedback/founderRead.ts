import { error, redirect } from '@sveltejs/kit';
import { resolve } from '$app/paths';
import { isMFARequired } from '#lib/api.js';
import { apiErrorMessage } from '#lib/apiErrorMessage.js';
import { staffLoginAfterSessionEnded } from '#lib/sessionEnded.js';

/**
 * What a founder read page's `load` does with a response that is not the
 * page it asked for (#1526). Returns only when the response is one to
 * read; every other case throws, the way `redirect` and `error` do.
 *
 * - 401: the session ended. To the login screen.
 * - 403 `MFA_REQUIRED`: this is the founder, on a session with no second
 *   factor. To TOTP enrollment, carrying `returnTo` so enrollment sends
 *   him back to the page he asked for -- the same hand-off
 *   `practices/[practiceId]/+layout.ts` makes.
 * - 404: anybody but the founder, or a piece that is gone. The root
 *   `+error.svelte` renders it, which is the page a URL with no route
 *   behind it gets, so the page's existence is not revealed either.
 *
 * `load` and not an onMount fetch, for #471's reason: a refusal has to
 * reach the error page rather than sit in a string this route owns.
 */
export async function refuseFounderRead(response: Response, returnTo: string): Promise<void> {
	if (response.status === 401) {
		redirect(303, staffLoginAfterSessionEnded());
	} else if (await isMFARequired(response)) {
		redirect(303, `${resolve('/(signed-out)/mfa/enroll')}?returnTo=${encodeURIComponent(returnTo)}`);
	} else if (!response.ok) {
		error(response.status, await apiErrorMessage(response));
	}
}
