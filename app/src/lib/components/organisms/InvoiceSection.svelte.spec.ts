import { page } from 'vitest/browser';
import { describe, expect, it, vi } from 'vitest';
import { render } from 'vitest-browser-svelte';
import InvoiceSection from './InvoiceSection.svelte';
import type { Invoice } from '#lib/invoice.js';

interface SetupOptions {
	invoices?: Invoice[];
	contractStatus?: string;
	clientsCanPay?: boolean;
	hasClientEmail?: boolean;
	isOwner?: boolean;
	onCreate?: (amountCents: number) => Promise<void>;
}

const paymentsSettingsHref = 'https://example.test/practices/practice-1/settings/payments';

const invoiceOpen: Invoice = {
	id: 'inv-1',
	contractId: 'contract-1',
	status: 'open',
	amountCents: 15_000,
	currency: 'usd',
	createdAt: '2026-01-01T00:00:00Z'
};

const invoicePaid: Invoice = {
	id: 'inv-2',
	contractId: 'contract-1',
	status: 'paid',
	amountCents: 20_000,
	currency: 'usd',
	createdAt: '2026-01-02T00:00:00Z',
	paidAt: '2026-01-05T00:00:00Z'
};

async function setup({
	invoices = [],
	contractStatus = 'signed',
	clientsCanPay = true,
	hasClientEmail = true,
	isOwner = false,
	onCreate = vi.fn().mockResolvedValue(undefined)
}: SetupOptions = {}) {
	await render(InvoiceSection, {
		invoices,
		contractStatus,
		clientsCanPay,
		hasClientEmail,
		isOwner,
		paymentsSettingsHref,
		onCreate
	});
	return { onCreate };
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

	it('shows the amount form when the Contract is billable, Clients can pay, and the Client has an email', async () => {
		await setup();

		await expect.element(page.getByLabelText('Amount (USD)')).toBeInTheDocument();
		await expect.element(page.getByRole('button', { name: 'Create Invoice' })).toBeInTheDocument();
	});

	it('calls onCreate with the amount converted to cents and clears the field', async () => {
		const { onCreate } = await setup();

		await page.getByLabelText('Amount (USD)').fill('150.5');
		await page.getByRole('button', { name: 'Create Invoice' }).click();

		expect(onCreate).toHaveBeenCalledWith(15_050);
		// jest-dom's toHaveValue treats an empty number input's value as
		// null, not '' -- see https://github.com/testing-library/jest-dom#tohavevalue
		// eslint-disable-next-line unicorn/no-null
		await expect.element(page.getByLabelText('Amount (USD)')).toHaveValue(null);
	});

	it('rejects a zero amount without calling onCreate', async () => {
		const { onCreate } = await setup();

		await page.getByLabelText('Amount (USD)').fill('0');
		await page.getByRole('button', { name: 'Create Invoice' }).click();

		expect(onCreate).not.toHaveBeenCalled();
		await expect.element(page.getByText('Enter an amount greater than zero')).toBeInTheDocument();
	});

	it('shows an error when onCreate throws', async () => {
		const onCreate = vi.fn().mockRejectedValue(new Error('amountCents must be greater than zero'));
		await setup({ onCreate });

		await page.getByLabelText('Amount (USD)').fill('50');
		await page.getByRole('button', { name: 'Create Invoice' }).click();

		await expect.element(page.getByText('amountCents must be greater than zero')).toBeInTheDocument();
	});

	it('falls back to a generic message when onCreate rejects with a non-Error', async () => {
		const onCreate = vi.fn().mockRejectedValue('boom');
		await setup({ onCreate });

		await page.getByLabelText('Amount (USD)').fill('50');
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
		await expect.element(page.getByRole('link', { name: 'Go to Payments settings' })).toBeInTheDocument();
		await expect.element(page.getByLabelText('Amount (USD)')).not.toBeInTheDocument();
	});

	it('shows the cannot-pay Notice with no link for a non-Owner when Clients cannot pay', async () => {
		await setup({ clientsCanPay: false, isOwner: false });

		await expect
			.element(page.getByText('Clients cannot pay this Practice yet. A Practice Owner has to connect Stripe.'))
			.toBeVisible();
		await expect.element(page.getByRole('link', { name: 'Go to Payments settings' })).not.toBeInTheDocument();
		await expect.element(page.getByLabelText('Amount (USD)')).not.toBeInTheDocument();
	});

	it('shows a Notice naming the missing email instead of the form when the Client has no email', async () => {
		await setup({ hasClientEmail: false });

		await expect
			.element(page.getByText('This Client has no email address on file. Add one before creating an Invoice.'))
			.toBeVisible();
		await expect.element(page.getByLabelText('Amount (USD)')).not.toBeInTheDocument();
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
		await expect.element(page.getByLabelText('Amount (USD)')).not.toBeInTheDocument();
	});

	it('shows why billing is unavailable instead of the form on a sent (unsigned) Contract', async () => {
		await setup({ contractStatus: 'sent' });

		await expect
			.element(page.getByText('Invoicing is unavailable until the Client signs this Contract.'))
			.toBeVisible();
		await expect.element(page.getByLabelText('Amount (USD)')).not.toBeInTheDocument();
	});

	it('shows a voided-specific message instead of the form on a voided Contract', async () => {
		await setup({ contractStatus: 'voided' });

		await expect
			.element(
				page.getByText('This Contract has been voided. Invoicing is unavailable until Staff issues a new Contract.')
			)
			.toBeVisible();
		await expect.element(page.getByLabelText('Amount (USD)')).not.toBeInTheDocument();
	});

	it('takes priority over the cannot-pay Notice on an unbillable Contract', async () => {
		await setup({ contractStatus: 'voided', clientsCanPay: false, isOwner: true });

		await expect.element(page.getByRole('link', { name: 'Go to Payments settings' })).not.toBeInTheDocument();
		await expect.element(page.getByText(/This Contract has been voided/)).toBeVisible();
	});
});
