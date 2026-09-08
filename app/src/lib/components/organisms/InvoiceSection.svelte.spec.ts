import { page } from 'vitest/browser';
import { describe, expect, it, vi } from 'vitest';
import { render } from 'vitest-browser-svelte';
import InvoiceSection from './InvoiceSection.svelte';
import type { BillingMode, Invoice, PaymentMethod } from '#lib/invoice.js';

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
	onCreate?: (billingMode?: BillingMode) => Promise<void>;
	onRecordPayment?: (
		invoiceId: string,
		input: { method: PaymentMethod; note?: string; paidOn: string }
	) => Promise<void>;
	onVoidInvoice?: (invoiceId: string) => Promise<void>;
	onWriteOffInvoice?: (invoiceId: string) => Promise<void>;
}

const paymentsSettingsHref = 'https://example.test/practices/practice-1/settings/payments';

const invoiceOpen: Invoice = {
	id: 'inv-1',
	contractId: 'contract-1',
	status: 'open',
	amountCents: 15_000,
	currency: 'usd',
	createdAt: '2026-01-01T00:00:00Z',
	reference: 'STRIPE-in_1',
	billingMode: 'stripe'
};

const invoicePaid: Invoice = {
	id: 'inv-2',
	contractId: 'contract-1',
	status: 'paid',
	amountCents: 20_000,
	currency: 'usd',
	createdAt: '2026-01-02T00:00:00Z',
	paidAt: '2026-01-05T00:00:00Z',
	reference: 'STRIPE-in_2',
	billingMode: 'stripe'
};

async function setup({
	invoices = [],
	contractStatus = 'signed',
	billingMode = 'stripe',
	clientsCanPay = true,
	hasClientEmail = true,
	isOwner = false,
	isOwnerOrAdmin = false,
	onCreate = vi.fn().mockResolvedValue(undefined),
	onRecordPayment = vi.fn().mockResolvedValue(undefined),
	onVoidInvoice = vi.fn().mockResolvedValue(undefined),
	onWriteOffInvoice = vi.fn().mockResolvedValue(undefined)
}: SetupOptions = {}) {
	await render(InvoiceSection, {
		invoices,
		contractStatus,
		billingMode: billingMode === 'unset' ? undefined : billingMode,
		clientsCanPay,
		hasClientEmail,
		isOwner,
		isOwnerOrAdmin,
		paymentsSettingsHref,
		onCreate,
		onRecordPayment,
		onVoidInvoice,
		onWriteOffInvoice
	});
	return { onCreate, onRecordPayment, onVoidInvoice, onWriteOffInvoice };
}

describe('InvoiceSection.svelte', () => {
	it('shows "No Invoices yet." when the list is empty', async () => {
		await setup();

		await expect.element(page.getByText('No Invoices yet.')).toBeInTheDocument();
	});

	it('lists an Invoice with its formatted amount and status', async () => {
		await setup({ invoices: [invoiceOpen] });

		await expect.element(page.getByText('$150.00 — Open')).toBeInTheDocument();
	});

	it('shows a paid Invoice with its paid date', async () => {
		await setup({ invoices: [invoicePaid] });

		const expectedDate = new Date(invoicePaid.paidAt!).toLocaleDateString();
		await expect.element(page.getByText(/\$200\.00 — Paid/)).toBeInTheDocument();
		await expect.element(page.getByText(`(paid ${expectedDate})`)).toBeInTheDocument();
	});

	it('falls back to the raw status string for a status with no known label', async () => {
		await setup({ invoices: [{ ...invoiceOpen, status: 'unknown_status' }] });

		await expect.element(page.getByText('$150.00 — unknown_status')).toBeInTheDocument();
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

	it('shows an error when onCreate throws', async () => {
		const onCreate = vi.fn().mockRejectedValue(new Error('engagement not found'));
		await setup({ onCreate });

		await page.getByRole('button', { name: 'Create Invoice' }).click();

		await expect.element(page.getByText('engagement not found')).toBeInTheDocument();
	});

	it('falls back to a generic message when onCreate rejects with a non-Error', async () => {
		const onCreate = vi.fn().mockRejectedValue('boom');
		await setup({ onCreate });

		await page.getByRole('button', { name: 'Create Invoice' }).click();

		await expect.element(page.getByText('Failed to create invoice')).toBeInTheDocument();
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
		it('shows the billing-mode ask with neither cannot-pay Notice', async () => {
			await setup({ billingMode: 'unset', clientsCanPay: false, hasClientEmail: false });

			await expect.element(page.getByText('How does this Practice bill Clients?')).toBeVisible();
			await expect.element(page.getByLabelText('Stripe')).toBeVisible();
			await expect.element(page.getByLabelText('By hand')).toBeVisible();
			await expect.element(page.getByRole('button', { name: 'Create Invoice' })).toBeVisible();
			await expect.element(page.getByText(/Clients cannot pay this Practice/)).not.toBeInTheDocument();
		});

		it('submits the chosen mode on the first raise', async () => {
			const { onCreate } = await setup({ billingMode: 'unset' });

			await page.getByLabelText('By hand').click();
			await page.getByRole('button', { name: 'Create Invoice' }).click();

			expect(onCreate).toHaveBeenCalledWith('by_hand');
		});

		it('defaults to Stripe when submitted without changing the radio', async () => {
			const { onCreate } = await setup({ billingMode: 'unset' });

			await page.getByRole('button', { name: 'Create Invoice' }).click();

			expect(onCreate).toHaveBeenCalledWith('stripe');
		});

		it('shows an error when onCreate throws from the billing-mode ask', async () => {
			const onCreate = vi.fn().mockRejectedValue(new Error('billing mode invalid'));
			await setup({ billingMode: 'unset', onCreate });

			await page.getByRole('button', { name: 'Create Invoice' }).click();

			await expect.element(page.getByText('billing mode invalid')).toBeVisible();
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

		it('requires a note when the method is Other', async () => {
			await setup({ invoices: [invoiceOpen], isOwnerOrAdmin: true });

			await page.getByRole('button', { name: 'Record payment' }).click();
			await page.getByLabelText('Other').click();
			await page.getByLabelText('Date received').fill('2026-01-02');
			await page.getByRole('button', { name: 'Continue' }).click();

			await expect.element(page.getByText('Enter a note for "Other"')).toBeVisible();
		});

		it('rejects a date in the future', async () => {
			await setup({ invoices: [invoiceOpen], isOwnerOrAdmin: true });

			const future = new Date();
			future.setDate(future.getDate() + 1);

			await page.getByRole('button', { name: 'Record payment' }).click();
			await page.getByLabelText('Date received').fill(future.toISOString().slice(0, 10));
			await page.getByRole('button', { name: 'Continue' }).click();

			await expect.element(page.getByText('The date cannot be in the future')).toBeVisible();
		});

		it('shows an error when onRecordPayment throws', async () => {
			const onRecordPayment = vi
				.fn()
				.mockRejectedValue(new Error('This Invoice is not open, so nothing can be recorded or changed against it.'));
			await setup({ invoices: [invoiceOpen], isOwnerOrAdmin: true, onRecordPayment });

			await page.getByRole('button', { name: 'Record payment' }).click();
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
			await page.getByLabelText('Date received').fill('2026-01-02');
			await page.getByRole('button', { name: 'Continue' }).click();
			await page.getByRole('button', { name: 'Confirm and record' }).click();

			await expect.element(page.getByText('Failed to record payment')).toBeVisible();
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
});
