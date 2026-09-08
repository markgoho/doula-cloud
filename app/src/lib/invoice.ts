/**
 * An Invoice billed against an Engagement's Contract, created via Stripe's
 * Invoicing API on behalf of the Practice's connected account (#79/#81).
 * Stripe hosts the payment page and emails the Client -- this module only
 * loads/creates the Invoice record Doula Cloud keeps, decoupled from
 * SvelteKit and the DOM so it can be unit-tested directly -- mirrors
 * contract.ts.
 */

import type { Fetcher } from './fetcher.js';

import { apiErrorMessage } from './apiErrorMessage.js';
import { formatMoney } from './money.js';

export interface Invoice {
	id: string;
	contractId: string;
	status: string;
	amountCents: number;
	currency: string;
	createdAt: string;
	paidAt?: string;
	/** A human-readable identifier (#271) -- a by-hand Invoice's own
	 * per-Practice sequence, or Stripe's own `number` -- so a check "for
	 * invoice ___" can be matched against it on paper. */
	reference: string;
	/** Which rail this Invoice was raised on (#271) -- fixed at creation,
	 * independent of the Practice's current billingMode. */
	billingMode: BillingMode;
	/** When this Invoice falls due (#768), written onto it when it was
	 * raised from the Practice's payment terms of that moment -- so
	 * changing the terms later never moves an Invoice already billed. The
	 * date, never a day count: `daysOverdue` derives the count here, at
	 * read, so it stays right in a tab left open overnight. */
	dueAt: string;
}

/** A Practice's choice of billing rail (#271): Stripe-hosted Invoicing,
 * or an Invoice the Practice raises and collects by hand. */
export type BillingMode = 'stripe' | 'by_hand';

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
	/** The part of the outstanding book that is past its due date (#768)
	 * -- a narrowing of the two figures above, never a book beside them,
	 * so an overdue Invoice is counted in both. Whole-book, like every
	 * total here. */
	overdueCents: number;
	overdueCount: number;
	/** Whether Clients can pay this Practice at all (#270) -- an aggregate
	 * fact alongside the three totals above, not derived from them: an
	 * empty book looks the same whether nobody has billed anything yet or
	 * Clients cannot pay this Practice at all. */
	clientsCanPay: boolean;
}

/** What the Practice-wide Invoice route's `load` hands its page: one page
 * of the book, plus which narrowing it was read under (#768). The
 * narrowing lives in the URL rather than in the page's memory, and the
 * page needs it for two things -- which filter link reads as current, and
 * which narrowing every later page asks for. */
export interface PracticeInvoiceListData extends PracticeInvoicePage {
	isNarrowedToOverdue: boolean;
}

/** The Practice-wide Invoice list's path -- exported so the route's
 * `load` can call `apiFetch` on it directly and handle 401/403 the way
 * SvelteKit needs (see the billing route's `+page.ts` for why a
 * role-gated read loads there rather than in `onMount`). */
export function practiceInvoicesPath(practiceId: string, cursor?: string, isNarrowedToOverdue = false): string {
	const path = `/api/practices/${practiceId}/invoices`;
	const query = new URLSearchParams();
	if (isNarrowedToOverdue) {
		query.set('overdue', 'true');
	}
	if (cursor) {
		query.set('cursor', cursor);
	}
	const search = query.toString();
	return search ? `${path}?${search}` : path;
}

/** Loads one page of every Invoice the Practice has billed, newest
 * first. Throws with the response body text on a non-2xx response --
 * the route's `load` maps status codes to SvelteKit errors before
 * calling this, so a throw here is only ever an unexpected failure. */
export async function loadPracticeInvoices(
	fetcher: Fetcher,
	practiceId: string,
	cursor?: string,
	isNarrowedToOverdue = false
): Promise<PracticeInvoicePage> {
	const response = await fetcher(practiceInvoicesPath(practiceId, cursor, isNarrowedToOverdue));
	if (!response.ok) {
		throw new Error(await apiErrorMessage(response));
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
		throw new Error(await apiErrorMessage(response));
	}
	const body: { items: Invoice[] } = await response.json();
	return body.items;
}

/** Creates an Invoice against engagementId's current Contract, for the
 * amount that Contract itself carries -- #947 removed the amountCents a
 * caller used to supply here; the BFF derives it from the Contract's own
 * amount_cents, so no request can make an Invoice disagree with the
 * signed Contract. Whether Clients can pay this Practice at all is a
 * standing fact the caller already has (EngagementDetail.clientsCanPay,
 * #270) and checks before ever showing the form that calls this -- so a
 * refusal here (409, e.g. Stripe still not connected) is always thrown
 * like any other non-2xx response, with the response body text, never a
 * routed gate state.
 *
 * billingMode (#271) is read only the first time this Practice ever
 * raises an Invoice -- the backend ignores it once a mode is already
 * set, so callers pass it only from the inline "how does this Practice
 * bill?" ask InvoiceSection shows exactly then. */
export async function createInvoice(
	fetcher: Fetcher,
	practiceId: string,
	engagementId: string,
	billingMode?: BillingMode
): Promise<Invoice> {
	const response = await fetcher(invoicesPath(practiceId, engagementId), {
		method: 'POST',
		headers: { 'Content-Type': 'application/json' },
		body: JSON.stringify({ billingMode })
	});
	if (!response.ok) {
		throw new Error(await apiErrorMessage(response));
	}
	return response.json();
}

function billingModePath(practiceId: string): string {
	return `/api/practices/${practiceId}/payments/billing-mode`;
}

/** Loads a Practice's current billing mode, undefined when it has never
 * chosen one (#271). Any Staff member may read this -- #270's own
 * reasoning for the sibling "can this Practice raise an Invoice at all"
 * fact. */
export async function loadBillingMode(fetcher: Fetcher, practiceId: string): Promise<BillingMode | undefined> {
	const response = await fetcher(billingModePath(practiceId));
	if (!response.ok) {
		throw new Error(await apiErrorMessage(response));
	}
	const body: { billingMode?: BillingMode } = await response.json();
	return body.billingMode;
}

/** Changes a Practice's already-established billing mode -- Owner-only
 * (#271), unlike the first-ever set, which rides createInvoice's own
 * request. */
export async function setBillingMode(
	fetcher: Fetcher,
	practiceId: string,
	billingMode: BillingMode
): Promise<BillingMode> {
	const response = await fetcher(billingModePath(practiceId), {
		method: 'PUT',
		headers: { 'Content-Type': 'application/json' },
		body: JSON.stringify({ billingMode })
	});
	if (!response.ok) {
		throw new Error(await apiErrorMessage(response));
	}
	const body: { billingMode: BillingMode } = await response.json();
	return body.billingMode;
}

/** The closed set of ways a manually recorded Payment (#271) arrived.
 * "other" requires a note -- see RecordPaymentInput. */
export type PaymentMethod = 'check' | 'bank_transfer' | 'cash' | 'other';

/**
A manually recorded Payment, as returned by recordPayment.
*/
export interface Payment {
	id: string;
	invoiceId: string;
	amountCents: number;
	method: PaymentMethod;
	note?: string;
	paidAt: string;
	createdAt: string;
}

/** What recordPayment sends: the method, an optional note (required by
 * the backend when method is "other"), and the date the recorder says
 * the money arrived ("YYYY-MM-DD"). The amount is never supplied -- it
 * is always the Invoice's own full amount (#271). */
export interface RecordPaymentInput {
	method: PaymentMethod;
	note?: string;
	paidOn: string;
}

function invoiceActionPath(practiceId: string, invoiceId: string, action: string): string {
	return `/api/practices/${practiceId}/invoices/${invoiceId}/${action}`;
}

/** Records a Payment that did not come through Stripe against invoiceId
 * (#271) -- Owner and Admin only; refused (409) unless the Invoice is
 * currently 'open'. Against a Stripe-backed Invoice this also marks
 * Stripe's own copy paid_out_of_band, so a Stripe-side refusal (502)
 * fails the whole record closed and nothing is saved. */
export async function recordPayment(
	fetcher: Fetcher,
	practiceId: string,
	invoiceId: string,
	input: RecordPaymentInput
): Promise<Payment> {
	const response = await fetcher(invoiceActionPath(practiceId, invoiceId, 'payments'), {
		method: 'POST',
		headers: { 'Content-Type': 'application/json' },
		body: JSON.stringify(input)
	});
	if (!response.ok) {
		throw new Error(await apiErrorMessage(response));
	}
	return response.json();
}

/** voidInvoice and writeOffInvoice (#271) move an open by-hand Invoice
 * off the book without a Payment -- a mistyped amount, or one the
 * Practice has given up on collecting. Both are Owner/Admin only and
 * refused on a Stripe-backed Invoice or one that is not 'open'. */
async function transitionInvoice(
	fetcher: Fetcher,
	practiceId: string,
	invoiceId: string,
	action: 'void' | 'write-off'
): Promise<{ status: string }> {
	const response = await fetcher(invoiceActionPath(practiceId, invoiceId, action), {
		method: 'POST',
		headers: { 'Content-Type': 'application/json' },
		body: '{}'
	});
	if (!response.ok) {
		throw new Error(await apiErrorMessage(response));
	}
	return response.json();
}

export function voidInvoice(fetcher: Fetcher, practiceId: string, invoiceId: string): Promise<{ status: string }> {
	return transitionInvoice(fetcher, practiceId, invoiceId, 'void');
}

export function writeOffInvoice(fetcher: Fetcher, practiceId: string, invoiceId: string): Promise<{ status: string }> {
	return transitionInvoice(fetcher, practiceId, invoiceId, 'write-off');
}

/** Formats amountCents as a USD currency string (e.g. "$150.00") for
 * display -- the only place cents-to-dollars conversion happens on read;
 * write-side conversion (dollars the Staff typed -> cents the BFF stores)
 * lives in InvoiceSection.svelte, next to the input it converts. Delegates
 * to money.ts's shared formatMoney (#285) rather than its own copy of the
 * same conversion. */
export function formatAmount(amountCents: number): string {
	return formatMoney(amountCents, 'USD');
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

/** The one Contract status an Invoice may be raised against -- mirrors
 * the Go BFF's one declaration of the precondition
 * (contracts.TransitionBill, api/internal/contracts/lifecycle.go, #275).
 * InvoiceSection derives from this rather than restating its own
 * comparison, so the screen and the API can't drift apart on which
 * Contract states are billable. */
export const billableContractStatus = 'signed';

/** The message InvoiceSection shows in place of the Create Invoice form
 * when the Contract's status isn't billableContractStatus -- #275's UI
 * side: the control is not offered, and the screen says why rather than
 * only hiding it. */
export function unbillableContractMessage(status: string): string {
	if (status === 'voided') {
		return 'This Contract has been voided. Invoicing is unavailable until Staff issues a new Contract.';
	}
	return 'Invoicing is unavailable until the Client signs this Contract.';
}

/** The message InvoiceSection shows in place of the Create Invoice form
 * when Clients cannot pay this Practice yet (#270) -- a fact about the
 * Practice, not a refusal aimed at whoever is looking at the form, and
 * naming the role that clears it rather than fetching the Staff roster
 * to name a person (a Practice may have more than one Owner). */
export const clientsCannotPayMessage =
	'Clients cannot pay this Practice yet. A Practice Owner has to connect Stripe.';

/** The message InvoiceSection shows in place of the Create Invoice form
 * when the Client has no email on file (#270) -- moved ahead of the
 * submit attempt that used to be the only place this was discovered,
 * per #255's own principle: state it as standing information rather than
 * only as feedback after a Send. */
export const clientHasNoEmailMessage = 'This Client has no email address on file. Add one before creating an Invoice.';

/** How many whole days past its due date an Invoice is, at `now` -- 0
 * when it is not late yet (#768).
 *
 * The count is derived here rather than sent by the BFF on purpose: the
 * row carries the date, so the number is right at every instant, not
 * only at the instant the page loaded. Whole days from the due instant,
 * so an Invoice due at 09:00 is not "1 day late" at 09:01.
 *
 * `now` is a parameter, not `Date.now()` read inside: it is what lets a
 * test move the comparison time instead of waiting for one to arrive,
 * which is the only way "overdue" can be proven without a clock, a
 * webhook or a scheduled sweep. */
export function daysOverdue(dueAt: string, now: Date): number {
	const dueMs = new Date(dueAt).getTime();
	const elapsed = now.getTime() - dueMs;
	if (!Number.isFinite(elapsed) || elapsed <= 0) {
		return 0;
	}
	return Math.floor(elapsed / MS_PER_DAY);
}

const MS_PER_DAY = 24 * 60 * 60 * 1000;

/** What the Practice-wide list's "Payment due" cell reads (#768): the due
 * date, and how late it is when it is late.
 *
 * Only an Invoice still awaiting payment can be late. A paid, voided or
 * written-off Invoice keeps its due date on the row and is never called
 * overdue, because none of the three is money still owed -- the same rule
 * the BFF's own overdue totals use, said once here so the screen and the
 * figure above it cannot disagree.
 *
 * "Payment due", never "due date": in this domain a due date is the
 * pregnancy's (ADR-0015), and the two must not share a word. */
export function dueLabel(invoice: Invoice, now: Date): string {
	const due = new Date(invoice.dueAt).toLocaleDateString();
	if (invoice.status !== 'open') {
		return due;
	}
	// Late is "the instant has passed", not "a whole day has passed" --
	// the same test the BFF's own overdue totals and narrowing use. The
	// day count is how late, which is a second question: without this
	// split, an Invoice that fell due an hour ago would be counted in the
	// Practice's overdue figure while its own row said nothing.
	if (new Date(invoice.dueAt).getTime() >= now.getTime()) {
		return due;
	}
	const late = daysOverdue(invoice.dueAt, now);
	if (late === 0) {
		return `${due} — overdue`;
	}
	return `${due} — ${late} ${late === 1 ? 'day' : 'days'} overdue`;
}

/** A Practice's payment terms (#768): how many days after an Invoice is
 * raised it falls due. `isDefault` is true when the Practice has never
 * set any of its own and is running on Doula Cloud's 30 days. */
export interface PaymentTerms {
	netDays: number;
	isDefault: boolean;
}

function paymentTermsPath(practiceId: string): string {
	return `/api/practices/${practiceId}/payments/payment-terms`;
}

/** Loads a Practice's payment terms. Any Staff member may read them --
 * she meets the due date on every Invoice she looks at. */
export async function loadPaymentTerms(fetcher: Fetcher, practiceId: string): Promise<PaymentTerms> {
	const response = await fetcher(paymentTermsPath(practiceId));
	if (!response.ok) {
		throw new Error(await apiErrorMessage(response));
	}
	return response.json();
}

/** Sets a Practice's payment terms -- Owner and Admin only. An Invoice
 * already raised keeps the terms it was billed under; this changes only
 * what the next one is due. */
export async function setPaymentTerms(
	fetcher: Fetcher,
	practiceId: string,
	netDays: number
): Promise<PaymentTerms> {
	const response = await fetcher(paymentTermsPath(practiceId), {
		method: 'PUT',
		headers: { 'Content-Type': 'application/json' },
		body: JSON.stringify({ netDays })
	});
	if (!response.ok) {
		throw new Error(await apiErrorMessage(response));
	}
	return response.json();
}
