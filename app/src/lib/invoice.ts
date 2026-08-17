/**
 * An Invoice billed against an Engagement's Contract, created via Stripe's
 * Invoicing API on behalf of the Practice's connected account (#79/#81).
 * Stripe hosts the payment page and emails the Client -- this module only
 * loads/creates the Invoice record Doula Cloud keeps, decoupled from
 * SvelteKit and the DOM so it can be unit-tested directly -- mirrors
 * contract.ts.
 */

export interface Invoice {
	id: string;
	contractId: string;
	status: string;
	amountCents: number;
	currency: string;
	createdAt: string;
	paidAt?: string;
}

/** The body of a POST invoices response -- either the created Invoice, or
 * a state to route the Owner into the #79 connect flow (isOwner true) or
 * show a non-Owner the static "ask an Owner" message (isOwner false),
 * per #78's lazy-connect-prompt rule. Mirrors the Go BFF's
 * PostInvoiceResponse (api/internal/payments/invoice.go). */
export interface CreateInvoiceResult {
	connectRequired: boolean;
	isOwner?: boolean;
	invoice?: Invoice;
}

/** A minimal fetch-shaped function, injected rather than imported -- see
 * contract.ts's Fetcher for why. */
export type Fetcher = (path: string, init?: RequestInit) => Promise<Response>;

function invoicesPath(practiceId: string, engagementId: string): string {
	return `/api/practices/${practiceId}/engagements/${engagementId}/contract/invoices`;
}

/** Loads every Invoice billed against engagementId's Contract(s), newest
 * first -- only the first page (GetInvoicesHandler's cursor pagination is
 * for a scale this UI doesn't need to reach yet). Unlike loadContract,
 * GetInvoicesHandler never 404s for "no Contract yet" -- it returns 200
 * with an empty items list for that case (a Contract isn't required to
 * list, only to create), so a 404 here always means a real error
 * (engagement not found) and is thrown like any other non-2xx response,
 * with the response body text. */
export async function loadInvoices(fetcher: Fetcher, practiceId: string, engagementId: string): Promise<Invoice[]> {
	const response = await fetcher(invoicesPath(practiceId, engagementId));
	if (!response.ok) {
		throw new Error(await response.text());
	}
	const body: { items: Invoice[] } = await response.json();
	return body.items;
}

/** Creates an Invoice against engagementId's current Contract for
 * amountCents -- or, if the Practice hasn't connected Stripe yet, returns
 * the connectRequired gate state instead (no Invoice is created in that
 * case). Throws with the response body text on a non-2xx response (e.g.
 * no Contract exists yet, or an invalid amount). */
export async function createInvoice(
	fetcher: Fetcher,
	practiceId: string,
	engagementId: string,
	amountCents: number
): Promise<CreateInvoiceResult> {
	const response = await fetcher(invoicesPath(practiceId, engagementId), {
		method: 'POST',
		headers: { 'Content-Type': 'application/json' },
		body: JSON.stringify({ amountCents })
	});
	if (!response.ok) {
		throw new Error(await response.text());
	}
	return response.json();
}

/** Formats amountCents as a USD currency string (e.g. "$150.00") for
 * display -- the only place cents-to-dollars conversion happens on read;
 * write-side conversion (dollars the Staff typed -> cents the BFF stores)
 * lives in InvoiceSection.svelte, next to the input it converts. */
export function formatAmount(amountCents: number): string {
	return (amountCents / 100).toLocaleString('en-US', { style: 'currency', currency: 'USD' });
}
