/**
 * A Practice's blocked email addresses (ADR-0029, #744).
 *
 * Doula Cloud stops sending to an address once mail to it hard-bounced,
 * or once the recipient marked the mail as spam. `loadEmailSuppressions`
 * reads the addresses one Practice is answerable for;
 * `clearEmailSuppression` lifts a bounce-caused block, which also deletes
 * it at Mailgun -- the local row alone would be a lie, since the vendor
 * refuses the address regardless of what this table says.
 *
 * A complaint-caused block is never lifted. The endpoint refuses one with
 * a 409 rather than trusting a screen to hide the action: every Practice
 * shares one sending domain (ADR-0011), so a second complaint spends
 * everyone's reputation.
 *
 * Decoupled from SvelteKit and the DOM (a `Fetcher` is injected, not
 * imported) so this can be unit-tested directly -- mirrors client.ts and
 * mfaRequirement.ts.
 */
import { apiErrorMessage } from './apiErrorMessage.js';

/** A minimal fetch-shaped function, injected rather than imported -- see
 * client.ts's Fetcher for why. */
export type Fetcher = (path: string, init?: RequestInit) => Promise<Response>;

/**
 * One blocked address -- mirrors the Go BFF's mailsuppress.suppressionDTO.
 */
export interface EmailSuppression {
	address: string;
	/** `'bounce'` or `'complaint'`. Carried raw rather than as a ready-made
	 * sentence because it is a fact about the block, not a label: the
	 * screen words it, and the BFF words `clearable` from it. */
	cause: string;
	/**
	 * RFC3339 -- when the block was recorded.
	 */
	createdAt: string;
	/** True only for a bounce. The screen offers no action without it, and
	 * the endpoint refuses one anyway. */
	clearable: boolean;
}

function suppressionsPath(practiceId: string): string {
	return `/api/practices/${practiceId}/email-suppressions`;
}

/**
 * Every address this Practice is answerable for that Doula Cloud has
 * stopped writing to. Owner or Admin only, server-side (ADR-0008 keeps
 * this in the same hands as the roster it is drawn from), so a refusal
 * reaches the caller as the BFF's own sentence.
 */
export async function loadEmailSuppressions(
	fetcher: Fetcher,
	practiceId: string
): Promise<EmailSuppression[]> {
	const response = await fetcher(suppressionsPath(practiceId));
	if (!response.ok) {
		throw new Error(await apiErrorMessage(response));
	}
	const body: { suppressions?: EmailSuppression[] } = await response.json();
	return body.suppressions ?? [];
}

/**
 * Lifts a bounce-caused block, at Mailgun as well as here.
 *
 * The address travels in the body rather than a path segment: a local
 * part may legitimately hold '+' and '/', which a path segment is the one
 * place callers reliably forget to escape.
 *
 * Throws the BFF's own sentence on a refusal -- read through
 * `apiErrorMessage` rather than `refusalMessage` on purpose. A failed
 * Mailgun call answers 502 with "nothing was changed", and that clause is
 * the one thing a person needs after a failed attempt; `refusalMessage`
 * would collapse every 5xx into one generic line and lose it.
 */
export async function clearEmailSuppression(
	fetcher: Fetcher,
	practiceId: string,
	address: string
): Promise<void> {
	const response = await fetcher(`${suppressionsPath(practiceId)}/clear`, {
		method: 'POST',
		headers: { 'Content-Type': 'application/json' },
		body: JSON.stringify({ address })
	});
	if (!response.ok) {
		throw new Error(await apiErrorMessage(response));
	}
}
