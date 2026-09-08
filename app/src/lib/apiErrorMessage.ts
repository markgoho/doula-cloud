import { parseRefusal } from './formErrors.js';

/**
Reads a failed response's body as a human-readable error message. Every
BFF endpoint now writes docs/api-design.md section 7's {code, message,
details} JSON shape (api/internal/apierr, #529); `parseRefusal`
(formErrors.ts, #840) reads it without the caller needing to decode JSON
itself, and still falls back to the raw text if a body ever isn't JSON.

Calls `parseRefusal` with `opaque5xx: false` -- unlike every reader in
formErrors.ts, this one shows a 5xx's own message where there is one
(see `clearEmailSuppression`'s doc comment for why one caller needs
that), so `parsed` here is never `undefined`: text is always read, and an
empty body projects to `''`, the same as before #840.

Dependency-free on purpose: several lib modules (contract.ts, offer.ts,
and others) deliberately avoid importing api.ts, since api.ts
pulls in SvelteKit's `$app` modules and firebase/auth and those modules
are unit-tested without either -- see their own "decoupled from
SvelteKit" doc comments. formErrors.ts carries no such import either (its
only import is a type), so importing from it costs nothing here. api.ts
re-exports this in turn, for its own callers.
*/
export async function apiErrorMessage(response: Response): Promise<string> {
	const parsed = await parseRefusal(response, { opaque5xx: false });
	return parsed.message ?? parsed.text;
}
