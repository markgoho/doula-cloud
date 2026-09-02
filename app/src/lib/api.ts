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
*/
export async function apiFetchWithSession(path: string, init: RequestInit = {}): Promise<Response> {
	const response = await apiFetch(path, init);
	if (response.status === 401) {
		await handleExpiredSession();
	}
	return response;
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

/**
Reads a failed response's body as a human-readable error message. Most
BFF endpoints still write plain text; a growing few (starting with
portalinvite, docs/api-design.md section 7's first adopter) write
{code, message} JSON instead -- this reads either without the caller
needing to know which.
*/
export async function apiErrorMessage(response: Response): Promise<string> {
	const text = await response.text();
	try {
		const parsed: unknown = JSON.parse(text);
		if (
			parsed !== null &&
			typeof parsed === 'object' &&
			'message' in parsed &&
			typeof parsed.message === 'string'
		) {
			return parsed.message;
		}
	} catch {
		// Not JSON -- most endpoints still write plain text, fall through.
	}
	return text;
}
