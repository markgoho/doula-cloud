import { page } from 'vitest/browser';
import { describe, expect, it, vi } from 'vitest';
import { render } from 'vitest-browser-svelte';
import InvoiceSection from './InvoiceSection.svelte';
import type { Invoice } from './invoice.js';

interface SetupOptions {
	invoices?: Invoice[];
	connectGate?: { isOwner: boolean };
	onCreate?: (amountCents: number) => Promise<void>;
	onConnect?: () => Promise<void>;
}

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
	connectGate,
	onCreate = vi.fn().mockResolvedValue(undefined),
	onConnect = vi.fn().mockResolvedValue(undefined)
}: SetupOptions = {}) {
	await render(InvoiceSection, { invoices, connectGate, onCreate, onConnect });
	return { onCreate, onConnect };
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

	it('shows the amount form when not gated', async () => {
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

	it('shows a Connect Stripe button instead of the form when connectGate.isOwner is true', async () => {
		await setup({ connectGate: { isOwner: true } });

		await expect.element(page.getByRole('button', { name: 'Connect Stripe' })).toBeInTheDocument();
		await expect.element(page.getByLabelText('Amount (USD)')).not.toBeInTheDocument();
	});

	it('calls onConnect when the Connect Stripe button is clicked', async () => {
		const { onConnect } = await setup({ connectGate: { isOwner: true } });

		await page.getByRole('button', { name: 'Connect Stripe' }).click();

		expect(onConnect).toHaveBeenCalled();
	});

	it('shows an error when onConnect throws', async () => {
		const onConnect = vi.fn().mockRejectedValue(new Error('some other failure'));
		await setup({ connectGate: { isOwner: true }, onConnect });

		await page.getByRole('button', { name: 'Connect Stripe' }).click();

		await expect.element(page.getByText('some other failure')).toBeInTheDocument();
	});

	it('falls back to a generic message when onConnect rejects with a non-Error', async () => {
		const onConnect = vi.fn().mockRejectedValue('boom');
		await setup({ connectGate: { isOwner: true }, onConnect });

		await page.getByRole('button', { name: 'Connect Stripe' }).click();

		await expect.element(page.getByText('Failed to start Stripe Connect onboarding')).toBeInTheDocument();
	});

	it('shows the static ask-an-Owner message instead of the form or a button when connectGate.isOwner is false', async () => {
		await setup({ connectGate: { isOwner: false } });

		await expect.element(page.getByText('Ask a Practice Owner to connect Stripe.')).toBeInTheDocument();
		await expect.element(page.getByLabelText('Amount (USD)')).not.toBeInTheDocument();
		await expect.element(page.getByRole('button', { name: 'Connect Stripe' })).not.toBeInTheDocument();
	});
});
