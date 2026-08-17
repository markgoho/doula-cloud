import { page } from 'vitest/browser';
import { describe, expect, it, vi } from 'vitest';
import { render } from 'vitest-browser-svelte';
import ContractStatus from './ContractStatus.svelte';

interface SetupOptions {
	status?: string;
	onVoid?: () => Promise<void>;
}

async function setup({ status = 'draft', onVoid }: SetupOptions = {}) {
	await render(ContractStatus, { status, onVoid });
}

describe('ContractStatus.svelte', () => {
	it('renders the status', async () => {
		await setup({ status: 'sent' });

		await expect.element(page.getByText('Status: sent')).toBeInTheDocument();
	});

	it('renders a terminal-state indicator when voided', async () => {
		await setup({ status: 'voided' });

		await expect.element(page.getByRole('status')).toHaveTextContent('no longer active');
	});

	it('renders no terminal-state indicator for a non-voided status', async () => {
		await setup({ status: 'signed' });

		await expect.element(page.getByRole('status')).not.toBeInTheDocument();
	});

	it('offers no Void action when there is no onVoid callback (Client-portal caller)', async () => {
		await setup({ status: 'signed' });

		await expect.element(page.getByRole('button', { name: 'Void Contract' })).not.toBeInTheDocument();
	});

	it('offers no Void action on a voided Contract (no actions implying it is active)', async () => {
		await setup({ status: 'voided', onVoid: vi.fn() });

		await expect.element(page.getByRole('button', { name: 'Void Contract' })).not.toBeInTheDocument();
	});

	it('offers no Void action on a draft or sent Contract', async () => {
		await setup({ status: 'sent', onVoid: vi.fn() });

		await expect.element(page.getByRole('button', { name: 'Void Contract' })).not.toBeInTheDocument();
	});

	it('offers the Void action on a signed Contract when onVoid is provided (Staff caller)', async () => {
		await setup({ status: 'signed', onVoid: vi.fn() });

		await expect.element(page.getByRole('button', { name: 'Void Contract' })).toBeInTheDocument();
	});

	it('calls onVoid when the Void action is clicked', async () => {
		const onVoid = vi.fn().mockResolvedValue(undefined);
		await setup({ status: 'signed', onVoid });

		await page.getByRole('button', { name: 'Void Contract' }).click();

		expect(onVoid).toHaveBeenCalledOnce();
	});

	it('shows an error message if onVoid rejects', async () => {
		const onVoid = vi.fn().mockRejectedValue(new Error('contract is not signed'));
		await setup({ status: 'signed', onVoid });

		await page.getByRole('button', { name: 'Void Contract' }).click();

		await expect.element(page.getByRole('alert')).toHaveTextContent('contract is not signed');
	});

	it('shows a fallback error message if onVoid rejects with a non-Error value', async () => {
		const onVoid = vi.fn().mockRejectedValue('boom');
		await setup({ status: 'signed', onVoid });

		await page.getByRole('button', { name: 'Void Contract' }).click();

		await expect.element(page.getByRole('alert')).toHaveTextContent('Failed to void contract');
	});
});
