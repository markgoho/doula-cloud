import {
	expect,
	type APIRequestContext,
	type APIResponse,
	type BrowserContext,
	type Page
} from '@playwright/test';
import { PREVIEW_SERVER_ORIGIN } from './ports';
import { retryPastRateLimit } from './rateLimit';

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
	// retryPastRateLimit (rateLimit.ts) for why a repeated batch needs it
	// here too, a repeat or two after it needs it at signup (#1138).
	const created = await retryPastRateLimit(() =>
		request.post(`${apiURL}/api/session`, {
			headers: { Authorization: `Bearer ${idToken}` }
		})
	);
	expect(
		created.ok(),
		`create-session failed: ${created.status()} ${await created.text()}`
	).toBe(true);

	return sessionCookieFrom(created, 'create-session');
}

/**
 * Puts headers' session cookie directly into context's cookie jar and
 * navigates page to practiceId's landing route -- the browser-level
 * equivalent of a /login walk, for a fixture that has no interest in
 * spending one (totpStub.ts is what a spec that does want the walk
 * installs). headers come from signIn above or from mfa.ts's
 * signInEnrolled alike: the cookie has the same shape whether or not the
 * session has a second factor (#1854). A direct cookie injection rather
 * than a second interactive sign-in on purpose: Playwright's
 * request-context `secure: false` sidesteps whether its own cookie jar
 * treats loopback as a trustworthy origin the way a real browser does
 * (see signIn's doc comment above on the same question) -- the server
 * never reads this attribute back off an inbound Cookie header, so it
 * costs nothing to relax here.
 */
export async function enterPractice(
	context: BrowserContext,
	page: Page,
	headers: { Cookie: string },
	practiceId: string
): Promise<void> {
	const token = headers.Cookie.replace('__session=', '');
	await context.addCookies([
		{ name: '__session', value: token, url: PREVIEW_SERVER_ORIGIN, httpOnly: true, secure: false, sameSite: 'Lax' }
	]);
	await page.goto(`/practices/${practiceId}`);
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
