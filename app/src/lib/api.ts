import { goto } from '$app/navigation';
import { resolve } from '$app/paths';
import { signOut } from 'firebase/auth';
import { getFirebaseAuth } from './firebase.js';
import { keyRequest } from './idempotencyKey.js';
import { portalLoginAfterSessionEnded, staffLoginAfterSessionEnded } from './sessionEnded.js';

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
belongs to, carrying the flag `#lib/sessionEnded.js` owns so that screen
can read this as "your session ended" rather than an ordinary visit.

A refusal carrying `{code: "MFA_REQUIRED"}` (#606) is a live, valid
session that may not enter *this* Practice -- not an ended session -- so
it routes to the enrollment screen instead, carrying `returnTo` so
enrollment can send her back to the Practice she was trying to reach. The
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
	await didFollowSessionRefusal(response);
	return response;
}

/**
What `apiFetchWithSession` does with a response it has fetched, for a
response fetched without it: a route's `load` reads through plain
`apiFetch`, because a `load` also runs on a hovered link (#2249), and the
screen follows the refusal once it is showing. Reports whether the browser
is being sent elsewhere, so the screen keeps its loading state instead of
drawing the refusal for a frame.
*/
export async function didFollowSessionRefusal(response: Response): Promise<boolean> {
	if (redirectGuard.isRedirecting) return true;
	if (response.status === 401) {
		redirectGuard.isRedirecting = true;
		await handleExpiredSession();
		redirectGuard.isRedirecting = false;
		return true;
	}
	if (await isMFARequired(response)) {
		if (redirectGuard.isRedirecting || location.pathname === resolve('/(signed-out)/mfa/enroll')) return false;
		redirectGuard.isRedirecting = true;
		await goto(`${resolve('/(signed-out)/mfa/enroll')}?returnTo=${encodeURIComponent(location.pathname)}`);
		redirectGuard.isRedirecting = false;
		return true;
	}
	if (await isTermsAgreementRequired(response)) {
		return didGoToAgreement();
	}
	return false;
}

/**
#1549: a 403 carrying `TERMS_AGREEMENT_REQUIRED` is an Owner whose
Practice waits on her agreement to a material change. She goes to the one
screen that asks, for the Practice in the current address. A refusal met
on that screen itself, or with no Practice in the address, stays where it
is.
*/
async function didGoToAgreement(): Promise<boolean> {
	const practiceId = /^\/practices\/([^/]+)/.exec(location.pathname)?.[1];
	if (practiceId === undefined) return false;
	const agreement = resolve('/practices/[practiceId]/agreement', { practiceId });
	if (redirectGuard.isRedirecting || location.pathname === agreement) return false;
	redirectGuard.isRedirecting = true;
	await goto(agreement);
	redirectGuard.isRedirecting = false;
	return true;
}

/**
The machine-readable code a refusal carries, or `undefined` when the
response is not a 403 or its body is not the BFF's JSON envelope. Reads a
clone, so the caller can still read the body itself.
*/
async function forbiddenCode(response: Response): Promise<unknown> {
	if (response.status !== 403) return undefined;
	try {
		const body: unknown = await response.clone().json();
		return body !== null && typeof body === 'object' && 'code' in body ? body.code : undefined;
	} catch {
		return undefined;
	}
}

/**
Reports whether response is #1549's refusal: an Owner whose Practice
waits on her agreement to a material change of the Terms of Service or
the Privacy Policy. Exported for `practices/[practiceId]/+layout.ts`, for
the reason `isMFARequired` is.
*/
export async function isTermsAgreementRequired(response: Response): Promise<boolean> {
	return (await forbiddenCode(response)) === 'TERMS_AGREEMENT_REQUIRED';
}

/**
Reports whether response is #606's MFA-required refusal -- a 403
carrying the machine-readable `MFA_REQUIRED` code, distinct from every
other 403 (a role check, a missing membership) that must NOT redirect
anywhere. Reads a clone so the original response body is still there for
whatever the caller does with it afterward. Exported for
`practices/[practiceId]/+layout.ts` (#835/#748): that `load` uses
`apiFetch`, not `apiFetchWithSession`, so it needs this same check
itself before treating a 403 as a stale Membership rather than a live
session barred from this one Practice.
*/
export async function isMFARequired(response: Response): Promise<boolean> {
	return (await forbiddenCode(response)) === 'MFA_REQUIRED';
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

Every `POST` through here carries an `Idempotency-Key` (#1924,
`#lib/idempotencyKey.js`): the same key when the same submit is sent again
after a dropped connection or a 5xx, a new one once a submit has settled.
A `fetch` that throws leaves its submit unsettled, which is the case the
key exists for.
*/
export async function apiFetch(path: string, init: RequestInit = {}): Promise<Response> {
	const keyed = keyRequest(path, init);
	const response = await fetch(apiBaseURL() + path, { ...keyed.init, credentials: 'include' });
	keyed.settle(response);
	return response;
}

async function handleExpiredSession(): Promise<void> {
	await signOut(getFirebaseAuth());
	// Which population's login screen, decided here; what the address says
	// about the session that ended, decided by `#lib/sessionEnded.js`
	// (#1131), which is the only place that flag is spelled.
	await goto(
		location.pathname.startsWith('/portal')
			? portalLoginAfterSessionEnded()
			: staffLoginAfterSessionEnded()
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
