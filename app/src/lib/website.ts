/**
 * The website a Practice declares to Stripe (#440): a URL of her own, or
 * a page Doula Cloud publishes for her at `doula.cloud/p/<slug>`.
 *
 * Stripe's hosted onboarding demands a website from every connected
 * account, and #421 walked what happens when it does not get one -- the
 * field accepts empty, she finishes every remaining step, submits, and
 * comes back "done" with no way to take payments and nothing on screen
 * saying why. See api/internal/website for the server's own reasoning.
 */

import { apiErrorMessage } from './api.js';

/** `undeclared` is a Practice that has not answered yet. It is a shape
 * the API reports, never one the app sends. */
export type WebsiteMode = 'undeclared' | 'own' | 'hosted';

export interface PracticeWebsite {
	mode: WebsiteMode;
	/**
	Her own website or social profile, normalized by the server.
	*/
	ownUrl: string;
	/**
	What she offers, in her words -- one of the two facts only she has.
	*/
	serviceDescription: string;
	/**
	Her cancellation or refund position -- the other one.
	*/
	cancellationPolicy: string;
	/**
	The name of whoever last wrote the answer, or empty if nobody has.
	*/
	updatedBy: string;
	/**
	RFC 3339, or empty if nobody has answered.
	*/
	updatedAt: string;
	/**
	Whether the page we publish for her has been confirmed to load
	(#443). Empty for a Practice on her own website, who has no page
	here.
	*/
	pageState: PageState;
	/**
	When a probe last looked, RFC 3339, or empty if none has.
	*/
	pageCheckedAt: string;
	/**
	Why the last probe failed, in a few words for her to read. Empty
	when it did not fail.
	*/
	pageCheckDetail: string;
	/**
	The public address of the page published for her, or empty when she
	is on her own website. The same URL Stripe is handed, so showing it
	to her shows her what Stripe will look at.
	*/
	pageUrl: string;
}

/**
 * Whether her published page is actually there (#443).
 *
 * `pending` is the ordinary couple of minutes between publishing and the
 * deploy finishing, and it is also where a page stays when the build
 * failed and nothing ever reported back -- so it reads as "not confirmed
 * yet" rather than as a problem, and never as a success. Only a probe
 * that fetched the page says `live`.
 *
 * Empty string is a Practice on her own website: there is no page of
 * ours to be in any state at all.
 */
export type PageState = '' | 'pending' | 'live' | 'failed';

/**
What the app may send: the two real answers, never `undeclared`.
*/
export interface WebsiteDeclaration {
	mode: 'own' | 'hosted';
	ownUrl?: string;
	serviceDescription?: string;
	cancellationPolicy?: string;
}

/** The character budget on each of the two facts. The same number
 * api/internal/website.MaxFactLength enforces and 00045's CHECK
 * constraints make impossible to exceed -- this copy is what the screen
 * counts down against, not what holds the line. */
export const MAX_FACT_LENGTH = 500;

/** A 400 from the website endpoint, carrying docs/api-design.md section
 * 7's field-level `details` so the screen can put each message beside the
 * input it is about rather than in one heap at the top. */
export class WebsiteValidationError extends Error {
	readonly fieldErrors: Record<string, string>;

	constructor(message: string, fieldErrors: Record<string, string>) {
		super(message);
		this.name = 'WebsiteValidationError';
		this.fieldErrors = fieldErrors;
	}
}

/** A minimal fetch-shaped function, injected rather than imported, so
 * these can be unit-tested without mocking the global fetch or
 * SvelteKit's `$app` modules -- mirrors payments.ts's Fetcher. */
export type Fetcher = (path: string, init?: RequestInit) => Promise<Response>;

function websitePath(practiceId: string): string {
	return `/api/practices/${practiceId}/website`;
}

/** Reads what a Practice has declared. Never throws for a Practice that
 * has not answered -- that reads as mode `undeclared`. */
export async function loadWebsite(fetcher: Fetcher, practiceId: string): Promise<PracticeWebsite> {
	const response = await fetcher(websitePath(practiceId));
	if (!response.ok) {
		throw new Error(await apiErrorMessage(response));
	}
	return response.json();
}

/** Writes a Practice's declaration, whole. A PUT rather than a POST: one
 * answer per Practice, so re-sending the same one is safe.
 *
 * A 400 becomes a WebsiteValidationError carrying the field-level
 * details; anything else becomes a plain Error with whatever sentence the
 * server sent. */
export async function saveWebsite(
	fetcher: Fetcher,
	practiceId: string,
	declaration: WebsiteDeclaration
): Promise<PracticeWebsite> {
	const response = await fetcher(websitePath(practiceId), {
		method: 'PUT',
		headers: { 'Content-Type': 'application/json' },
		body: JSON.stringify(declaration)
	});
	if (response.ok) {
		return response.json();
	}

	if (response.status === 400) {
		// Read the body once, then decide: a Response body cannot be read
		// twice, and apiErrorMessage would consume it.
		const text = await response.text();
		let message = text;
		let fieldErrors: Record<string, string> = {};
		try {
			const parsed: unknown = JSON.parse(text);
			if (parsed !== null && typeof parsed === 'object') {
				const body = parsed as { message?: unknown; details?: unknown };
				if (typeof body.message === 'string') {
					message = body.message;
				}
				if (body.details !== null && typeof body.details === 'object') {
					fieldErrors = body.details as Record<string, string>;
				}
			}
		} catch {
			// Not JSON. The plain sentence is still the message.
		}
		throw new WebsiteValidationError(message, fieldErrors);
	}

	throw new Error(await apiErrorMessage(response));
}
