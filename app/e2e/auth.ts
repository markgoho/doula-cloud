import { expect, type APIRequestContext } from '@playwright/test';

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

	const setCookie = created
		.headersArray()
		.find((h) => h.name.toLowerCase() === 'set-cookie')?.value;
	const value = setCookie?.match(/__session=([^;]*)/)?.[1];
	expect(value, `create-session set no __session cookie: ${setCookie}`).toBeTruthy();

	return { Cookie: `__session=${value}` };
}
