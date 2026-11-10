import type { FormError } from './authRefusal.ts';
import { refusalMessage, SERVICE_PROBLEM } from './session.ts';
import { STEP_UP_CODE_FIELD } from './stepUp.ts';

/**
 * What a route behind only a session answers: every read, and the acts that
 * ask no fresh code (a founding grant, a pilot allowlist address). doula-ops
 * answers 'step-up' only from `requireStepUp` (api/cmd/ops/stepup.go), so it
 * is not an arm here, and a caller has nothing to ignore.
 */
export type Outcome<T> =
	| { kind: 'ok'; value: T }
	| { kind: 'refused'; errors: FormError[] }
	| { kind: 'signed-out' };

/**
 * What an act behind `requireStepUp` answers: everything a session route
 * does, and 'step-up' when the code the operator just gave is spent or
 * too old to admit the act.
 */
export type StepUpOutcome<T> = Outcome<T> | { kind: 'step-up' };

/**
 * The words 'step-up' is reported in, beside the code field, the same
 * wherever an act asks for a fresh code.
 */
export const STEP_UP_AGAIN: FormError = {
	message: 'Enter the code your authenticator app shows now to do this.',
	targetId: STEP_UP_CODE_FIELD
};

/**
 * What a thrown read or act is reported in: a dropped connection or an
 * unreadable answer carries nothing an operator could act on. A new array
 * each time, so a second throw is still a change `ErrorSummary` re-focuses on.
 */
export function serviceProblem(): FormError[] {
	return [{ message: SERVICE_PROBLEM }];
}

export async function refusal(response: Response, fieldOf: Record<string, string> = {}): Promise<Outcome<never>> {
	if (response.status === 401) return { kind: 'signed-out' };
	let body: { message?: unknown; details?: unknown };
	try {
		body = await response.clone().json();
	} catch {
		return { kind: 'refused', errors: [{ message: SERVICE_PROBLEM }] };
	}
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

export async function stepUpRefusal(response: Response, fieldOf: Record<string, string> = {}): Promise<StepUpOutcome<never>> {
	if (response.status === 403) {
		let code: unknown;
		try {
			({ code } = await response.clone().json());
		} catch {
			// An unreadable 403 is not a step-up; `refusal` words it.
		}
		if (code === 'SECOND_FACTOR_REQUIRED') return { kind: 'step-up' };
	}
	return refusal(response, fieldOf);
}

// A check page takes one key per visit and sends it with every press, so a resubmit after a lost answer replays the act.
export function actKey(): string {
	return crypto.randomUUID();
}

export function postAct(path: string, key: string, body: unknown = {}, fetcher: typeof fetch = fetch): Promise<Response> {
	return fetcher(path, {
		method: 'POST',
		credentials: 'include',
		headers: { 'Content-Type': 'application/json', 'Idempotency-Key': key },
		body: JSON.stringify(body)
	});
}
