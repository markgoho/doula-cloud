import { page } from 'vitest/browser';
import { describe, expect, it, vi } from 'vitest';
import { render } from 'vitest-browser-svelte';
import ContractStatus, { PREVIOUS_CONTRACT_PDF_NOTICE, VOID_CONSEQUENCE } from './ContractStatus.svelte';
import type { VoidRequestSummary } from '#lib/contract.js';

interface SetupOptions {
	status?: string;
	hasSignedPdf?: boolean;
	amountChangedAt?: string;
	voidRequests?: VoidRequestSummary[];
	onVoid?: () => Promise<void>;
	onDownloadPdf?: () => Promise<void>;
	onRequestVoid?: (reason: string) => Promise<void>;
	onDeclineVoidRequest?: (requestId: string, reason: string) => Promise<void>;
}

async function setup({
	status = 'draft',
	hasSignedPdf = false,
	amountChangedAt,
	voidRequests,
	onVoid,
	onDownloadPdf,
	onRequestVoid,
	onDeclineVoidRequest
}: SetupOptions = {}) {
	return render(ContractStatus, {
		status,
		hasSignedPdf,
		amountChangedAt,
		voidRequests,
		onVoid,
		onDownloadPdf,
		onRequestVoid,
		onDeclineVoidRequest
	});
}

// #2004: Void asks again in a dialog before anything is sent.
async function confirmVoid() {
	await page.getByRole('button', { name: 'Void this Contract' }).click();
	await page.getByRole('dialog').getByRole('button', { name: 'Void this Contract' }).click();
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
	it('renders the status as a row, in the Staff words (#2004)', async () => {
		await setup({ status: 'sent' });

		await expect.element(page.getByRole('term')).toHaveTextContent('Status');
		await expect.element(page.getByRole('definition')).toHaveTextContent('Sent, waiting for the Client to sign');
	});

	it('says a voided Contract is no longer active', async () => {
		await setup({ status: 'voided' });

		await expect.element(page.getByText('Voided, no longer active')).toBeVisible();
	});

	it('says when a signed Contract was signed and sent, and by whom (#2104)', async () => {
		await render(ContractStatus, {
			status: 'signed',
			hasSignedPdf: false,
			sent: { at: '2026-08-12T15:00:00Z', by: 'Renata Alvarez' },
			signed: { at: '2026-08-14T15:00:00Z', by: 'Camille Boyd' }
		});

		await expect.element(page.getByText('Signed on Aug 14, 2026')).toBeVisible();
		await expect.element(page.getByText('Signed by')).toBeVisible();
		await expect.element(page.getByText('Camille Boyd', { exact: true })).toBeVisible();
		await expect.element(page.getByText('Aug 12, 2026 by Renata Alvarez')).toBeVisible();
	});

	it('keeps a voided Contract’s signature as a row of its own (#2104)', async () => {
		await render(ContractStatus, {
			status: 'voided',
			hasSignedPdf: true,
			signed: { at: '2026-08-14T15:00:00Z', by: 'Camille Boyd' }
		});

		await expect.element(page.getByText('Voided, no longer active')).toBeVisible();
		await expect.element(page.getByText('Aug 14, 2026 by Camille Boyd')).toBeVisible();
	});

	it("adds the caller's own rows after the status", async () => {
		await render(ContractStatus, {
			status: 'signed',
			hasSignedPdf: false,
			rows: [{ label: 'Fee', value: '$1,850.00' }]
		});

		expect(page.getByRole('term').elements().map((term) => term.textContent)).toEqual(['Status', 'Fee']);
	});

	it('shows no price-changed row when amountChangedAt is absent', async () => {
		await setup({ status: 'draft' });

		await expect.element(page.getByText('Price changed')).not.toBeInTheDocument();
	});

	it('shows a price-changed row, with the machine-readable instant, when amountChangedAt is set (#968)', async () => {
		const { container } = await setup({ status: 'draft', amountChangedAt: '2026-09-08T12:00:00Z' });

		await expect.element(page.getByText('Price changed')).toBeVisible();
		// A `datetime` attribute has no accessible signal of its own
		// (svelte-tests.md, querySelector case 3).
		expect(container.querySelector('time')).toHaveAttribute('datetime', '2026-09-08T12:00:00Z');
	});

	it('offers no Void action when there is no onVoid callback (Client-portal caller)', async () => {
		await setup({ status: 'signed' });

		await expect.element(page.getByRole('button', { name: 'Void this Contract' })).not.toBeInTheDocument();
	});

	it('offers no Void action on a voided Contract (no actions implying it is active)', async () => {
		await setup({ status: 'voided', onVoid: vi.fn() });

		await expect.element(page.getByRole('button', { name: 'Void this Contract' })).not.toBeInTheDocument();
	});

	it('offers no Void action on a draft or sent Contract', async () => {
		await setup({ status: 'sent', onVoid: vi.fn() });

		await expect.element(page.getByRole('button', { name: 'Void this Contract' })).not.toBeInTheDocument();
	});

	// #2004: destructive, so under its own heading, with its consequence
	// said first, and never the primary.
	it('offers Void on a signed Contract under its own heading, after its consequence', async () => {
		await setup({ status: 'signed', onVoid: vi.fn() });

		await expect.element(page.getByRole('heading', { level: 3, name: 'Void this Contract' })).toBeVisible();
		// .first(): the closed confirmation says it again, out of sight.
		await expect.element(page.getByText(VOID_CONSEQUENCE).first()).toBeVisible();
		await expect.element(page.getByRole('button', { name: 'Void this Contract' })).toBeVisible();
	});

	it('asks again before voiding, and sends nothing on Cancel', async () => {
		const onVoid = vi.fn().mockResolvedValue(undefined);
		await setup({ status: 'signed', onVoid });

		await page.getByRole('button', { name: 'Void this Contract' }).click();
		await expect.element(page.getByRole('dialog')).toBeVisible();
		await page.getByRole('dialog').getByRole('button', { name: 'Cancel' }).click();

		expect(onVoid).not.toHaveBeenCalled();
	});

	it('calls onVoid once the void is confirmed', async () => {
		const onVoid = vi.fn().mockResolvedValue(undefined);
		await setup({ status: 'signed', onVoid });

		await confirmVoid();

		expect(onVoid).toHaveBeenCalledOnce();
	});

	it('shows the refusal in the confirmation if onVoid rejects', async () => {
		const onVoid = vi.fn().mockRejectedValue(new Error('contract is not signed'));
		await setup({ status: 'signed', onVoid });

		await confirmVoid();

		await expect.element(page.getByRole('dialog').getByText('contract is not signed')).toBeVisible();
	});

	it('shows a fallback refusal if onVoid rejects with a non-Error value', async () => {
		const onVoid = vi.fn().mockRejectedValue('boom');
		await setup({ status: 'signed', onVoid });

		await confirmVoid();

		await expect.element(page.getByRole('dialog').getByText('Failed to void contract')).toBeVisible();
	});

	it('clears a refusal when the confirmation is canceled', async () => {
		const onVoid = vi.fn().mockRejectedValue(new Error('contract is not signed'));
		await setup({ status: 'signed', onVoid });

		await confirmVoid();
		await page.getByRole('dialog').getByRole('button', { name: 'Cancel' }).click();
		await page.getByRole('button', { name: 'Void this Contract' }).click();

		await expect.element(page.getByRole('dialog').getByText('contract is not signed')).not.toBeInTheDocument();
	});

	it('offers no PDF download when there is no onDownloadPdf callback (a role the endpoint refuses)', async () => {
		await setup({ status: 'signed', hasSignedPdf: true, onVoid: vi.fn() });

		await expect
			.element(page.getByRole('button', { name: 'Download signed Contract (PDF)' }))
			.not.toBeInTheDocument();
	});

	it('offers no PDF download on a draft or sent Contract, which has no signed PDF', async () => {
		await setup({ status: 'sent', hasSignedPdf: false, onDownloadPdf: vi.fn() });

		await expect
			.element(page.getByRole('button', { name: 'Download signed Contract (PDF)' }))
			.not.toBeInTheDocument();
	});

	// #1119: the download used to be gated on `status === 'signed'`, so
	// voiding a Contract took the Practice's own copy of it away -- while
	// the endpoint went on serving it (#299). The gate is the PDF's
	// existence now, which a void does not touch.
	it('offers the PDF download on a voided Contract whose signed PDF still exists', async () => {
		await setup({ status: 'voided', hasSignedPdf: true, onDownloadPdf: vi.fn() });

		await expect
			.element(page.getByRole('button', { name: 'Download signed Contract (PDF)' }))
			.toBeVisible();
	});

	it('offers no PDF download on a voided Contract that was never signed', async () => {
		await setup({ status: 'voided', hasSignedPdf: false, onDownloadPdf: vi.fn() });

		await expect
			.element(page.getByRole('button', { name: 'Download signed Contract (PDF)' }))
			.not.toBeInTheDocument();
	});

	// #1229: a fresh Draft (or an unsigned Sent Contract) never has a PDF
	// of its own -- #72's partial unique index requires the prior row to
	// be voided before this one could exist -- so hasSignedPdf true here
	// can only be that earlier, voided Contract's file.
	it('names the PDF as an earlier Contract\'s on a draft Contract that has a signed PDF', async () => {
		await setup({ status: 'draft', hasSignedPdf: true, onDownloadPdf: vi.fn() });

		await expect
			.element(page.getByText(PREVIOUS_CONTRACT_PDF_NOTICE))
			.toBeVisible();
	});

	it('names the PDF as an earlier Contract\'s on a sent Contract that has a signed PDF', async () => {
		await setup({ status: 'sent', hasSignedPdf: true, onDownloadPdf: vi.fn() });

		await expect
			.element(page.getByText(PREVIOUS_CONTRACT_PDF_NOTICE))
			.toBeVisible();
	});

	it('adds no notice on a signed Contract, whose PDF is always its own', async () => {
		await setup({ status: 'signed', hasSignedPdf: true, onDownloadPdf: vi.fn() });

		await expect
			.element(page.getByText(PREVIOUS_CONTRACT_PDF_NOTICE))
			.not.toBeInTheDocument();
	});

	it('adds no notice on a voided Contract, whose PDF is always its own', async () => {
		await setup({ status: 'voided', hasSignedPdf: true, onDownloadPdf: vi.fn() });

		await expect
			.element(page.getByText(PREVIOUS_CONTRACT_PDF_NOTICE))
			.not.toBeInTheDocument();
	});

	it('offers the PDF download on a signed Contract when onDownloadPdf is provided', async () => {
		await setup({ status: 'signed', hasSignedPdf: true, onDownloadPdf: vi.fn() });

		await expect
			.element(page.getByRole('button', { name: 'Download signed Contract (PDF)' }))
			.toBeVisible();
	});

	it('calls onDownloadPdf when the download action is clicked', async () => {
		const onDownloadPdf = vi.fn().mockResolvedValue(undefined);
		await setup({ status: 'signed', hasSignedPdf: true, onDownloadPdf });

		await page.getByRole('button', { name: 'Download signed Contract (PDF)' }).click();

		expect(onDownloadPdf).toHaveBeenCalledOnce();
	});

	it('shows an error message if onDownloadPdf rejects', async () => {
		const onDownloadPdf = vi.fn().mockRejectedValue(new Error('signed PDF not found'));
		await setup({ status: 'signed', hasSignedPdf: true, onDownloadPdf });

		await page.getByRole('button', { name: 'Download signed Contract (PDF)' }).click();

		await expect.element(page.getByRole('alert')).toHaveTextContent('signed PDF not found');
	});

	it('shows a fallback error message if onDownloadPdf rejects with a non-Error value', async () => {
		const onDownloadPdf = vi.fn().mockRejectedValue('boom');
		await setup({ status: 'signed', hasSignedPdf: true, onDownloadPdf });

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
			// #2103: back on the opener, not on the page.
			await expect.element(page.getByRole('button', { name: 'Request void' })).toHaveFocus();
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
			// #2103: back on the opener, not on the page.
			await expect.element(page.getByRole('button', { name: 'Decline' })).toHaveFocus();
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
