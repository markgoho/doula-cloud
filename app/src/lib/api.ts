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
	const response = await fetch(apiBaseURL() + path, { ...init, credentials: 'include' });
	if (response.status === 401) {
		await handleExpiredSession();
	}
	return response;
}

async function handleExpiredSession(): Promise<void> {
	await signOut(getFirebaseAuth());
	const loginPath = resolve(location.pathname.startsWith('/portal') ? '/portal/login' : '/login');
	await goto(`${loginPath}?sessionEnded=true`);
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
