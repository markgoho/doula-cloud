/**
 * The words Getting paid (#2046) and its payment terms question share: a
 * row's value on the summary, and the sentence a save says on its way
 * back to it. One place, so the page and the question cannot name one
 * fact two ways.
 */
import type { BillingMode, PaymentTerms } from '#lib/invoice.js';
import type { PracticeWebsite } from '#lib/website.js';

/**
 * The billing mode's two answers, in InvoiceSection's own labels for the
 * same question (#1590), without its full stops: here they are a value,
 * not an option a person reads as a sentence.
 */
export const billingModeOptions = [
	{
		value: 'stripe' as const,
		label: 'By card, through Stripe',
		description: 'DoulaCloud sends the bill and the Client pays by card.'
	},
	{
		value: 'by_hand' as const,
		label: 'By hand',
		description: 'This Practice sends the bill and collects the payment itself, and records each payment on the Engagement.'
	}
];

/**
 * The billing mode as its row's value. A Practice that has raised no
 * Invoice has chosen nothing, and is asked on the first one (#1922).
 */
export function billingModeValue(mode: BillingMode | undefined): string {
	if (mode === undefined) return 'Not chosen yet. You are asked the first time an Invoice is raised.';
	return billingModeOptions.find((option) => option.value === mode)!.label;
}

/**
What a billing mode save says.
*/
export function billingModeSavedMessage(mode: BillingMode): string {
	return mode === 'by_hand'
		? 'Billing mode saved. This Practice bills Clients by hand.'
		: 'Billing mode saved. This Practice bills Clients by card, through Stripe.';
}

/**
 * The payment terms as their row's value. The default is named as one,
 * so "30" chosen and "30" inherited do not read the same (#768).
 */
export function paymentTermsValue(terms: PaymentTerms): string {
	const due = `Due ${terms.netDays} ${terms.netDays === 1 ? 'day' : 'days'} after an invoice is raised`;
	return terms.isDefault ? `${due}. This is the default.` : due;
}

/**
What a payment terms save says on Getting paid.
*/
export function paymentTermsSavedMessage(terms: PaymentTerms): string {
	return `Payment terms saved. The next invoice raised is due ${terms.netDays} ${terms.netDays === 1 ? 'day' : 'days'} after it is raised.`;
}

/**
 * The query that marks a visit to Your website as one Getting paid sent
 * (MO-G15, #2196), so its save comes back here rather than staying put.
 * A fixed flag, not a return address: Getting paid is the one place it
 * returns to, so nothing read off the URL is ever followed.
 */
export const FROM_GETTING_PAID_QUERY = 'from=getting-paid';

/**
What a website save says on Getting paid, for the answer it saved.
*/
export function websiteSavedMessage(mode: PracticeWebsite['mode']): string {
	return mode === 'hosted'
		? 'Your page is published. You can connect Stripe now.'
		: 'Your website is saved. You can connect Stripe now.';
}
