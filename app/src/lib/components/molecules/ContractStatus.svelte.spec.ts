import { page } from 'vitest/browser';
import { describe, expect, it, vi } from 'vitest';
import { render } from 'vitest-browser-svelte';
import ContractStatus from './ContractStatus.svelte';
import type { VoidRequestSummary } from '#lib/contract.js';

interface SetupOptions {
	status?: string;
	amountChangedAt?: string;
	voidRequests?: VoidRequestSummary[];
	onVoid?: () => Promise<void>;
	onDownloadPdf?: () => Promise<void>;
	onRequestVoid?: (reason: string) => Promise<void>;
	onDeclineVoidRequest?: (requestId: string, reason: string) => Promise<void>;
}

async function setup({
	status = 'draft',
	amountChangedAt,
	voidRequests,
	onVoid,
	onDownloadPdf,
	onRequestVoid,
	onDeclineVoidRequest
}: SetupOptions = {}) {
	await render(ContractStatus, {
		status,
		amountChangedAt,
		voidRequests,
		onVoid,
		onDownloadPdf,
		onRequestVoid,
		onDeclineVoidRequest
	});
}

function openVoidRequest(overrides: Partial<VoidRequestSummary> = {}): VoidRequestSummary {
	return {
		id: 'request-1',
		requestedBy: 'staff-1',
		requestedByName: 'Jamie Doula',
		reason: 'the client rescheduled',
		status: 'open',
		createdAt: '2026-01-01T00:00:00Z',
		...overrides
	};
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

	it('shows no price-changed notice when amountChangedAt is absent', async () => {
		await setup({ status: 'draft' });

		await expect.element(page.getByText('Price changed on')).not.toBeInTheDocument();
	});

	it('shows a price-changed notice, with the machine-readable instant, when amountChangedAt is set (#968)', async () => {
		await setup({ status: 'draft', amountChangedAt: '2026-09-08T12:00:00Z' });

		await expect.element(page.getByText('Price changed on')).toBeInTheDocument();
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

	it('offers no PDF download when there is no onDownloadPdf callback (a role the endpoint refuses)', async () => {
		await setup({ status: 'signed', onVoid: vi.fn() });

		await expect
			.element(page.getByRole('button', { name: 'Download signed Contract (PDF)' }))
			.not.toBeInTheDocument();
	});

	it('offers no PDF download on a draft or sent Contract even with onDownloadPdf', async () => {
		await setup({ status: 'sent', onDownloadPdf: vi.fn() });

		await expect
			.element(page.getByRole('button', { name: 'Download signed Contract (PDF)' }))
			.not.toBeInTheDocument();
	});

	it('offers no PDF download on a voided Contract even with onDownloadPdf', async () => {
		await setup({ status: 'voided', onDownloadPdf: vi.fn() });

		await expect
			.element(page.getByRole('button', { name: 'Download signed Contract (PDF)' }))
			.not.toBeInTheDocument();
	});

	it('offers the PDF download on a signed Contract when onDownloadPdf is provided', async () => {
		await setup({ status: 'signed', onDownloadPdf: vi.fn() });

		await expect
			.element(page.getByRole('button', { name: 'Download signed Contract (PDF)' }))
			.toBeInTheDocument();
	});

	it('calls onDownloadPdf when the download action is clicked', async () => {
		const onDownloadPdf = vi.fn().mockResolvedValue(undefined);
		await setup({ status: 'signed', onDownloadPdf });

		await page.getByRole('button', { name: 'Download signed Contract (PDF)' }).click();

		expect(onDownloadPdf).toHaveBeenCalledOnce();
	});

	it('shows an error message if onDownloadPdf rejects', async () => {
		const onDownloadPdf = vi.fn().mockRejectedValue(new Error('signed PDF not found'));
		await setup({ status: 'signed', onDownloadPdf });

		await page.getByRole('button', { name: 'Download signed Contract (PDF)' }).click();

		await expect.element(page.getByRole('alert')).toHaveTextContent('signed PDF not found');
	});

	it('shows a fallback error message if onDownloadPdf rejects with a non-Error value', async () => {
		const onDownloadPdf = vi.fn().mockRejectedValue('boom');
		await setup({ status: 'signed', onDownloadPdf });

		await page.getByRole('button', { name: 'Download signed Contract (PDF)' }).click();

		await expect.element(page.getByRole('alert')).toHaveTextContent('Failed to download signed Contract');
	});

	describe('Request void (#971, a Doula asking instead of voiding herself)', () => {
		it('offers no Request void action when there is no onRequestVoid callback', async () => {
			await setup({ status: 'signed' });

			await expect.element(page.getByRole('button', { name: 'Request void' })).not.toBeInTheDocument();
		});

		it('offers no Request void action on a draft, sent, or voided Contract', async () => {
			await setup({ status: 'draft', onRequestVoid: vi.fn() });

			await expect.element(page.getByRole('button', { name: 'Request void' })).not.toBeInTheDocument();
		});

		it('offers the Request void action on a signed Contract when onRequestVoid is provided', async () => {
			await setup({ status: 'signed', onRequestVoid: vi.fn() });

			await expect.element(page.getByRole('button', { name: 'Request void' })).toBeVisible();
		});

		it('reveals a reason field behind the Request void control', async () => {
			await setup({ status: 'signed', onRequestVoid: vi.fn() });

			await expect.element(page.getByLabelText('Reason')).not.toBeInTheDocument();
			await page.getByRole('button', { name: 'Request void' }).click();
			await expect.element(page.getByLabelText('Reason')).toBeVisible();
		});

		it('shows a field error and calls nothing when the reason is left blank', async () => {
			const onRequestVoid = vi.fn();
			await setup({ status: 'signed', onRequestVoid });

			await page.getByRole('button', { name: 'Request void' }).click();
			await page.getByRole('button', { name: 'Send request' }).click();

			await expect
				.element(page.getByRole('alert'))
				.toHaveTextContent('Enter why this contract needs to be voided');
			expect(onRequestVoid).not.toHaveBeenCalled();
		});

		it('calls onRequestVoid with the typed reason and closes the form on success', async () => {
			const onRequestVoid = vi.fn().mockResolvedValue(undefined);
			await setup({ status: 'signed', onRequestVoid });

			await page.getByRole('button', { name: 'Request void' }).click();
			await page.getByLabelText('Reason').fill('the client rescheduled');
			await page.getByRole('button', { name: 'Send request' }).click();

			expect(onRequestVoid).toHaveBeenCalledWith('the client rescheduled');
			await expect.element(page.getByLabelText('Reason')).not.toBeInTheDocument();
		});

		it('canceling the form hides it without calling onRequestVoid', async () => {
			const onRequestVoid = vi.fn();
			await setup({ status: 'signed', onRequestVoid });

			await page.getByRole('button', { name: 'Request void' }).click();
			await page.getByRole('button', { name: 'Cancel' }).click();

			await expect.element(page.getByLabelText('Reason')).not.toBeInTheDocument();
			expect(onRequestVoid).not.toHaveBeenCalled();
		});

		it('shows an error message if onRequestVoid rejects', async () => {
			const onRequestVoid = vi.fn().mockRejectedValue(new Error('a void may only be requested for a signed contract'));
			await setup({ status: 'signed', onRequestVoid });

			await page.getByRole('button', { name: 'Request void' }).click();
			await page.getByLabelText('Reason').fill('asking');
			await page.getByRole('button', { name: 'Send request' }).click();

			await expect
				.element(page.getByRole('alert'))
				.toHaveTextContent('a void may only be requested for a signed contract');
		});

		it('shows a fallback error message if onRequestVoid rejects with a non-Error value', async () => {
			const onRequestVoid = vi.fn().mockRejectedValue('boom');
			await setup({ status: 'signed', onRequestVoid });

			await page.getByRole('button', { name: 'Request void' }).click();
			await page.getByLabelText('Reason').fill('asking');
			await page.getByRole('button', { name: 'Send request' }).click();

			await expect.element(page.getByRole('alert')).toHaveTextContent('Failed to request a void');
		});

		it('shows a waiting notice instead of the control once a request is open', async () => {
			await setup({ status: 'signed', onRequestVoid: vi.fn(), voidRequests: [openVoidRequest()] });

			await expect.element(page.getByRole('button', { name: 'Request void' })).not.toBeInTheDocument();
			await expect.element(page.getByRole('status')).toHaveTextContent('waiting for an owner or admin');
		});

		it('shows a declined request as its own notice, with the decliner’s own reason', async () => {
			await setup({
				status: 'signed',
				onRequestVoid: vi.fn(),
				voidRequests: [openVoidRequest({ status: 'declined', declineReason: 'not yet' })]
			});

			await expect
				.element(page.getByText('Void request declined: not yet'))
				.toBeVisible();
		});
	});

	describe('Decline a void request (#971, the Owner/Admin side)', () => {
		it('offers no Decline action when there is no onDeclineVoidRequest callback', async () => {
			await setup({ status: 'signed', onVoid: vi.fn(), voidRequests: [openVoidRequest()] });

			await expect.element(page.getByRole('button', { name: 'Decline' })).not.toBeInTheDocument();
		});

		it('names who asked and why, and offers Decline, for each open request', async () => {
			await setup({
				status: 'signed',
				onVoid: vi.fn(),
				onDeclineVoidRequest: vi.fn(),
				voidRequests: [openVoidRequest()]
			});

			await expect
				.element(page.getByText('Jamie Doula asked to void this contract: the client rescheduled'))
				.toBeVisible();
			await expect.element(page.getByRole('button', { name: 'Decline' })).toBeVisible();
		});

		it('reveals a reason field behind the Decline control', async () => {
			await setup({
				status: 'signed',
				onDeclineVoidRequest: vi.fn(),
				voidRequests: [openVoidRequest()]
			});

			await page.getByRole('button', { name: 'Decline' }).click();

			await expect.element(page.getByLabelText('Reason for declining')).toBeVisible();
		});

		it('shows a field error and calls nothing when the decline reason is left blank', async () => {
			const onDeclineVoidRequest = vi.fn();
			await setup({
				status: 'signed',
				onDeclineVoidRequest,
				voidRequests: [openVoidRequest()]
			});

			await page.getByRole('button', { name: 'Decline' }).click();
			await page.getByRole('button', { name: 'Decline' }).click();

			await expect
				.element(page.getByRole('alert'))
				.toHaveTextContent('Enter why this request is being declined');
			expect(onDeclineVoidRequest).not.toHaveBeenCalled();
		});

		it('calls onDeclineVoidRequest with the request id and typed reason on success', async () => {
			const onDeclineVoidRequest = vi.fn().mockResolvedValue(undefined);
			await setup({
				status: 'signed',
				onDeclineVoidRequest,
				voidRequests: [openVoidRequest({ id: 'request-9' })]
			});

			await page.getByRole('button', { name: 'Decline' }).click();
			await page.getByLabelText('Reason for declining').fill('not yet');
			await page.getByRole('button', { name: 'Decline' }).click();

			expect(onDeclineVoidRequest).toHaveBeenCalledWith('request-9', 'not yet');
		});

		it('canceling the decline form hides it without calling onDeclineVoidRequest', async () => {
			const onDeclineVoidRequest = vi.fn();
			await setup({
				status: 'signed',
				onDeclineVoidRequest,
				voidRequests: [openVoidRequest()]
			});

			await page.getByRole('button', { name: 'Decline' }).click();
			await page.getByRole('button', { name: 'Cancel' }).click();

			await expect.element(page.getByLabelText('Reason for declining')).not.toBeInTheDocument();
			expect(onDeclineVoidRequest).not.toHaveBeenCalled();
		});

		it('shows an error message if onDeclineVoidRequest rejects', async () => {
			const onDeclineVoidRequest = vi
				.fn()
				.mockRejectedValue(new Error('no open void request found with this id for this contract'));
			await setup({
				status: 'signed',
				onDeclineVoidRequest,
				voidRequests: [openVoidRequest()]
			});

			await page.getByRole('button', { name: 'Decline' }).click();
			await page.getByLabelText('Reason for declining').fill('not yet');
			await page.getByRole('button', { name: 'Decline' }).click();

			await expect
				.element(page.getByRole('alert'))
				.toHaveTextContent('no open void request found with this id for this contract');
		});

		it('shows a fallback error message if onDeclineVoidRequest rejects with a non-Error value', async () => {
			const onDeclineVoidRequest = vi.fn().mockRejectedValue('boom');
			await setup({
				status: 'signed',
				onDeclineVoidRequest,
				voidRequests: [openVoidRequest()]
			});

			await page.getByRole('button', { name: 'Decline' }).click();
			await page.getByLabelText('Reason for declining').fill('not yet');
			await page.getByRole('button', { name: 'Decline' }).click();

			await expect.element(page.getByRole('alert')).toHaveTextContent('Failed to decline this request');
		});
	});
});
