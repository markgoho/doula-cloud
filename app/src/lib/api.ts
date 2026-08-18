/**
The Go BFF's origin. Set by Playwright/dev; a real deploy serves both from the same origin.
*/
export function apiBaseURL(): string {
	return (import.meta.env.VITE_API_BASE_URL as string | undefined) ?? '';
}

/**
Fetches an API path with the caller's Identity Platform ID token attached.
*/
export async function apiFetch(
	path: string,
	idToken: string,
	init: RequestInit = {}
): Promise<Response> {
	return fetch(apiBaseURL() + path, {
		...init,
		headers: { ...init.headers, Authorization: `Bearer ${idToken}` }
	});
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
