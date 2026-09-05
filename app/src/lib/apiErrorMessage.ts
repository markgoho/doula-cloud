/**
Reads a failed response's body as a human-readable error message. Every
BFF endpoint now writes docs/api-design.md section 7's {code, message,
details} JSON shape (api/internal/apierr, #529); this reads it without
the caller needing to decode JSON itself, and still falls back to the
raw text if a body ever isn't JSON.

Dependency-free on purpose: several lib modules (client.ts, contract.ts,
offer.ts, and others) deliberately avoid importing api.ts, since api.ts
pulls in SvelteKit's `$app` modules and firebase/auth and those modules
are unit-tested without either -- see their own "decoupled from
SvelteKit" doc comments. api.ts re-exports this for its own callers.
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
		// Not JSON -- fall back to the raw text.
	}
	return text;
}
