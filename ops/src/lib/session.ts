/**
The signed-in operator, as doula-ops's GET /api/session names them.
*/
export type Operator = { name: string; email: string };

/**
 * How an ID token's exchange for a session ended: a session, a login with
 * no authenticator app yet (doula-ops's MFA_REQUIRED), or a refusal to
 * show as it is.
 */
export type Exchange =
	| { kind: 'signed-in'; operator: Operator }
	| { kind: 'enroll' }
	| { kind: 'refused'; message: string };

export const SERVICE_PROBLEM = 'Sorry, there is a problem with the service. Try again later.';

type Fetch = typeof fetch;

/**
doula-ops's refusal in its own words, or the service problem for a 5xx or a body that is not one.
*/
export async function refusalMessage(response: Response): Promise<string> {
	try {
		const body: { message?: unknown } = await response.json();
		return typeof body.message === 'string' && response.status < 500 ? body.message : SERVICE_PROBLEM;
	} catch {
		return SERVICE_PROBLEM;
	}
}

/**
Exchanges an operator's ID token for doula-ops's session cookie.
*/
export async function exchangeToken(idToken: string, fetcher: Fetch = fetch): Promise<Exchange> {
	const response = await fetcher('/api/session', {
		method: 'POST',
		credentials: 'include',
		headers: { Authorization: `Bearer ${idToken}` }
	});
	if (response.ok) return { kind: 'signed-in', operator: await response.json() };
	if (response.status === 403) {
		const body: { code?: string } = await response.clone().json();
		if (body.code === 'MFA_REQUIRED') return { kind: 'enroll' };
	}
	return { kind: 'refused', message: await refusalMessage(response) };
}

/**
The signed-in operator, or undefined once the session has ended.
*/
export async function readSession(fetcher: Fetch = fetch): Promise<Operator | undefined> {
	const response = await fetcher('/api/session', { credentials: 'include' });
	if (!response.ok) return undefined;
	return await response.json();
}

/**
Ends this browser's session. Ending one that has already ended is no failure.
*/
export async function endSession(fetcher: Fetch = fetch): Promise<void> {
	const response = await fetcher('/api/session', { method: 'DELETE', credentials: 'include' });
	if (!response.ok && response.status !== 401) throw new Error(`sign out: ${response.status}`);
}
