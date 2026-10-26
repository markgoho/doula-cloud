/**
 * The client half of `docs/api-design.md` section 3 (#1924): every `POST`
 * this app sends carries an `Idempotency-Key`, so a retry after a dropped
 * connection replays the first result on the BFF instead of repeating the
 * act -- a second Message, a second payment record, a second Invitation.
 *
 * `apiFetch` (`#lib/api.js`) is the only caller of {@link keyRequest}, and
 * every feature call reaches the network through it, so no call site
 * writes a key of its own.
 *
 * ## One key per submit
 *
 * A call site cannot say "this is the same submit as last time", so the
 * seam reads it from what it sees: the same `POST`, to the same path, with
 * the same body, sent again while the last attempt has not settled. An
 * attempt settles on any response below 500 -- the BFF stored that result
 * against the key, or refused the request for a reason the next press will
 * meet again -- and the next submit then gets a new key. A dropped
 * connection (`fetch` threw) or a 5xx leaves the key in place, so "Try
 * again" sends the same one.
 *
 * ## A key the caller made
 *
 * A write queued on the device to send when the signal returns (#1933)
 * makes its key when she writes it, with {@link newIdempotencyKey}, stores
 * it with the write, and sets {@link IDEMPOTENCY_KEY_HEADER} itself on
 * every replay, across a reload. The seam sends a key it was given as it
 * is and keeps no note of it.
 */

export const IDEMPOTENCY_KEY_HEADER = 'Idempotency-Key';

/**
 * A new key, for a caller that stores its own (#1933).
 */
export function newIdempotencyKey(): string {
	return crypto.randomUUID();
}

/**
 * The request to send, and what to call with its response so the submit
 * can settle. A request whose `fetch` throws is never settled.
 */
export interface KeyedRequest {
	init: RequestInit;
	settle: (response: Response) => void;
}

/**
 * The key of each submit sent but not yet settled, by what the submit
 * sent. In memory only: a reload is a new page, and a write that must
 * outlive one stores its own key.
 */
const pending = new Map<string, string>();

const leaveAsIs = (init: RequestInit): KeyedRequest => ({ init, settle: () => {} });

export function keyRequest(path: string, init: RequestInit): KeyedRequest {
	if ((init.method ?? 'GET').toUpperCase() !== 'POST') return leaveAsIs(init);
	const headers = new Headers(init.headers);
	if (headers.has(IDEMPOTENCY_KEY_HEADER)) return leaveAsIs(init);

	const body = bodyFingerprint(init.body);
	if (body === undefined) {
		headers.set(IDEMPOTENCY_KEY_HEADER, newIdempotencyKey());
		return leaveAsIs({ ...init, headers });
	}

	const submit = `${path}\n${body}`;
	const key = pending.get(submit) ?? newIdempotencyKey();
	pending.set(submit, key);
	headers.set(IDEMPOTENCY_KEY_HEADER, key);
	return {
		init: { ...init, headers },
		settle: (response) => {
			if (response.status < 500) pending.delete(submit);
		}
	};
}

/**
 * What a body says, as a string two sends of the same submit agree on, or
 * `undefined` for a body this cannot read without consuming it (a Blob, a
 * stream) -- that one gets a fresh key every time rather than a guess. A
 * file in a form is read by its name, size, type and date, not its bytes.
 */
function bodyFingerprint(body: RequestInit['body']): string | undefined {
	if (body === undefined || body === null) return '';
	if (typeof body === 'string') return body;
	if (body instanceof URLSearchParams) return body.toString();
	if (body instanceof FormData) {
		// A loop rather than Iterator helpers, which Safari has only had
		// since 18.4.
		const fields: unknown[] = [];
		for (const [name, value] of body) {
			fields.push(typeof value === 'string' ? [name, value] : [name, value.name, value.size, value.type, value.lastModified]);
		}
		return JSON.stringify(fields);
	}
	return undefined;
}
