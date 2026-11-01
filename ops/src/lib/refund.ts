import { postAct, refusal, type Outcome } from './act.ts';
import type { FormError } from './authRefusal.ts';

export const CREDITS_FIELD = 'credits';
export const REQUESTED_BY_FIELD = 'requested-by';
export const REQUESTED_ON_FIELD = 'requested-on';
export const REQUESTED_ON_TARGET = `${REQUESTED_ON_FIELD}-month`;

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

type Fetch = typeof fetch;

const FIELD_OF: Record<string, string> = {
	credits: CREDITS_FIELD,
	requestedBy: REQUESTED_BY_FIELD,
	requestedOn: REQUESTED_ON_TARGET
};

export function requestedOn({ day, month, year }: Pick<RefundAnswers, 'day' | 'month' | 'year'>): string | undefined {
	if ([day, month, year].some((part) => !/^\d+$/.test(part.trim())) || year.trim().length !== 4) return undefined;
	const [d, m, y] = [day, month, year].map((part) => Number(part.trim()));
	const date = new Date(Date.UTC(y, m - 1, d));
	if (date.getUTCFullYear() !== y || date.getUTCMonth() !== m - 1 || date.getUTCDate() !== d) return undefined;
	return date.toISOString().slice(0, 10);
}

export function answerErrors(answers: RefundAnswers): FormError[] {
	const errors: FormError[] = [];
	if (!/^\d+$/.test(answers.credits.trim()) || Number(answers.credits) < 1) {
		errors.push({ message: 'Enter the number of Credits to refund, 1 or more', targetId: CREDITS_FIELD });
	}
	if (answers.requestedBy === '') errors.push({ message: 'Choose the Owner who asked for the refund', targetId: REQUESTED_BY_FIELD });
	if (!requestedOn(answers)) errors.push({ message: 'Enter the date the request arrived', targetId: REQUESTED_ON_TARGET });
	return errors;
}

export async function readRefundCheck(practiceId: string, credits?: number, fetcher: Fetch = fetch): Promise<Outcome<RefundCheck>> {
	const query = credits === undefined ? '' : `?credits=${credits}`;
	const response = await fetcher(`/api/practices/${practiceId}/refund${query}`, { credentials: 'include' });
	if (response.ok) return { kind: 'ok', value: await response.json() };
	return refusal(response, FIELD_OF);
}

export async function sendRefund(
	practiceId: string,
	order: { credits: number; requestedBy: string; requestedOn: string },
	key: string,
	fetcher: Fetch = fetch
): Promise<Outcome<RefundResult>> {
	const response = await postAct(`/api/practices/${practiceId}/refunds`, key, order, fetcher);
	if (response.ok) return { kind: 'ok', value: await response.json() };
	return refusal(response, FIELD_OF);
}
