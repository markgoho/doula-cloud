import { expect, type APIRequestContext, type APIResponse } from '@playwright/test';

// Since #151 the BFF reads a Bearer ID token on three bootstrap endpoints
// only -- Staff signup, the two invitation-acceptance endpoints -- plus
// create-session. Every other route reads the __session cookie. A spec
// that provisions its fixtures through the API therefore has to exchange
// its ID token for a session first, exactly as the browser does.

/**
 * Exchanges an Identity Platform ID token for a session, and returns the
 * headers a later API call must send to be authenticated.
 *
 * The Cookie header is built by hand rather than left to the request
 * context's own cookie jar: the session cookie is Secure, and the e2e
 * stack serves the BFF over plain http on loopback, so relying on the jar
 * would make these fixtures depend on how a given Playwright version
 * treats loopback as a trustworthy origin.
 */
export async function signIn(
	request: APIRequestContext,
	apiURL: string,
	idToken: string
): Promise<{ Cookie: string }> {
	const created = await request.post(`${apiURL}/api/session`, {
		headers: { Authorization: `Bearer ${idToken}` }
	});
	expect(
		created.ok(),
		`create-session failed: ${created.status()} ${await created.text()}`
	).toBe(true);

	return sessionCookieFrom(created, 'create-session');
}

/**
 * Pulls the `__session` cookie off a response that set one directly --
 * create-session's own response above, and AcceptInviteHandler's (#525,
 * #150), which mints the session in the same response as the acceptance
 * rather than making a caller sign in as a second step.
 */
export function sessionCookieFrom(response: APIResponse, context: string): { Cookie: string } {
	const setCookie = response
		.headersArray()
		.find((h) => h.name.toLowerCase() === 'set-cookie')?.value;
	const value = setCookie?.match(/__session=([^;]*)/)?.[1];
	expect(value, `${context} set no __session cookie: ${setCookie}`).toBeTruthy();

	return { Cookie: `__session=${value}` };
}
