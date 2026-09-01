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

/** One row of the Practice-wide Invoice list (#265) -- the same Invoice,
 * plus who it is for and the Engagement it is a way in to. It extends
 * `Invoice` rather than replacing it because the wire shape really is a
 * superset here; the BFF keeps its own PracticeInvoiceView a separate
 * struct only because Go has no such extension, and neither list should
 * be able to quietly grow the other's fields. `clientName` is her
 * preferred name, the one every screen uses. */
export interface PracticeInvoice extends Invoice {
	engagementId: string;
	clientName: string;
}

/** A page of the Practice-wide Invoice list, with the whole book's
 * totals alongside -- outstanding (billed and not yet collected) and
 * paid. The totals are of every Invoice at the Practice, never of the
 * page, so they do not change as the reader pages. Mirrors the Go BFF's
 * PracticeInvoicesResponse (api/internal/payments/practice_invoices.go). */
export interface PracticeInvoicePage {
	items: PracticeInvoice[];
	nextCursor?: string;
	hasMore: boolean;
	outstandingCents: number;
	outstandingCount: number;
	paidCents: number;
}

/** A minimal fetch-shaped function, injected rather than imported -- see
 * contract.ts's Fetcher for why. */
export type Fetcher = (path: string, init?: RequestInit) => Promise<Response>;

/** The Practice-wide Invoice list's path -- exported so the route's
 * `load` can call `apiFetch` on it directly and handle 401/403 the way
 * SvelteKit needs (see the billing route's `+page.ts` for why a
 * role-gated read loads there rather than in `onMount`). */
export function practiceInvoicesPath(practiceId: string, cursor?: string): string {
	const path = `/api/practices/${practiceId}/invoices`;
	return cursor ? `${path}?cursor=${encodeURIComponent(cursor)}` : path;
}

/** Loads one page of every Invoice the Practice has billed, newest
 * first. Throws with the response body text on a non-2xx response --
 * the route's `load` maps status codes to SvelteKit errors before
 * calling this, so a throw here is only ever an unexpected failure. */
export async function loadPracticeInvoices(
	fetcher: Fetcher,
	practiceId: string,
	cursor?: string
): Promise<PracticeInvoicePage> {
	const response = await fetcher(practiceInvoicesPath(practiceId, cursor));
	if (!response.ok) {
		throw new Error(await response.text());
	}
	return response.json();
}

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

/** The Stripe Invoice statuses the BFF stores verbatim, in the words a
 * person reads. Lives here rather than in InvoiceSection.svelte, where it
 * started, now that the Practice-wide list (#265) is a second consumer of
 * the same five words -- one Invoice must not be "Open" on one screen and
 * "Outstanding" on the next. An unknown status falls through to itself
 * rather than to a blank, so a status Stripe adds later still reads. */
const invoiceStatusLabels: Record<string, string> = {
	draft: 'Draft',
	open: 'Open',
	paid: 'Paid',
	uncollectible: 'Uncollectible',
	void: 'Void'
};

export function invoiceStatusLabel(status: string): string {
	return invoiceStatusLabels[status] ?? status;
}
