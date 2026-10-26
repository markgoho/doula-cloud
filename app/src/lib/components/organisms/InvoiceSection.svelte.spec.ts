import { page } from 'vitest/browser';
import { afterEach, describe, expect, it, vi } from 'vitest';
import { render } from 'vitest-browser-svelte';
import InvoiceSection from './InvoiceSection.svelte';
import type { BillingMode, ContractorInvoice, Invoice, PaymentMethod, RefundPaymentInput } from '#lib/invoice.js';
import { RefusalError } from '#lib/formErrors.js';

interface SetupOptions {
	invoices?: Invoice[];
	contractStatus?: string;
	// 'unset' rather than leaving this undefined: a destructuring default
	// only applies when a key is *absent* or its value is exactly
	// `undefined`, so `setup({ billingMode: undefined })` would silently
	// fall through to the 'stripe' default below instead of exercising
	// InvoiceSection's own not-yet-chosen state.
	billingMode?: BillingMode | 'unset';
	clientsCanPay?: boolean;
	hasClientEmail?: boolean;
	isOwner?: boolean;
	isOwnerOrAdmin?: boolean;
	canReadInvoiceMoney?: boolean;
	contractorInvoices?: ContractorInvoice[];
	practiceTimezone?: string;
	onCreate?: (billingMode?: BillingMode) => Promise<void>;
	onRecordPayment?: (
		invoiceId: string,
		input: { method: PaymentMethod; note?: string; paidOn: string }
	) => Promise<void>;
	onVoidInvoice?: (invoiceId: string) => Promise<void>;
	onWriteOffInvoice?: (invoiceId: string) => Promise<void>;
	onReversePayment?: (invoiceId: string, paymentId: string, reason: string) => Promise<void>;
	onRefundPayment?: (invoiceId: string, paymentId: string, input: RefundPaymentInput) => Promise<void>;
}

const paymentsSettingsHref = 'https://example.test/practices/practice-1/settings/payments';

const invoiceOpen: Invoice = {
	id: 'inv-1',
	contractId: 'contract-1',
	status: 'open',
	amountCents: 15_000,
	currency: 'usd',
	createdAt: '2026-01-01T00:00:00Z',
	dueOn: '2026-01-31',
	reference: 'STRIPE-in_1',
	refundedCents: 0,
	daysOverdue: 0,
	billingMode: 'stripe'
};

const invoicePaid: Invoice = {
	id: 'inv-2',
	contractId: 'contract-1',
	status: 'paid',
	amountCents: 20_000,
	currency: 'usd',
	createdAt: '2026-01-02T00:00:00Z',
	paidOn: '2026-01-05',
	dueOn: '2026-02-01',
	reference: 'STRIPE-in_2',
	refundedCents: 0,
	daysOverdue: 0,
	billingMode: 'stripe'
};

// A paid Stripe-backed Invoice covered by a card Payment (#1009): Return
// money is offered, Reverse payment never is (#945 refuses that rail).
const invoicePaidByCard: Invoice = {
	...invoicePaid,
	id: 'inv-4',
	activePaymentId: 'payment-card',
	activePaymentKind: 'stripe'
};

// A paid by-hand Invoice with an active Payment (#945) -- the one state
// that renders the "Reverse payment" action. A Stripe-backed paid
// Invoice (invoicePaid above) never carries activePaymentId, since
// reversal refuses that case outright.
const invoicePaidByHand: Invoice = {
	id: 'inv-3',
	contractId: 'contract-1',
	status: 'paid',
	amountCents: 25_000,
	currency: 'usd',
	createdAt: '2026-01-03T00:00:00Z',
	paidOn: '2026-01-06',
	dueOn: '2026-02-02',
	reference: 'INV-0003',
	refundedCents: 0,
	daysOverdue: 0,
	billingMode: 'by_hand',
	activePaymentId: 'payment-1',
	activePaymentKind: 'manual'
};

async function setup({
	invoices = [],
	contractStatus = 'signed',
	billingMode = 'stripe',
	clientsCanPay = true,
	hasClientEmail = true,
	isOwner = false,
	isOwnerOrAdmin = false,
	canReadInvoiceMoney = true,
	contractorInvoices = [],
	// America/New_York, the same fixture zone the repo's other timezone
	// tests use (ADR-0036) -- a zone behind UTC, which is the case the
	// future-date boundary tests below exercise.
	practiceTimezone = 'America/New_York',
	onCreate = vi.fn().mockResolvedValue(undefined),
	onRecordPayment = vi.fn().mockResolvedValue(undefined),
	onVoidInvoice = vi.fn().mockResolvedValue(undefined),
	onWriteOffInvoice = vi.fn().mockResolvedValue(undefined),
	onReversePayment = vi.fn().mockResolvedValue(undefined),
	onRefundPayment = vi.fn().mockResolvedValue(undefined)
}: SetupOptions = {}) {
	await render(InvoiceSection, {
		invoices,
		contractStatus,
		billingMode: billingMode === 'unset' ? undefined : billingMode,
		clientsCanPay,
		hasClientEmail,
		isOwner,
		isOwnerOrAdmin,
		canReadInvoiceMoney,
		contractorInvoices,
		practiceTimezone,
		paymentsSettingsHref,
		onCreate,
		onRecordPayment,
		onVoidInvoice,
		onWriteOffInvoice,
		onReversePayment,
		onRefundPayment
	});
	return { onCreate, onRecordPayment, onVoidInvoice, onWriteOffInvoice, onReversePayment, onRefundPayment };
}

/* #2106: the billing-mode question is on the page only once Create Invoice
 * asks for it, so a test of the question opens it first. */
async function setupAsked(options: SetupOptions) {
	const callbacks = await setup(options);
	await page.getByRole('button', { name: 'Create Invoice' }).click();
	return callbacks;
}

afterEach(() => {
	vi.useRealTimers();
});

/*
 * One fact of the Invoice on screen (#2059). DataTable draws a table and a
 * card per row and shows one by its own width; at this spec's width it is
 * the card, whose facts are a Summary list. `getByRole` skips the hidden
 * tree, so this finds the copy a person sees.
 */
function invoiceFact(text: string) {
	return page.getByRole('definition').filter({ hasText: text });
}

/* The card's title is the amount, in a block with no role of its own; the
 * card's tree is the later of DataTable's two, so `last()` is its copy. */
function invoiceTitle(amount: string) {
	return page.getByText(amount, { exact: true }).last();
}

describe('InvoiceSection.svelte', () => {
	it('shows "No Invoices yet." when the list is empty', async () => {
		await setup();

		await expect.element(page.getByText('No Invoices yet.')).toBeInTheDocument();
	});

	// #2059: a row per Invoice, its status a Badge of its own.
	it('lists an Invoice with its formatted amount and its status', async () => {
		await setup({ invoices: [invoiceOpen] });

		await expect.element(invoiceTitle('$150.00')).toBeVisible();
		await expect.element(invoiceFact('Open')).toBeVisible();
	});

	// #2122: the Owner's row reads Overdue from the BFF's count, the same
	// word a contractor and the Practice-wide list read for this Invoice,
	// and still offers the Payment it is waiting on.
	it('reads Overdue once the BFF counts an open Invoice late', async () => {
		await setup({ invoices: [{ ...invoiceOpen, daysOverdue: 12 }], isOwnerOrAdmin: true });

		await expect.element(invoiceFact('Overdue')).toBeVisible();
		await expect.element(invoiceFact('Open')).not.toBeInTheDocument();
		await expect.element(page.getByRole('button', { name: 'Record payment' })).toBeVisible();
	});

	// #2017's rule: no list item holds a control.
	it('puts no Invoice in a list item', async () => {
		await setup({ invoices: [invoiceOpen, invoicePaid], isOwnerOrAdmin: true });

		await expect.element(page.getByRole('button', { name: 'Record payment' })).toBeVisible();
		await expect.element(page.getByRole('listitem')).not.toBeInTheDocument();
	});

	// The same words on every row, so each action names its Invoice.
	it('describes each action by the Invoice it acts on', async () => {
		await setup({ invoices: [invoiceOpen], isOwnerOrAdmin: true });

		await expect
			.element(page.getByRole('button', { name: 'Record payment' }))
			.toHaveAccessibleDescription('Invoice STRIPE-in_1, $150.00');
	});

	// #1958, #1965: paidOn is the day the BFF read in the Practice's own
	// timezone, and it prints as that day in any reader zone, behind UTC
	// or ahead of it.
	it('shows a paid Invoice on the day its Payment was recorded, month first', async () => {
		await setup({ invoices: [invoicePaid] });

		await expect.element(invoiceFact('Paid')).toBeVisible();
		await expect.element(invoiceFact('Jan 5, 2026')).toBeVisible();
	});

	it('falls back to the raw status string for a status with no known label', async () => {
		await setup({ invoices: [{ ...invoiceOpen, status: 'unknown_status' }] });

		await expect.element(invoiceFact('unknown_status')).toBeVisible();
	});

	it('shows the Create Invoice button when the Contract is billable, Clients can pay, and the Client has an email', async () => {
		await setup();

		await expect.element(page.getByRole('button', { name: 'Create Invoice' })).toBeInTheDocument();
	});

	// #947: the amount is the Contract's own, never a figure typed here --
	// clicking Create Invoice calls onCreate with no amount at all.
	it('calls onCreate with no amount', async () => {
		const { onCreate } = await setup();

		await page.getByRole('button', { name: 'Create Invoice' }).click();

		expect(onCreate).toHaveBeenCalledWith();
	});

	// #1775: a refusal is an error summary, as the Record payment form's
	// is, never a bare alert line under the button.
	it('shows a refusal from onCreate in an error summary', async () => {
		const onCreate = vi.fn().mockRejectedValue(new Error('engagement not found'));
		await setup({ onCreate });

		await page.getByRole('button', { name: 'Create Invoice' }).click();

		await expect.element(page.getByRole('heading', { name: 'There is a problem' })).toBeVisible();
		await expect.element(page.getByRole('alert')).toHaveTextContent('engagement not found');
	});

	it('falls back to a generic message when onCreate rejects with a non-Error', async () => {
		const onCreate = vi.fn().mockRejectedValue('boom');
		await setup({ onCreate });

		await page.getByRole('button', { name: 'Create Invoice' }).click();

		await expect.element(page.getByRole('alert')).toHaveTextContent('Failed to create invoice');
	});

	// The question is not on screen once a billing mode is set, so there
	// is no control to link to: the refusal stays an untargeted entry.
	it('shows a billingMode refusal as an untargeted entry when the question is not on screen', async () => {
		const onCreate = vi
			.fn()
			.mockRejectedValue(
				new RefusalError('refused', { billingMode: 'Select how this Practice bills Clients' })
			);
		await setup({ onCreate });

		await page.getByRole('button', { name: 'Create Invoice' }).click();

		await expect.element(page.getByRole('alert')).toHaveTextContent('refused');
		await expect.element(page.getByRole('link', { name: 'Select how this Practice bills Clients' })).not.toBeInTheDocument();
	});

	// #270: clientsCanPay is a standing fact read before the form ever
	// shows, replacing the old post-submit connectRequired/isOwner gate.
	it('shows the cannot-pay Notice and a link to Payments settings for an Owner when Clients cannot pay', async () => {
		await setup({ clientsCanPay: false, isOwner: true });

		await expect
			.element(page.getByText('Clients cannot pay this Practice yet. A Practice Owner has to connect Stripe.'))
			.toBeVisible();
		await expect.element(page.getByRole('link', { name: 'Go to Payments settings' })).toBeVisible();
		await expect.element(page.getByRole('button', { name: 'Create Invoice' })).not.toBeInTheDocument();
	});

	it('shows the cannot-pay Notice with no link for a non-Owner when Clients cannot pay', async () => {
		await setup({ clientsCanPay: false, isOwner: false });

		await expect
			.element(page.getByText('Clients cannot pay this Practice yet. A Practice Owner has to connect Stripe.'))
			.toBeVisible();
		await expect.element(page.getByRole('link', { name: 'Go to Payments settings' })).not.toBeInTheDocument();
		await expect.element(page.getByRole('button', { name: 'Create Invoice' })).not.toBeInTheDocument();
	});

	it('shows a Notice naming the missing email instead of the form when the Client has no email', async () => {
		await setup({ hasClientEmail: false });

		await expect
			.element(page.getByText('This Client has no email address on file. Add one before creating an Invoice.'))
			.toBeVisible();
		await expect.element(page.getByRole('button', { name: 'Create Invoice' })).not.toBeInTheDocument();
	});

	// #275: a Contract that cannot be billed hides Create Invoice and says
	// why, taking priority over the clientsCanPay/hasClientEmail checks --
	// a voided Contract at a Practice Clients cannot pay must not show the
	// cannot-pay Notice first.
	it('shows why billing is unavailable instead of the form on a draft Contract', async () => {
		await setup({ contractStatus: 'draft' });

		await expect
			.element(page.getByText('Invoicing is unavailable until the Client signs this Contract.'))
			.toBeVisible();
		await expect.element(page.getByRole('button', { name: 'Create Invoice' })).not.toBeInTheDocument();
	});

	it('shows why billing is unavailable instead of the form on a sent (unsigned) Contract', async () => {
		await setup({ contractStatus: 'sent' });

		await expect
			.element(page.getByText('Invoicing is unavailable until the Client signs this Contract.'))
			.toBeVisible();
		await expect.element(page.getByRole('button', { name: 'Create Invoice' })).not.toBeInTheDocument();
	});

	it('shows a voided-specific message instead of the form on a voided Contract', async () => {
		await setup({ contractStatus: 'voided' });

		await expect
			.element(
				page.getByText('This Contract has been voided. Invoicing is unavailable until Staff issues a new Contract.')
			)
			.toBeVisible();
		await expect.element(page.getByRole('button', { name: 'Create Invoice' })).not.toBeInTheDocument();
	});

	it('takes priority over the cannot-pay Notice on an unbillable Contract', async () => {
		await setup({ contractStatus: 'voided', clientsCanPay: false, isOwner: true });

		await expect.element(page.getByRole('link', { name: 'Go to Payments settings' })).not.toBeInTheDocument();
		await expect.element(page.getByText(/This Contract has been voided/)).toBeVisible();
	});

	// #271: the first Invoice a Practice ever raises asks which rail it
	// bills on, inline -- never a routed gate and never assumed from an
	// absent Stripe Connect account.
	describe('billing mode not yet chosen (#271)', () => {
		it('offers one Create Invoice and does not put the question on the page until it is pressed (#2106)', async () => {
			await setup({ billingMode: 'unset' });

			await expect.element(page.getByRole('button', { name: 'Create Invoice' })).toBeVisible();
			await expect.element(page.getByText('How does this Practice bill Clients?')).not.toBeInTheDocument();
		});

		it('opens the question under its own heading, which takes focus (#2106)', async () => {
			await setupAsked({ billingMode: 'unset' });

			const heading = page.getByRole('heading', { name: 'Create an Invoice' });
			await expect.element(heading).toBeVisible();
			await expect.element(heading).toHaveFocus();
			await expect.element(page.getByRole('group', { name: 'How does this Practice bill Clients?' })).toBeVisible();
		});

		it('closes the question on Cancel, sends nothing, and forgets the choice (#2106)', async () => {
			const { onCreate } = await setupAsked({ billingMode: 'unset' });

			await page.getByLabelText('By hand.').click();
			await page.getByRole('button', { name: 'Cancel' }).click();

			expect(onCreate).not.toHaveBeenCalled();
			await expect.element(page.getByText('How does this Practice bill Clients?')).not.toBeInTheDocument();
			await page.getByRole('button', { name: 'Create Invoice' }).click();
			await expect.element(page.getByLabelText('By hand.')).not.toBeChecked();
		});

		it('says the Invoice was created once the first raise works (#2106)', async () => {
			await setupAsked({ billingMode: 'unset' });

			await page.getByLabelText('By hand.').click();
			await page.getByRole('button', { name: 'Create Invoice' }).click();

			const created = page.getByText('Invoice created.');
			await expect.element(created).toBeVisible();
			await expect.element(created).toHaveFocus();
		});

		it('shows the billing-mode ask with neither cannot-pay Notice', async () => {
			await setupAsked({ billingMode: 'unset', clientsCanPay: false, hasClientEmail: false });

			await expect.element(page.getByText('How does this Practice bill Clients?')).toBeVisible();
			await expect.element(page.getByLabelText('By card, through Stripe.')).toBeVisible();
			await expect.element(page.getByLabelText('By hand.')).toBeVisible();
			await expect.element(page.getByRole('button', { name: 'Create Invoice' })).toBeVisible();
			await expect.element(page.getByText(/Clients cannot pay this Practice/)).not.toBeInTheDocument();
		});

		it('says the words of the question, the hint and both options (#1590)', async () => {
			await setupAsked({ billingMode: 'unset', clientsCanPay: true });

			await expect.element(page.getByText('You answer this one time. A Practice Owner can change it later in Settings.')).toBeVisible();
			await expect.element(page.getByText('DoulaCloud sends the bill and the Client pays by card.', { exact: true })).toBeVisible();
			await expect
				.element(page.getByText('This Practice sends the bill and collects the payment itself. You record each payment here.'))
				.toBeVisible();
		});

		it('selects no option when the question opens (#1590)', async () => {
			await setupAsked({ billingMode: 'unset' });

			await expect.element(page.getByLabelText('By card, through Stripe.')).not.toBeChecked();
			await expect.element(page.getByLabelText('By hand.')).not.toBeChecked();
		});

		it('warns on the Stripe option, before the choice, when Clients cannot pay yet (#1590)', async () => {
			await setupAsked({ billingMode: 'unset', clientsCanPay: false });

			await expect.element(page.getByText('A Practice Owner must connect Stripe first.', { exact: false })).toBeVisible();
		});

		it('does not warn when Clients can already pay (#1590)', async () => {
			await setupAsked({ billingMode: 'unset', clientsCanPay: true });

			await expect.element(page.getByText('must connect Stripe first', { exact: false })).not.toBeInTheDocument();
		});

		it('submits the chosen mode on the first raise', async () => {
			const { onCreate } = await setupAsked({ billingMode: 'unset' });

			await page.getByLabelText('By hand.').click();
			await page.getByRole('button', { name: 'Create Invoice' }).click();

			expect(onCreate).toHaveBeenCalledWith('by_hand');
		});

		it('submits Stripe when that option is chosen', async () => {
			const { onCreate } = await setupAsked({ billingMode: 'unset' });

			await page.getByLabelText('By card, through Stripe.').click();
			await page.getByRole('button', { name: 'Create Invoice' }).click();

			expect(onCreate).toHaveBeenCalledWith('stripe');
		});

		it('refuses a submit with no option chosen, names the question, and sends no request (#1590)', async () => {
			const { onCreate } = await setupAsked({ billingMode: 'unset' });

			await page.getByRole('button', { name: 'Create Invoice' }).click();

			expect(onCreate).not.toHaveBeenCalled();
			await expect
				.element(page.getByRole('link', { name: 'Select how this Practice bills Clients' }))
				.toHaveAttribute('href', '#invoice-billing-mode-stripe');
			await expect.element(page.getByLabelText('By card, through Stripe.')).toBeVisible();
		});

		it('clears the refusal once an option is chosen and the form is sent (#1590)', async () => {
			const { onCreate } = await setupAsked({ billingMode: 'unset' });

			await page.getByRole('button', { name: 'Create Invoice' }).click();
			await page.getByLabelText('By hand.').click();
			await page.getByRole('button', { name: 'Create Invoice' }).click();

			expect(onCreate).toHaveBeenCalledWith('by_hand');
			await expect.element(page.getByRole('link', { name: 'Select how this Practice bills Clients' })).not.toBeInTheDocument();
		});

		// #1775: the 409 #1590 names (Stripe not connected) names no field,
		// so it is the summary's one untargeted entry.
		it('shows a refusal from the billing-mode ask in the error summary', async () => {
			const onCreate = vi
				.fn()
				.mockRejectedValue(
					new RefusalError('Clients cannot pay this Practice yet. A Practice Owner has to connect Stripe.')
				);
			await setupAsked({ billingMode: 'unset', onCreate });

			await page.getByLabelText('By card, through Stripe.').click();
			await page.getByRole('button', { name: 'Create Invoice' }).click();

			await expect.element(page.getByRole('heading', { name: 'There is a problem' })).toBeVisible();
			await expect
				.element(page.getByRole('alert'))
				.toHaveTextContent('Clients cannot pay this Practice yet. A Practice Owner has to connect Stripe.');
		});

		it('shows a refusal that names the billing-mode question against the group and in the summary (#1775)', async () => {
			const onCreate = vi.fn().mockRejectedValue(
				new RefusalError('This Practice has not chosen how it bills Clients yet.', {
					billingMode: 'Select how this Practice bills Clients'
				})
			);
			await setupAsked({ billingMode: 'unset', onCreate });

			await page.getByLabelText('By hand.').click();
			await page.getByRole('button', { name: 'Create Invoice' }).click();

			await expect
				.element(page.getByRole('link', { name: 'Select how this Practice bills Clients' }))
				.toHaveAttribute('href', '#invoice-billing-mode-stripe');
			await expect
				.element(page.getByRole('group', { name: 'How does this Practice bill Clients?' }))
				.toHaveTextContent('Select how this Practice bills Clients');
			await expect.element(page.getByText('This Practice has not chosen how it bills Clients yet.')).not.toBeInTheDocument();
		});
	});

	// #271, #430: a by-hand Invoice mails nothing, so neither the
	// cannot-pay nor the no-email check ever applies to it.
	it('shows the Create Invoice button directly on the by-hand rail even when Clients cannot pay and the Client has no email', async () => {
		await setup({ billingMode: 'by_hand', clientsCanPay: false, hasClientEmail: false });

		await expect.element(page.getByRole('button', { name: 'Create Invoice' })).toBeVisible();
		await expect.element(page.getByText(/Clients cannot pay this Practice/)).not.toBeInTheDocument();
		await expect.element(page.getByText(/no email address on file/)).not.toBeInTheDocument();
	});

	describe('recording a Payment (#271)', () => {
		it('offers Record payment for an Owner/Admin on an open Invoice, and not on a paid one', async () => {
			await setup({ invoices: [invoiceOpen, invoicePaid], isOwnerOrAdmin: true });

			expect(page.getByRole('button', { name: 'Record payment' }).all()).toHaveLength(1);
		});

		it('offers no Record payment action for a Doula', async () => {
			await setup({ invoices: [invoiceOpen], isOwnerOrAdmin: false });

			await expect.element(page.getByRole('button', { name: 'Record payment' })).not.toBeInTheDocument();
		});

		it('walks Continue then Confirm and record through to onRecordPayment, then closes the form', async () => {
			const { onRecordPayment } = await setup({ invoices: [invoiceOpen], isOwnerOrAdmin: true });

			await page.getByRole('button', { name: 'Record payment' }).click();
			await page.getByLabelText('Bank transfer').click();
			await page.getByLabelText('Note (optional)').fill('Zelle, screenshot on file');
			await page.getByLabelText('Date received').fill('2026-01-02');
			await page.getByRole('button', { name: 'Continue' }).click();

			await expect.element(page.getByText('Bank transfer')).toBeVisible();
			await expect.element(page.getByText('Zelle, screenshot on file')).toBeVisible();
			// The day typed, month first (#1958), never one day earlier.
			await expect.element(page.getByText('Jan 2, 2026', { exact: true })).toBeVisible();

			await page.getByRole('button', { name: 'Confirm and record' }).click();

			expect(onRecordPayment).toHaveBeenCalledWith(invoiceOpen.id, {
				method: 'bank_transfer',
				note: 'Zelle, screenshot on file',
				paidOn: '2026-01-02'
			});
			await expect.element(page.getByRole('button', { name: 'Confirm and record' })).not.toBeInTheDocument();
		});

		it('Change returns from the review step to the form without losing what was entered', async () => {
			await setup({ invoices: [invoiceOpen], isOwnerOrAdmin: true });

			await page.getByRole('button', { name: 'Record payment' }).click();
			await page.getByRole('radio', { name: 'Check' }).click();
			await page.getByLabelText('Date received').fill('2026-01-02');
			await page.getByRole('button', { name: 'Continue' }).click();
			await page.getByRole('button', { name: 'Change' }).click();

			await expect.element(page.getByLabelText('Date received')).toHaveValue('2026-01-02');
		});

		it('Cancel closes the form without calling onRecordPayment', async () => {
			const { onRecordPayment } = await setup({ invoices: [invoiceOpen], isOwnerOrAdmin: true });

			await page.getByRole('button', { name: 'Record payment' }).click();
			await page.getByRole('button', { name: 'Cancel' }).click();

			expect(onRecordPayment).not.toHaveBeenCalled();
			await expect.element(page.getByLabelText('Date received')).not.toBeInTheDocument();
		});

		// #1913, ADR-0055: how a Client paid is a question, so it opens
		// with nothing chosen. The date is pre-filled on purpose (#1923).
		it('opens with no method chosen, and today as the date received', async () => {
			vi.useFakeTimers();
			vi.setSystemTime(new Date('2026-06-15T15:00:00Z'));
			await setup({ invoices: [invoiceOpen], isOwnerOrAdmin: true, practiceTimezone: 'America/New_York' });

			await page.getByRole('button', { name: 'Record payment' }).click();

			for (const name of ['Check', 'Bank transfer', 'Cash', 'Other']) {
				await expect.element(page.getByRole('radio', { name })).not.toBeChecked();
			}
			await expect.element(page.getByLabelText('Date received')).toHaveValue('2026-06-15');
		});

		it('refuses Continue with no method chosen, names the question, and does not reach the review step', async () => {
			const { onRecordPayment } = await setup({ invoices: [invoiceOpen], isOwnerOrAdmin: true });

			await page.getByRole('button', { name: 'Record payment' }).click();
			await page.getByRole('button', { name: 'Continue' }).click();

			await expect
				.element(page.getByRole('link', { name: 'Select how the payment was made' }))
				.toHaveAttribute('href', '#record-payment-method-check');
			await expect
				.element(page.getByRole('group', { name: 'Method' }))
				.toHaveTextContent('Select how the payment was made');
			await expect.element(page.getByRole('button', { name: 'Continue' })).toBeVisible();
			expect(page.getByRole('button', { name: 'Confirm and record' }).all()).toHaveLength(0);
			expect(onRecordPayment).not.toHaveBeenCalled();
		});

		it('opens with no method chosen again when the form is closed and opened', async () => {
			await setup({ invoices: [invoiceOpen], isOwnerOrAdmin: true });

			await page.getByRole('button', { name: 'Record payment' }).click();
			await page.getByRole('radio', { name: 'Cash' }).click();
			await page.getByRole('button', { name: 'Cancel' }).click();
			await page.getByRole('button', { name: 'Record payment' }).click();

			await expect.element(page.getByRole('radio', { name: 'Cash' })).not.toBeChecked();
		});

		// #1038: the message is wired into the Note field itself -- marked
		// invalid via LabeledField's own error slot -- not only a detached
		// role="alert" paragraph.
		it('requires a note when the method is Other, wired into the Note field', async () => {
			await setup({ invoices: [invoiceOpen], isOwnerOrAdmin: true });

			await page.getByRole('button', { name: 'Record payment' }).click();
			await page.getByLabelText('Other').click();
			await page.getByLabelText('Date received').fill('2026-01-02');
			await page.getByRole('button', { name: 'Continue' }).click();

			// .first() -- #1228's ErrorSummary repeats every message as a
			// link, so the wording now appears twice on screen, same as the
			// established pattern elsewhere in this suite (fillFacts, etc).
			await expect.element(page.getByText('Enter a note for "Other"').first()).toBeVisible();
			await expect.element(page.getByLabelText('Note (optional)')).toHaveAttribute('aria-invalid', 'true');
			await expect.element(page.getByLabelText('Date received')).toHaveAttribute('aria-invalid', 'false');
		});

		// #1038: wired into the Date received field, not a detached alert.
		it('rejects a date in the future, wired into the Date received field', async () => {
			await setup({ invoices: [invoiceOpen], isOwnerOrAdmin: true });

			const future = new Date();
			future.setDate(future.getDate() + 1);

			await page.getByRole('button', { name: 'Record payment' }).click();
			await page.getByRole('radio', { name: 'Check' }).click();
			await page.getByLabelText('Date received').fill(future.toISOString().slice(0, 10));
			await page.getByRole('button', { name: 'Continue' }).click();

			await expect.element(page.getByText('The date cannot be in the future').first()).toBeVisible();
			await expect.element(page.getByLabelText('Date received')).toHaveAttribute('aria-invalid', 'true');
		});

		// #1280: the same boundary #1167 already fixed server-side. 02:30
		// UTC on the 16th is 22:30 on the 15th in America/New_York -- a
		// zone behind UTC -- so the Practice's own "today" is the 15th,
		// not the 16th `toISOString().slice(0, 10)` used to read. June,
		// not the real run date, and past DST's spring-forward edge (no
		// ambiguity from the clock change itself) -- a date matching
		// whatever day this suite happens to run on would pass whether or
		// not `vi.setSystemTime` actually reaches the component's own
		// `new Date()` in the browser realm, which is exactly the thing
		// this test has to prove.
		it("reads today in the Practice's own zone, not UTC's, for both the default date and the future-date ceiling", async () => {
			vi.useFakeTimers();
			vi.setSystemTime(new Date('2026-06-16T02:30:00Z'));
			await setup({ invoices: [invoiceOpen], isOwnerOrAdmin: true, practiceTimezone: 'America/New_York' });

			await page.getByRole('button', { name: 'Record payment' }).click();
			await page.getByRole('radio', { name: 'Check' }).click();
			// The default -- todayIsoDate() feeds paymentDate's own initial
			// value (startRecordingPayment).
			await expect.element(page.getByLabelText('Date received')).toHaveValue('2026-06-15');

			// UTC's own "today" is the zone's tomorrow, still refused.
			await page.getByLabelText('Date received').fill('2026-06-16');
			await page.getByRole('button', { name: 'Continue' }).click();
			await expect.element(page.getByText('The date cannot be in the future').first()).toBeVisible();

			// The zone's own "today" clears the same ceiling.
			await page.getByLabelText('Date received').fill('2026-06-15');
			await page.getByRole('button', { name: 'Continue' }).click();
			await expect.element(page.getByRole('button', { name: 'Confirm and record' })).toBeVisible();
			await expect.element(page.getByText('The date cannot be in the future')).not.toBeInTheDocument();
		});

		// #1228: `Date received`'s own `required` was the only refusal this
		// field had; StackedForm's `novalidate` (ADR-0021) took it away, so
		// `reviewPayment` covers emptiness itself now. The field defaults
		// to today's date (todayIsoDate()), so this clears it first --
		// otherwise there is nothing empty to refuse.
		it('rejects an empty date, wired into the Date received field, without calling onRecordPayment', async () => {
			const { onRecordPayment } = await setup({ invoices: [invoiceOpen], isOwnerOrAdmin: true });

			await page.getByRole('button', { name: 'Record payment' }).click();
			await page.getByRole('radio', { name: 'Check' }).click();
			await page.getByLabelText('Date received').fill('');
			await page.getByRole('button', { name: 'Continue' }).click();

			expect(onRecordPayment).not.toHaveBeenCalled();
			await expect.element(page.getByText('Enter the date the payment was received').first()).toBeVisible();
			await expect.element(page.getByLabelText('Date received')).toHaveAttribute('aria-invalid', 'true');
		});

		it('shows an error when onRecordPayment throws', async () => {
			const onRecordPayment = vi
				.fn()
				.mockRejectedValue(new Error('This Invoice is not open, so nothing can be recorded or changed against it.'));
			await setup({ invoices: [invoiceOpen], isOwnerOrAdmin: true, onRecordPayment });

			await page.getByRole('button', { name: 'Record payment' }).click();
			await page.getByRole('radio', { name: 'Check' }).click();
			await page.getByLabelText('Date received').fill('2026-01-02');
			await page.getByRole('button', { name: 'Continue' }).click();
			await page.getByRole('button', { name: 'Confirm and record' }).click();

			await expect
				.element(page.getByText('This Invoice is not open, so nothing can be recorded or changed against it.'))
				.toBeVisible();
		});

		it('falls back to a generic message when onRecordPayment rejects with a non-Error', async () => {
			const onRecordPayment = vi.fn().mockRejectedValue('boom');
			await setup({ invoices: [invoiceOpen], isOwnerOrAdmin: true, onRecordPayment });

			await page.getByRole('button', { name: 'Record payment' }).click();
			await page.getByRole('radio', { name: 'Check' }).click();
			await page.getByLabelText('Date received').fill('2026-01-02');
			await page.getByRole('button', { name: 'Continue' }).click();
			await page.getByRole('button', { name: 'Confirm and record' }).click();

			await expect.element(page.getByText('Failed to record payment')).toBeVisible();
		});

		// #1038: PostManualPaymentHandler's own `details` map (#1037) --
		// each of the three keys read onto the field it names, per
		// formErrors.ts's GOV.UK rules, rather than a detached alert.
		it('wires a note refusal from the BFF onto the Note field', async () => {
			const onRecordPayment = vi
				.fn()
				.mockRejectedValue(
					new RefusalError('note is required when method is "other"', {
						note: 'Enter a note for "Other"'
					})
				);
			await setup({ invoices: [invoiceOpen], isOwnerOrAdmin: true, onRecordPayment });

			await page.getByRole('button', { name: 'Record payment' }).click();
			await page.getByRole('radio', { name: 'Check' }).click();
			await page.getByLabelText('Date received').fill('2026-01-02');
			await page.getByRole('button', { name: 'Continue' }).click();
			await page.getByRole('button', { name: 'Confirm and record' }).click();

			await expect.element(page.getByText('Enter a note for "Other"').first()).toBeVisible();
			await expect.element(page.getByLabelText('Note (optional)')).toHaveAttribute('aria-invalid', 'true');
		});

		it('wires a paidOn refusal from the BFF onto the Date received field', async () => {
			const onRecordPayment = vi
				.fn()
				.mockRejectedValue(
					new RefusalError('paidOn must be a date in YYYY-MM-DD form', {
						paidOn: 'Enter the date received as a real date, like 2027-04-23'
					})
				);
			await setup({ invoices: [invoiceOpen], isOwnerOrAdmin: true, onRecordPayment });

			await page.getByRole('button', { name: 'Record payment' }).click();
			await page.getByRole('radio', { name: 'Check' }).click();
			await page.getByLabelText('Date received').fill('2026-01-02');
			await page.getByRole('button', { name: 'Continue' }).click();
			await page.getByRole('button', { name: 'Confirm and record' }).click();

			await expect
				.element(page.getByText('Enter the date received as a real date, like 2027-04-23').first())
				.toBeVisible();
			await expect.element(page.getByLabelText('Date received')).toHaveAttribute('aria-invalid', 'true');
		});

		it('wires a method refusal from the BFF onto the Method group', async () => {
			const onRecordPayment = vi.fn().mockRejectedValue(
				new RefusalError('method must be "check", "bank_transfer", "cash", or "other"', {
					method: 'Select how the payment was made'
				})
			);
			await setup({ invoices: [invoiceOpen], isOwnerOrAdmin: true, onRecordPayment });

			await page.getByRole('button', { name: 'Record payment' }).click();
			await page.getByRole('radio', { name: 'Check' }).click();
			await page.getByLabelText('Date received').fill('2026-01-02');
			await page.getByRole('button', { name: 'Continue' }).click();
			await page.getByRole('button', { name: 'Confirm and record' }).click();

			await expect.element(page.getByText('Select how the payment was made').first()).toBeVisible();
		});

		// #1038's second shape: a refusal naming no field of this form --
		// a Stripe-side failure -- still falls back to the untargeted
		// role="alert" summary, per docs/api-design.md section 7.
		it('falls back to the untargeted summary for a RefusalError with no details at all', async () => {
			const onRecordPayment = vi
				.fn()
				.mockRejectedValue(
					new RefusalError('Stripe would not mark this Invoice paid, so nothing was recorded.')
				);
			await setup({ invoices: [invoiceOpen], isOwnerOrAdmin: true, onRecordPayment });

			await page.getByRole('button', { name: 'Record payment' }).click();
			await page.getByRole('radio', { name: 'Check' }).click();
			await page.getByLabelText('Date received').fill('2026-01-02');
			await page.getByRole('button', { name: 'Continue' }).click();
			await page.getByRole('button', { name: 'Confirm and record' }).click();

			await expect
				.element(page.getByText('Stripe would not mark this Invoice paid, so nothing was recorded.'))
				.toBeVisible();
			// Stays on the review step: an untargeted refusal has no field
			// to send the reader back to, unlike the targeted refusals above.
			await expect.element(page.getByLabelText('Note (optional)')).not.toBeInTheDocument();
		});

		it('falls back to the untargeted summary when a RefusalError\'s details name no field of this form', async () => {
			const onRecordPayment = vi
				.fn()
				.mockRejectedValue(new RefusalError('refused', { someUnrelatedField: 'x' }));
			await setup({ invoices: [invoiceOpen], isOwnerOrAdmin: true, onRecordPayment });

			await page.getByRole('button', { name: 'Record payment' }).click();
			await page.getByRole('radio', { name: 'Check' }).click();
			await page.getByLabelText('Date received').fill('2026-01-02');
			await page.getByRole('button', { name: 'Continue' }).click();
			await page.getByRole('button', { name: 'Confirm and record' }).click();

			await expect.element(page.getByText('refused')).toBeVisible();
		});
	});

	describe('void and write off a by-hand Invoice (#271)', () => {
		const openByHand: Invoice = { ...invoiceOpen, id: 'inv-3', billingMode: 'by_hand', reference: 'INV-0003' };

		it('offers Void and Write off for a by-hand open Invoice, but not for a Stripe-backed one', async () => {
			await setup({ invoices: [invoiceOpen, openByHand], isOwnerOrAdmin: true });

			expect(page.getByRole('button', { name: 'Void' }).all()).toHaveLength(1);
			expect(page.getByRole('button', { name: 'Write off' }).all()).toHaveLength(1);
		});

		it('calls onVoidInvoice when Void is clicked', async () => {
			const { onVoidInvoice } = await setup({ invoices: [openByHand], isOwnerOrAdmin: true });

			await page.getByRole('button', { name: 'Void' }).click();

			expect(onVoidInvoice).toHaveBeenCalledWith(openByHand.id);
		});

		it('calls onWriteOffInvoice when Write off is clicked', async () => {
			const { onWriteOffInvoice } = await setup({ invoices: [openByHand], isOwnerOrAdmin: true });

			await page.getByRole('button', { name: 'Write off' }).click();

			expect(onWriteOffInvoice).toHaveBeenCalledWith(openByHand.id);
		});

		it('shows an error when onWriteOffInvoice throws', async () => {
			const onWriteOffInvoice = vi.fn().mockRejectedValue(new Error('This Invoice is not open.'));
			await setup({ invoices: [openByHand], isOwnerOrAdmin: true, onWriteOffInvoice });

			await page.getByRole('button', { name: 'Write off' }).click();

			await expect.element(page.getByText('This Invoice is not open.')).toBeVisible();
		});

		it('shows an error when onVoidInvoice throws', async () => {
			const onVoidInvoice = vi.fn().mockRejectedValue(new Error('This Invoice is not open.'));
			await setup({ invoices: [openByHand], isOwnerOrAdmin: true, onVoidInvoice });

			await page.getByRole('button', { name: 'Void' }).click();

			await expect.element(page.getByText('This Invoice is not open.')).toBeVisible();
		});

		it('falls back to a generic message when onVoidInvoice rejects with a non-Error', async () => {
			const onVoidInvoice = vi.fn().mockRejectedValue('boom');
			await setup({ invoices: [openByHand], isOwnerOrAdmin: true, onVoidInvoice });

			await page.getByRole('button', { name: 'Void' }).click();

			await expect.element(page.getByText('Failed to void this Invoice')).toBeVisible();
		});

		it('falls back to a generic message when onWriteOffInvoice rejects with a non-Error', async () => {
			const onWriteOffInvoice = vi.fn().mockRejectedValue('boom');
			await setup({ invoices: [openByHand], isOwnerOrAdmin: true, onWriteOffInvoice });

			await page.getByRole('button', { name: 'Write off' }).click();

			await expect.element(page.getByText('Failed to write off this Invoice')).toBeVisible();
		});
	});

	describe('reverse a manually recorded Payment (#945)', () => {
		it('offers Reverse payment for a paid by-hand Invoice with an active Payment, but not for a paid Stripe-backed one', async () => {
			await setup({ invoices: [invoicePaid, invoicePaidByHand], isOwnerOrAdmin: true });

			await expect.element(page.getByRole('button', { name: 'Reverse payment' })).toBeVisible();
			expect(page.getByRole('button', { name: 'Reverse payment' }).all()).toHaveLength(1);
		});

		it('hides Reverse payment from anyone but Owner or Admin', async () => {
			await setup({ invoices: [invoicePaidByHand], isOwnerOrAdmin: false });

			expect(page.getByRole('button', { name: 'Reverse payment' }).all()).toHaveLength(0);
		});

		// #1038: wired into the Reason field itself, not a detached alert.
		it('blocks Continue with a blank reason, wired into the Reason field', async () => {
			await setup({ invoices: [invoicePaidByHand], isOwnerOrAdmin: true });

			await page.getByRole('button', { name: 'Reverse payment' }).click();
			await page.getByRole('button', { name: 'Continue' }).click();

			await expect.element(page.getByText('Enter a reason for reversing this payment')).toBeVisible();
			await expect.element(page.getByLabelText('Reason')).toHaveAttribute('aria-invalid', 'true');
			expect(page.getByRole('button', { name: 'Confirm and reverse' }).all()).toHaveLength(0);
		});

		it('calls onReversePayment with the Invoice id, Payment id, and reason once confirmed', async () => {
			const { onReversePayment } = await setup({ invoices: [invoicePaidByHand], isOwnerOrAdmin: true });

			await page.getByRole('button', { name: 'Reverse payment' }).click();
			await page.getByLabelText('Reason').fill('Logged against the wrong Invoice');
			await page.getByRole('button', { name: 'Continue' }).click();
			await expect.element(page.getByText('Logged against the wrong Invoice')).toBeVisible();
			await page.getByRole('button', { name: 'Confirm and reverse' }).click();

			expect(onReversePayment).toHaveBeenCalledWith(
				invoicePaidByHand.id,
				invoicePaidByHand.activePaymentId,
				'Logged against the wrong Invoice'
			);
		});

		it('returns to the form on Change', async () => {
			await setup({ invoices: [invoicePaidByHand], isOwnerOrAdmin: true });

			await page.getByRole('button', { name: 'Reverse payment' }).click();
			await page.getByLabelText('Reason').fill('Logged against the wrong Invoice');
			await page.getByRole('button', { name: 'Continue' }).click();
			await page.getByRole('button', { name: 'Change' }).click();

			await expect.element(page.getByRole('button', { name: 'Continue' })).toBeVisible();
		});

		it('shows an error when onReversePayment throws', async () => {
			const onReversePayment = vi.fn().mockRejectedValue(new Error('This Payment has already been reversed.'));
			await setup({ invoices: [invoicePaidByHand], isOwnerOrAdmin: true, onReversePayment });

			await page.getByRole('button', { name: 'Reverse payment' }).click();
			await page.getByLabelText('Reason').fill('reason');
			await page.getByRole('button', { name: 'Continue' }).click();
			await page.getByRole('button', { name: 'Confirm and reverse' }).click();

			await expect.element(page.getByText('This Payment has already been reversed.')).toBeVisible();
		});

		it('falls back to a generic message when onReversePayment rejects with a non-Error', async () => {
			const onReversePayment = vi.fn().mockRejectedValue('boom');
			await setup({ invoices: [invoicePaidByHand], isOwnerOrAdmin: true, onReversePayment });

			await page.getByRole('button', { name: 'Reverse payment' }).click();
			await page.getByLabelText('Reason').fill('reason');
			await page.getByRole('button', { name: 'Continue' }).click();
			await page.getByRole('button', { name: 'Confirm and reverse' }).click();

			await expect.element(page.getByText('Failed to reverse this payment')).toBeVisible();
		});

		// #1038: PostReversePaymentHandler's own `reason` details key
		// (#945) is read onto the Reason field, per formErrors.ts's
		// GOV.UK rules, rather than a detached alert.
		it('wires a reason refusal from the BFF onto the Reason field', async () => {
			const onReversePayment = vi
				.fn()
				.mockRejectedValue(
					new RefusalError('reason cannot be blank', {
						reason: 'Enter a reason for reversing this payment'
					})
				);
			await setup({ invoices: [invoicePaidByHand], isOwnerOrAdmin: true, onReversePayment });

			await page.getByRole('button', { name: 'Reverse payment' }).click();
			await page.getByLabelText('Reason').fill('reason');
			await page.getByRole('button', { name: 'Continue' }).click();
			await page.getByRole('button', { name: 'Confirm and reverse' }).click();

			await expect.element(page.getByText('Enter a reason for reversing this payment')).toBeVisible();
			await expect.element(page.getByLabelText('Reason')).toHaveAttribute('aria-invalid', 'true');
		});

		// #1038's second shape: an already-reversed conflict names no
		// field of this form, so it still falls back to the untargeted
		// role="alert" summary, per docs/api-design.md section 7.
		it('falls back to the untargeted summary for a RefusalError with no details at all', async () => {
			const onReversePayment = vi
				.fn()
				.mockRejectedValue(new RefusalError('This Payment has already been reversed.'));
			await setup({ invoices: [invoicePaidByHand], isOwnerOrAdmin: true, onReversePayment });

			await page.getByRole('button', { name: 'Reverse payment' }).click();
			await page.getByLabelText('Reason').fill('reason');
			await page.getByRole('button', { name: 'Continue' }).click();
			await page.getByRole('button', { name: 'Confirm and reverse' }).click();

			await expect.element(page.getByText('This Payment has already been reversed.')).toBeVisible();
			// Stays on the review step: an untargeted refusal has no field
			// to send the reader back to, unlike the targeted refusal above.
			await expect.element(page.getByLabelText('Reason')).not.toBeInTheDocument();
		});

		it('falls back to the untargeted summary when a RefusalError\'s details name no field of this form', async () => {
			const onReversePayment = vi
				.fn()
				.mockRejectedValue(new RefusalError('refused', { someUnrelatedField: 'x' }));
			await setup({ invoices: [invoicePaidByHand], isOwnerOrAdmin: true, onReversePayment });

			await page.getByRole('button', { name: 'Reverse payment' }).click();
			await page.getByLabelText('Reason').fill('reason');
			await page.getByRole('button', { name: 'Continue' }).click();
			await page.getByRole('button', { name: 'Confirm and reverse' }).click();

			await expect.element(page.getByText('refused')).toBeVisible();
		});

		it('closes the record-payment form when Reverse payment is started', async () => {
			const openByHandForReversal: Invoice = { ...invoiceOpen, id: 'inv-4', billingMode: 'by_hand' };
			await setup({ invoices: [openByHandForReversal, invoicePaidByHand], isOwnerOrAdmin: true });

			await page.getByRole('button', { name: 'Record payment' }).click();
			await expect.element(page.getByRole('heading', { name: /Record a payment/ })).toBeVisible();

			await page.getByRole('button', { name: 'Reverse payment' }).click();

			expect(page.getByRole('heading', { name: /Record a payment/ }).all()).toHaveLength(0);
			await expect.element(page.getByRole('heading', { name: /Reverse a payment/ })).toBeVisible();
		});
	});

	describe('return money to the Client (#1009)', () => {
		it('offers Return money on a paid Invoice on either rail, and Reverse payment only on the by-hand one', async () => {
			await setup({ invoices: [invoicePaidByCard, invoicePaidByHand], isOwnerOrAdmin: true });

			expect(page.getByRole('button', { name: 'Return money' }).all()).toHaveLength(2);
			expect(page.getByRole('button', { name: 'Reverse payment' }).all()).toHaveLength(1);
		});

		it('hides Return money from anyone but Owner or Admin', async () => {
			await setup({ invoices: [invoicePaidByCard, invoicePaidByHand], isOwnerOrAdmin: false });

			expect(page.getByRole('button', { name: 'Return money' }).all()).toHaveLength(0);
		});

		it('says how much went back on a partly refunded Invoice, keeps it Paid, and stops offering Reverse payment', async () => {
			const partlyRefunded: Invoice = { ...invoicePaidByHand, refundedCents: 10_000 };
			await setup({ invoices: [partlyRefunded], isOwnerOrAdmin: true });

			await expect.element(invoiceFact('$100.00')).toBeVisible();
			await expect.element(invoiceFact('Paid')).toBeVisible();
			await expect.element(page.getByRole('button', { name: 'Return money' })).toBeVisible();
			expect(page.getByRole('button', { name: 'Reverse payment' }).all()).toHaveLength(0);
		});

		it('stops offering Return money once everything has gone back', async () => {
			const fullyRefunded: Invoice = { ...invoicePaidByHand, refundedCents: invoicePaidByHand.amountCents };
			await setup({ invoices: [fullyRefunded], isOwnerOrAdmin: true });

			await expect.element(invoiceFact('$250.00')).toBeVisible();
			expect(page.getByRole('button', { name: 'Return money' }).all()).toHaveLength(0);
		});

		it('calls onRefundPayment with the Invoice id, Payment id, and what was entered, then closes the form', async () => {
			const { onRefundPayment } = await setup({ invoices: [invoicePaidByHand], isOwnerOrAdmin: true });

			await page.getByRole('button', { name: 'Return money' }).click();
			await page.getByRole('radio', { name: 'Check' }).click();
			await page.getByLabelText('Amount to return (USD)').fill('100');
			await page.getByRole('button', { name: 'Continue' }).click();
			await page.getByRole('button', { name: 'Confirm and return money' }).click();

			expect(onRefundPayment).toHaveBeenCalledWith(invoicePaidByHand.id, invoicePaidByHand.activePaymentId, {
				amountCents: 10_000,
				method: 'check',
				note: undefined
			});
			await expect.element(page.getByRole('region', { name: 'Return money to the Client' })).not.toBeInTheDocument();
		});

		it('keeps the form open and shows why when onRefundPayment throws', async () => {
			const onRefundPayment = vi.fn().mockRejectedValue(new Error('That is more than is left to return on this Payment.'));
			await setup({ invoices: [invoicePaidByHand], isOwnerOrAdmin: true, onRefundPayment });

			await page.getByRole('button', { name: 'Return money' }).click();
			await page.getByRole('radio', { name: 'Check' }).click();
			await page.getByLabelText('Amount to return (USD)').fill('100');
			await page.getByRole('button', { name: 'Continue' }).click();
			await page.getByRole('button', { name: 'Confirm and return money' }).click();

			await expect.element(page.getByText('That is more than is left to return on this Payment.')).toBeVisible();
		});

		it('keeps one form open at a time across Return money, Reverse payment, and Record payment', async () => {
			const openByHand: Invoice = { ...invoiceOpen, id: 'inv-5', billingMode: 'by_hand', reference: 'INV-0005' };
			await setup({ invoices: [openByHand, invoicePaidByHand], isOwnerOrAdmin: true });
			const refundForm = page.getByRole('region', { name: 'Return money to the Client' });

			await page.getByRole('button', { name: 'Return money' }).click();
			await expect.element(refundForm).toBeVisible();

			await page.getByRole('button', { name: 'Reverse payment' }).click();
			await expect.element(refundForm).not.toBeInTheDocument();

			await page.getByRole('button', { name: 'Return money' }).click();
			await page.getByRole('button', { name: 'Record payment' }).click();
			await expect.element(refundForm).not.toBeInTheDocument();

			await page.getByRole('button', { name: 'Return money' }).click();
			await page.getByRole('button', { name: 'Cancel' }).click();
			await expect.element(refundForm).not.toBeInTheDocument();
		});
	});
});

// A contractor Doula reads each Invoice's date and status and never its
// money (#2094, ADR-0008's amendment of 2026-10-26), and may still raise an
// Invoice (its write table). Her rows come in a shape of their own, so the
// section draws them from `contractorInvoices`, never from `invoices`.
const contractorLate: ContractorInvoice = {
	id: 'inv-late',
	status: 'open',
	raisedOn: '2026-08-01',
	dueOn: '2026-08-31',
	daysOverdue: 12
};

const contractorPaid: ContractorInvoice = {
	id: 'inv-paid',
	status: 'paid',
	raisedOn: '2026-07-01',
	dueOn: '2026-07-31',
	paidOn: '2026-07-20',
	daysOverdue: 0
};

describe('InvoiceSection.svelte for a contractor Doula (#2002, #2094)', () => {
	it('says what she sees and what she does not, as information and not as an error', async () => {
		await setup({ canReadInvoiceMoney: false });

		const statement = page.getByRole('status').filter({ hasText: 'You see when each Invoice was raised' });
		await expect.element(statement).toBeVisible();
		await expect
			.element(statement)
			.toHaveTextContent(
				'You see when each Invoice was raised and where it stands, not what the Client pays. Your own fee is in the Contract section.'
			);
		await expect.element(page.getByRole('alert')).not.toBeInTheDocument();
	});

	it('lists each Invoice by the day it was raised and where it stands, Overdue once it is late', async () => {
		await setup({ canReadInvoiceMoney: false, contractorInvoices: [contractorLate, contractorPaid] });

		await expect.element(invoiceTitle('Aug 1, 2026')).toBeVisible();
		await expect.element(invoiceFact('Overdue')).toBeVisible();
		await expect.element(invoiceFact('Aug 31, 2026')).toBeVisible();
		await expect.element(invoiceTitle('Jul 1, 2026')).toBeVisible();
		await expect.element(invoiceFact('Paid')).toBeVisible();
		await expect.element(invoiceFact('Jul 20, 2026')).toBeVisible();
	});

	it('draws no money and no Invoice action, even when money-bearing Invoices are passed', async () => {
		await setup({
			canReadInvoiceMoney: false,
			isOwnerOrAdmin: true,
			invoices: [invoiceOpen, invoicePaid],
			contractorInvoices: [contractorLate]
		});

		await expect.element(invoiceTitle('Aug 1, 2026')).toBeVisible();
		await expect.element(page.getByText(/\$/)).not.toBeInTheDocument();
		await expect.element(page.getByRole('button', { name: 'Record payment' })).not.toBeInTheDocument();
	});

	it('says "No Invoices yet." when there are none', async () => {
		await setup({ canReadInvoiceMoney: false });

		await expect.element(page.getByText('No Invoices yet.')).toBeVisible();
	});

	it('still offers Create Invoice, which ADR-0008 lets her raise, and says when it worked', async () => {
		const { onCreate } = await setup({ canReadInvoiceMoney: false });

		await page.getByRole('button', { name: 'Create Invoice' }).click();

		expect(onCreate).toHaveBeenCalledWith();
		const created = page.getByText('Invoice created.');
		await expect.element(created).toBeVisible();
		await expect.element(created).toHaveFocus();
	});

	it('says nothing was created when the create is refused', async () => {
		await setup({
			canReadInvoiceMoney: false,
			onCreate: vi.fn().mockRejectedValue(new Error('This Contract is not signed.'))
		});

		await page.getByRole('button', { name: 'Create Invoice' }).click();

		await expect.element(page.getByText('This Contract is not signed.')).toBeVisible();
		await expect.element(page.getByText('Invoice created.')).not.toBeInTheDocument();
	});

	it('asks the billing-mode question on the first Invoice, as for anyone who raises it, once asked', async () => {
		await setupAsked({ canReadInvoiceMoney: false, billingMode: 'unset' });

		await expect.element(page.getByRole('group', { name: /How does this Practice bill Clients/ })).toBeVisible();
	});
});
