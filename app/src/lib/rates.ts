/**
 * A Practice's rate card (#966): the flat amount in cents it charges for
 * each Engagement kind. This module holds the load/save orchestration
 * and validation for the practice-settings screen, decoupled from
 * SvelteKit and the DOM so it can be unit-tested directly -- mirrors
 * contractTemplate.ts.
 */

import type { Fetcher } from './fetcher.js';

import { apiErrorMessage } from './apiErrorMessage.js';

/** An Engagement's kind, matching CONTEXT.md's Engagement entry -- "birth"
 * or "postpartum, what the Practice sold". */
export type EngagementKind = 'birth' | 'postpartum';

/** One kind's rate. amountCents is null when the Practice has not set a
 * rate for this kind yet -- a valid state (#966's AC), not an error. */
export interface Rate {
	kind: EngagementKind;
	amountCents: number | null;
}

export interface RatesResponse {
	rates: Rate[];
}

function ratesPath(practiceId: string): string {
	return `/api/practices/${practiceId}/rates`;
}

function ratePath(practiceId: string, kind: EngagementKind): string {
	return `/api/practices/${practiceId}/rates/${kind}`;
}

/** Loads a Practice's rate card -- always both kinds, whether or not each
 * has a rate set. Throws with the response body text on a non-2xx
 * response, mirroring contractTemplate.ts's loadContractTemplate. */
export async function loadRates(fetcher: Fetcher, practiceId: string): Promise<RatesResponse> {
	const response = await fetcher(ratesPath(practiceId));
	if (!response.ok) {
		throw new Error(await apiErrorMessage(response));
	}
	return response.json();
}

/** Sets or changes one kind's rate. Only an Owner or Admin's session
 * reaches this without a 403 (the Go BFF's own PutRateHandler enforces
 * it) -- this module makes no role check of its own, the same
 * "the server remains the authority" split contractTemplate.ts's
 * validateProse draws. */
export async function saveRate(
	fetcher: Fetcher,
	practiceId: string,
	kind: EngagementKind,
	amountCents: number
): Promise<Rate> {
	const response = await fetcher(ratePath(practiceId, kind), {
		method: 'PUT',
		headers: { 'Content-Type': 'application/json' },
		body: JSON.stringify({ amountCents })
	});
	if (!response.ok) {
		throw new Error(await apiErrorMessage(response));
	}
	return response.json();
}

/** Parses a dollars-and-cents string typed into a form field into the
 * integer cents PutRateHandler requires, or undefined if dollars is not a
 * finite, positive number -- mirrored client-side purely for early UX
 * feedback before a round trip; the server remains the authority. */
export function dollarsToCents(dollars: string): number | undefined {
	const value = Number(dollars);
	if (!Number.isFinite(value) || value <= 0) {
		return undefined;
	}
	return Math.round(value * 100);
}

/** Renders amountCents as a plain "12.00"-shaped string for a form
 * field's own value -- not formatMoney's locale-aware "$12.00", since a
 * form field re-parses what it displays and a currency symbol would have
 * to be stripped back out first. */
export function centsToDollars(amountCents: number): string {
	return (amountCents / 100).toFixed(2);
}
