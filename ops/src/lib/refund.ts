import type { FormError } from './authRefusal.ts';
import { refusalMessage, SERVICE_PROBLEM } from './session.ts';

export const CREDITS_FIELD = 'credits';
export const REQUESTED_BY_FIELD = 'requested-by';
export const REQUESTED_ON_FIELD = 'requested-on-day';

export type RefundOwner = { staffId: string; name: string };
export type RefundQuote = { credits: number; amountCents: number; taxCents: number; maxCredits: number };
export type RefundCheck = { practiceName: string; owners: RefundOwner[]; quote: RefundQuote | null };
export type RefundResult = {
	practiceName: string;
	credits: number;
	amountCents: number;
	taxCents: number;
	requestedByName: string;
	requestedOn: string;
};

export type RefundAnswers = { credits: string; requestedBy: string; day: string; month: string; year: string };

/**
 * How a call to doula-ops ended: its answer, a refusal to show, a session
 * that has ended, or (for the act) a step-up that is needed again.
 */
export type Outcome<T> =
	| { kind: 'ok'; value: T }
	| { kind: 'refused'; errors: FormError[] }
	| { kind: 'signed-out' }
	| { kind: 'step-up' };

type Fetch = typeof fetch;

const FIELD_OF: Record<string, string> = {
	credits: CREDITS_FIELD,
	requestedBy: REQUESTED_BY_FIELD,
	requestedOn: REQUESTED_ON_FIELD
};

const dollars = new Intl.NumberFormat('en-US', { style: 'currency', currency: 'USD' });

/**
An amount in cents as a person reads it: $42.74.
*/
export function money(cents: number): string {
	return dollars.format(cents / 100);
}

/**
A YYYY-MM-DD date as a person reads it: 30 October 2026.
*/
export function longDate(isoDate: string): string {
	const [year, month, day] = isoDate.split('-').map(Number);
	return new Date(Date.UTC(year, month - 1, day)).toLocaleDateString('en-US', {
		timeZone: 'UTC',
		month: 'long',
		day: 'numeric',
		year: 'numeric'
	});
}

/**
The three date boxes as YYYY-MM-DD, or undefined when they are not one real date.
*/
export function requestedOn({ day, month, year }: Pick<RefundAnswers, 'day' | 'month' | 'year'>): string | undefined {
	if ([day, month, year].some((part) => !/^\d+$/.test(part.trim())) || year.trim().length !== 4) return undefined;
	const [d, m, y] = [day, month, year].map((part) => Number(part.trim()));
	const date = new Date(Date.UTC(y, m - 1, d));
	if (date.getUTCFullYear() !== y || date.getUTCMonth() !== m - 1 || date.getUTCDate() !== d) return undefined;
	return date.toISOString().slice(0, 10);
}

/**
What is missing from the answers before doula-ops is asked anything.
*/
export function answerErrors(answers: RefundAnswers): FormError[] {
	const errors: FormError[] = [];
	if (!/^\d+$/.test(answers.credits.trim()) || Number(answers.credits) < 1) {
		errors.push({ message: 'Enter the number of Credits to refund, 1 or more', field: CREDITS_FIELD });
	}
	if (answers.requestedBy === '') errors.push({ message: 'Choose the Owner who asked for the refund', field: REQUESTED_BY_FIELD });
	if (!requestedOn(answers)) errors.push({ message: 'Enter the date the request arrived', field: REQUESTED_ON_FIELD });
	return errors;
}

async function refusal<T>(response: Response): Promise<Outcome<T>> {
	if (response.status === 401) return { kind: 'signed-out' };
	let body: { code?: unknown; message?: unknown; details?: unknown };
	try {
		body = await response.clone().json();
	} catch {
		return { kind: 'refused', errors: [{ message: SERVICE_PROBLEM }] };
	}
	if (response.status === 403 && body.code === 'SECOND_FACTOR_REQUIRED') return { kind: 'step-up' };
	// Stripe's refusal: nothing changed, and doula-ops says so in words.
	if (response.status === 502 && typeof body.message === 'string') return { kind: 'refused', errors: [{ message: body.message }] };
	if (response.status < 500 && typeof body.details === 'object' && body.details !== null) {
		const errors = Object.entries(body.details as Record<string, string>).map(([field, message]) => ({
			message,
			field: FIELD_OF[field]
		}));
		return { kind: 'refused', errors };
	}
	return { kind: 'refused', errors: [{ message: await refusalMessage(response) }] };
}

/**
What the check page states: the Practice, its Owners, and what a refund of credits gives back.
*/
export async function readRefundCheck(practiceId: string, credits?: number, fetcher: Fetch = fetch): Promise<Outcome<RefundCheck>> {
	const query = credits === undefined ? '' : `?credits=${credits}`;
	const response = await fetcher(`/api/practices/${practiceId}/refund${query}`, { credentials: 'include' });
	if (response.ok) return { kind: 'ok', value: await response.json() };
	return refusal(response);
}

/**
 * Sends the refund under key, the name of this one request: a retry with
 * the same key is answered with the refund already made.
 */
export async function sendRefund(
	practiceId: string,
	order: { credits: number; requestedBy: string; requestedOn: string },
	key: string,
	fetcher: Fetch = fetch
): Promise<Outcome<RefundResult>> {
	const response = await fetcher(`/api/practices/${practiceId}/refunds`, {
		method: 'POST',
		credentials: 'include',
		headers: { 'Content-Type': 'application/json', 'Idempotency-Key': key },
		body: JSON.stringify(order)
	});
	if (response.ok) return { kind: 'ok', value: await response.json() };
	return refusal(response);
}
