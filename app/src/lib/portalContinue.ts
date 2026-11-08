/**
 * What the two portal doors that spend an emailed token on a press share
 * (#610, #617, ADR-0026): the invitation's Continue and the sign-in link's
 * Continue. Each posts its token and, once the BFF has set the session
 * cookie, lands the Client the same way.
 */
import { goto } from '$app/navigation';
import { resolve } from '$app/paths';
import { apiBaseURL, apiFetchWithSession } from './api.js';
import { refusalErrors } from './formErrors.js';
import type { FormError } from './formSubmission.svelte.js';
import { decidePortalLanding, type PortalSessionInfo } from './portalLanding.js';

/**
 * The press that spends a token. `credentials: 'include'` because #610
 * reads the __session cookie off this request to decide whether continuing
 * evicts a Practice session, and a cross-origin request drops it without
 * this -- which would evict her with no warning at all. A no-op on the
 * same origin, which is what production serves. `isConfirmed` is the
 * second press, sent only once the BFF's warning has been read.
 */
export function postContinue(path: string, body: object, isConfirmed: boolean): Promise<Response> {
	return fetch(`${apiBaseURL()}${path}`, {
		method: 'POST',
		credentials: 'include',
		headers: {
			'Content-Type': 'application/json',
			...(isConfirmed && { 'X-Confirmed': 'true' })
		},
		body: JSON.stringify(body)
	});
}

/**
 * Where a Client lands once signed in: her one Engagement, or the app root
 * for more than one (ADR-0015's second Practice, #312) or none yet. A live
 * portal session here rules out a Staff session surviving to be misrouted
 * by `/`'s own probe order. Refused, it returns what to show.
 */
export async function landInPortal(): Promise<FormError[] | undefined> {
	const sessionResponse = await apiFetchWithSession('/api/portal/session');
	if (!sessionResponse.ok) {
		return await refusalErrors(sessionResponse);
	}
	const landing = decidePortalLanding((await sessionResponse.json()) as PortalSessionInfo);
	await goto(
		landing.type === 'redirect'
			? resolve('/portal/(authenticated)/engagements/[engagementId]', { engagementId: landing.engagementId })
			: resolve('/(signed-out)')
	);
}
