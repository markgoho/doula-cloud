/**
 * Sending a piece of Feedback (#1519, #1523): the shape both the Staff
 * and the Portal send share, and the one BFF call this ticket (#1527)
 * wires up. Kept apart from `organisms/FeedbackForm.svelte` the same way
 * `invoice.ts` sits apart from `InvoiceSection.svelte` -- a domain module
 * an organism calls into, unit-tested with an injected `Fetcher` rather
 * than a mocked `fetch` or `$app` module.
 *
 * `Kind`'s three values are `api/internal/feedback.Kind*`'s own string
 * constants (00118's `feedback_kind` enum labels), so a validated value
 * posts with no translation step on either side.
 */
import type { Fetcher } from './fetcher.js';
import { refusalError } from './formErrors.js';

export const KIND_NOT_WORKING = 'not_working';
export const KIND_IDEA_OR_REQUEST = 'idea_or_request';
export const KIND_SOMETHING_ELSE = 'something_else';

export type Kind = typeof KIND_NOT_WORKING | typeof KIND_IDEA_OR_REQUEST | typeof KIND_SOMETHING_ELSE;

/**
 * The three radio choices (#1498 Q5), in the order the founder reviewed
 * them. One list rather than one per shell: the wording is identical for
 * Staff and Portal (#1502 Q4's own table), only the legend around them
 * differs, and that lives in copy the host passes to `FeedbackForm`.
 */
export const kindOptions: readonly { value: Kind; label: string }[] = [
	{ value: KIND_NOT_WORKING, label: 'Something is not working' },
	{ value: KIND_IDEA_OR_REQUEST, label: 'An idea or a request' },
	{ value: KIND_SOMETHING_ELSE, label: 'Something else' }
];

/**
 * The screen a send happened from, named the way `api/internal/feedback.
 * Page` reads it off the wire -- `FeedbackForm` builds this straight from
 * `$app/state`, no reshaping either side of the request.
 */
export interface FeedbackPage {
	url: string;
	route: { id: string };
}

/**
 * The body every send request shares (`api/internal/feedback.
 * ClientInput`) -- kind, the free text, and the context #1498 Q4 says the
 * form shows the sender it will attach before it ever posts.
 */
export interface ClientInput {
	kind: Kind;
	text: string;
	page: FeedbackPage;
	appBuild: string;
	screenWidth: number;
}

/**
 * The Staff DTO (`api/internal/staffauth.FeedbackRequest`): `ClientInput`
 * plus the Practice the screen under the drawer belongs to, when it
 * belongs to one -- `/account` sends none, and the BFF never trusts this
 * for anything beyond "which membership to check roles against".
 */
export interface StaffFeedbackInput extends ClientInput {
	practiceId?: string;
}

/**
 * Sends a piece of Feedback from a Staff screen (#1523). Refused (400,
 * 403) or unreachable (5xx) alike throw a `RefusalError` --
 * `FeedbackForm`'s own catch reads it the same way `RefundPaymentForm`'s
 * `confirm()` does, and keeps the drafted text in the drawer either way.
 */
export async function sendStaffFeedback(fetcher: Fetcher, input: StaffFeedbackInput): Promise<void> {
	const response = await fetcher('/api/staff/feedback', {
		method: 'POST',
		headers: { 'Content-Type': 'application/json' },
		body: JSON.stringify(input)
	});
	if (!response.ok) {
		throw await refusalError(response);
	}
}

/**
 * The Portal DTO (`api/internal/clientauth.PortalFeedbackRequest`,
 * #1523): `ClientInput` plus the Engagement the screen under the drawer
 * sits inside, when it sits inside one -- a screen with no single
 * Engagement (#1528's own AC) sends none, and the BFF checks a given one
 * against the caller's own Portal Account rather than trusting it.
 */
export interface PortalFeedbackInput extends ClientInput {
	engagementId?: string;
}

/**
 * Sends a piece of Feedback from a Portal screen (#1523, #1528). Same
 * refusal shape as `sendStaffFeedback`.
 */
export async function sendPortalFeedback(fetcher: Fetcher, input: PortalFeedbackInput): Promise<void> {
	const response = await fetcher('/api/portal/feedback', {
		method: 'POST',
		headers: { 'Content-Type': 'application/json' },
		body: JSON.stringify(input)
	});
	if (!response.ok) {
		throw await refusalError(response);
	}
}

/**
 * The browser family shown in the Feedback disclosure (#1498 Q4) --
 * User-Agent sniffing is otherwise unheard of in this app, and it exists
 * here only because the founder reads it as a plain fact on the private
 * GitHub issue (#1501 Q1), not to branch behavior on. Checked in the same
 * order `api/internal/feedback/browser.go` (#1523) checks the server
 * side of the same string, so the two name a browser the same way: Edge
 * and Chrome both carry "Chrome/" in their UA, so Edge's own "Edg/" token
 * has to be read first.
 */
export function browserName(userAgent: string): string {
	if (userAgent.includes('Edg/')) return 'Edge';
	if (userAgent.includes('Firefox/')) return 'Firefox';
	if (userAgent.includes('Chrome/')) return 'Chrome';
	if (userAgent.includes('Safari/')) return 'Safari';
	return 'Unknown browser';
}
