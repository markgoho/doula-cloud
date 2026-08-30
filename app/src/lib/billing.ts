/**
 * A Practice's billing credit balance and ledger history (#75). The
 * balance is never stored -- it is derived server-side by summing
 * credit_ledger rows (api/internal/billing/balance.go) -- so this module
 * only loads the read-only view; there is no save/mutate path here.
 */

export interface LedgerEntry {
	origin: string;
	quantity: number;
	createdAt: string;
}

/** One page of the ledger -- the cursor-pagination envelope from
 * docs/api-design.md section 4, mirroring billing.LedgerPage (#446). */
export interface LedgerPage {
	items: LedgerEntry[];
	nextCursor?: string;
	hasMore: boolean;
}

export interface Balance {
	balance: number;
	ledger: LedgerPage;
}

/** What each `credit_ledger` origin is called on the Billing screen.
 *
 * A founding grant and a signup bonus are two different things (#439, and
 * #449 gave them two enum values so the pilot terms can say "one-time"
 * about one and not the other), so the screen has to tell them apart
 * rather than printing whichever enum value arrived. It also stops
 * printing the enum at all: `signup_bonus` is an engineering word on a
 * screen a doula reads, which ADR-0022 already refused for "System".
 */
const ORIGIN_LABELS: Record<string, string> = {
	signup_bonus: 'Welcome credits',
	founding_grant: 'Founding member credits',
	purchase: 'Purchase',
	consumption: 'Engagement started',
	refund: 'Refund'
};

/** The label for a ledger row's origin. An origin with no label falls back
 * to the raw value: a new enum value should read oddly on the screen, not
 * leave the row blank. */
export function originLabel(origin: string): string {
	return ORIGIN_LABELS[origin] ?? origin;
}

/** A ledger quantity with its sign always shown -- a credit reads "+3", a
 * debit reads "-1" on its own, with nothing to compare it to in the same
 * cell (#509). */
export function formatSignedQuantity(quantity: number): string {
	return `${quantity > 0 ? '+' : ''}${quantity}`;
}

/** A minimal fetch-shaped function, injected rather than imported, so load
 * can be unit-tested without mocking the global fetch or SvelteKit's `$app`
 * modules -- mirrors planTemplate.ts's Fetcher. */
export type Fetcher = (path: string, init?: RequestInit) => Promise<Response>;

/** Exported for `billing/+page.ts` (#471), which needs the response's raw
 * status to distinguish a role refusal (403) from any other failure --
 * `loadBalance` below discards it. */
export function billingPath(practiceId: string): string {
	return `/api/practices/${practiceId}/billing`;
}

/** Loads a Practice's current balance and ledger history. Throws with the
 * response body text on a non-2xx response, mirroring loadTemplate's
 * error-surfacing convention. */
export async function loadBalance(fetcher: Fetcher, practiceId: string): Promise<Balance> {
	const response = await fetcher(billingPath(practiceId));
	if (!response.ok) {
		throw new Error(await response.text());
	}
	return response.json();
}

/** Starts a credit purchase for `quantity` credits and returns the
 * Stripe-hosted Checkout URL the caller's browser must navigate to (#110).
 * Throws with the response body text on a non-2xx response -- e.g. a
 * non-Owner attempting the request, which the backend's `RequireOwner`
 * rejects. */
export async function purchaseCredits(
	fetcher: Fetcher,
	practiceId: string,
	quantity: number
): Promise<string> {
	const response = await fetcher(`${billingPath(practiceId)}/purchases`, {
		method: 'POST',
		headers: { 'Content-Type': 'application/json' },
		body: JSON.stringify({ quantity })
	});
	if (!response.ok) {
		throw new Error(await response.text());
	}
	const body: { checkoutUrl: string } = await response.json();
	return body.checkoutUrl;
}
