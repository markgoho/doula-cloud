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

export interface Balance {
	balance: number;
	ledger: LedgerEntry[];
}

/** A minimal fetch-shaped function, injected rather than imported, so load
 * can be unit-tested without mocking the global fetch or SvelteKit's `$app`
 * modules -- mirrors planTemplate.ts's Fetcher. */
export type Fetcher = (path: string, init?: RequestInit) => Promise<Response>;

function billingPath(practiceId: string): string {
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
