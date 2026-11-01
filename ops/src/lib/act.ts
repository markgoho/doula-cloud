import type { FormError } from './authRefusal.ts';
import { refusalMessage, SERVICE_PROBLEM } from './session.ts';

/**
 * How a call to doula-ops ended: its answer, a refusal to show, a session
 * that has ended, or (for an act behind a fresh code) a step-up that is
 * needed again.
 */
export type Outcome<T> =
	| { kind: 'ok'; value: T }
	| { kind: 'refused'; errors: FormError[] }
	| { kind: 'signed-out' }
	| { kind: 'step-up' };

/**
 * A refused response as an Outcome, each detail beside the field fieldOf
 * names for it.
 */
export async function refusal<T>(response: Response, fieldOf: Record<string, string> = {}): Promise<Outcome<T>> {
	if (response.status === 401) return { kind: 'signed-out' };
	let body: { code?: unknown; message?: unknown; details?: unknown };
	try {
		body = await response.clone().json();
	} catch {
		return { kind: 'refused', errors: [{ message: SERVICE_PROBLEM }] };
	}
	if (response.status === 403 && body.code === 'SECOND_FACTOR_REQUIRED') return { kind: 'step-up' };
	// A third party's refusal: nothing changed, and doula-ops says so in words.
	if (response.status === 502 && typeof body.message === 'string') return { kind: 'refused', errors: [{ message: body.message }] };
	if (response.status < 500 && typeof body.details === 'object' && body.details !== null) {
		const errors = Object.entries(body.details as Record<string, string>).map(([field, message]) => ({
			message,
			targetId: fieldOf[field]
		}));
		return { kind: 'refused', errors };
	}
	return { kind: 'refused', errors: [{ message: await refusalMessage(response) }] };
}

/**
 * A new act's Idempotency-Key. A check page takes one per visit and sends
 * it with every press, so a resubmit after a lost answer replays the act.
 */
export function actKey(): string {
	return crypto.randomUUID();
}

/**
The one place an ops act is sent: a POST of body under key.
*/
export function postAct(path: string, key: string, body: unknown = {}, fetcher: typeof fetch = fetch): Promise<Response> {
	return fetcher(path, {
		method: 'POST',
		credentials: 'include',
		headers: { 'Content-Type': 'application/json', 'Idempotency-Key': key },
		body: JSON.stringify(body)
	});
}
