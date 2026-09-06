import { goto } from '$app/navigation';
import { resolve } from '$app/paths';
import { signOut } from 'firebase/auth';
import { getFirebaseAuth } from './firebase.js';

/**
The Go BFF's origin. Set by Playwright/dev; a real deploy serves both from the same origin.
*/
export function apiBaseURL(): string {
	return (import.meta.env.VITE_API_BASE_URL as string | undefined) ?? '';
}

/**
Fetches an API path with the browser's session cookie. Every feature
call site uses this -- the sign-in/signup/accept-invite bootstrap
exchanges are the only places left that send an ID token, and they do
so with a plain `fetch` of their own, since that token only ever makes
one trip (to mint the cookie) rather than being a credential feature
code carries around. On a 401 (no session, expired, or revoked) this
clears the signed-in Identity Platform user, if any, and sends the
browser to the login screen for whichever population the current route
belongs to, carrying `sessionEnded=true` so that screen can read this as
"your session ended" rather than an ordinary visit.

A refusal carrying `{code: "MFA_REQUIRED"}` (#606) is a live, valid
session that may not enter *this* Practice -- not an ended session -- so
it routes to the enrolment screen instead, carrying `returnTo` so
enrolment can send her back to the Practice she was trying to reach. The
response body is cloned before this peeks at it, so a caller that goes
on to read the same response (`apiErrorMessage`, say) still can.
*/
// The practice landing page makes several of these calls on one
// navigation (load(), loadActivity(), loadWaitingOnReply(), each awaited
// in turn, and loadPracticeLanding's own internal Promise.all fan-out),
// sequentially, not just concurrently -- so `isRedirecting` alone
// (dedupes two calls in flight at once) is not enough: a later call's
// refusal, discovered only after an earlier one's own goto already
// landed on /mfa/enroll, must also check it isn't already there before
// reading location.pathname, or it clobbers returnTo with
// that path instead of falling through. A property on an object, not a
// bare module-level variable, so setting it stays an assignment to that
// object rather than a reassignment of the binding itself.
const redirectGuard = { isRedirecting: false };

export async function apiFetchWithSession(path: string, init: RequestInit = {}): Promise<Response> {
	const response = await apiFetch(path, init);
	if (redirectGuard.isRedirecting) return response;
	if (response.status === 401) {
		redirectGuard.isRedirecting = true;
		await handleExpiredSession();
		redirectGuard.isRedirecting = false;
	} else if (await isMFARequired(response)) {
		if (redirectGuard.isRedirecting || location.pathname === resolve('/mfa/enroll')) return response;
		redirectGuard.isRedirecting = true;
		await goto(`${resolve('/mfa/enroll')}?returnTo=${encodeURIComponent(location.pathname)}`);
		redirectGuard.isRedirecting = false;
	}
	return response;
}

/**
Reports whether response is #606's MFA-required refusal -- a 403
carrying the machine-readable `MFA_REQUIRED` code, distinct from every
other 403 (a role check, a missing membership) that must NOT redirect
anywhere. Reads a clone so the original response body is still there for
whatever the caller does with it afterward.
*/
async function isMFARequired(response: Response): Promise<boolean> {
	if (response.status !== 403) return false;
	try {
		const body: unknown = await response.clone().json();
		return (
			body !== null &&
			typeof body === 'object' &&
			'code' in body &&
			body.code === 'MFA_REQUIRED'
		);
	} catch {
		return false;
	}
}

/**
The same credentialed fetch with the 401 handling left off, for the two
calls that have to swallow their own failures instead of being sent to
the login screen: registering this device for push (#61) and
unregistering it on sign-out (#152 for Staff, #153 for the Client
portal). Both are best-effort, and the unregister runs against a session
sign-out is about to end anyway --
routing either through apiFetchWithSession would navigate the page away
mid-flight. Feature code wants apiFetchWithSession, not this.
*/
export function apiFetch(path: string, init: RequestInit = {}): Promise<Response> {
	return fetch(apiBaseURL() + path, { ...init, credentials: 'include' });
}

async function handleExpiredSession(): Promise<void> {
	await signOut(getFirebaseAuth());
	// Resolved separately per branch rather than from a union: `resolve` is
	// overloaded per route, and a union argument stops matching any single
	// overload.
	await goto(
		location.pathname.startsWith('/portal')
			? `${resolve('/portal/(signed-out)/login')}?sessionEnded=true`
			: `${resolve('/(signed-out)/login')}?sessionEnded=true`
	);
}

/**
Reads a session endpoint (`/api/staff/session` or `/api/portal/session`)
without treating an absent session as a failure. `apiFetchWithSession`
is the wrong tool for this: it treats every non-OK response as an
expired session and sends the browser to a login screen. The callers of
`probeSession` -- `/` and both login screens themselves -- ask this
question before any sign-in has happened, so "no session of this kind"
is the ordinary case here, not a failure, and reads as `undefined`
rather than a redirect. A thrown fetch (offline, the wrong host, a
non-JSON body from a rewrite miss) reads the same way, so the caller can
treat every one of these outcomes as "can't tell, proceed as signed out".
*/
export async function probeSession<Session>(path: string): Promise<Session | undefined> {
	try {
		const response = await apiFetch(path);
		if (!response.ok) return undefined;
		return (await response.json()) as Session;
	} catch {
		return undefined;
	}
}

// Re-exported for this module's own callers; see apiErrorMessage.ts for
// why its definition lives there instead of here.
export { apiErrorMessage } from './apiErrorMessage.js';
