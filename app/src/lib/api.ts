/** The Go BFF's origin. Set by Playwright/dev; a real deploy serves both from the same origin. */
export function apiBaseURL(): string {
	return (import.meta.env.VITE_API_BASE_URL as string | undefined) ?? '';
}

/** Fetches an API path with the caller's Identity Platform ID token attached. */
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
