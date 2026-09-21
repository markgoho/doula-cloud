import { page } from 'vitest/browser';
import { describe, expect, it, vi } from 'vitest';
import { render } from 'vitest-browser-svelte';
import RefundPaymentForm from './RefundPaymentForm.svelte';
import { stripeRefundFeeWarning, type Invoice, type RefundPaymentInput } from '#lib/invoice.js';
import { RefusalError } from '#lib/formErrors.js';

const byHand: Invoice = {
	id: 'inv-1',
	contractId: 'contract-1',
	status: 'paid',
	amountCents: 50_000,
	currency: 'usd',
	createdAt: '2026-01-01T00:00:00Z',
	paidAt: '2026-01-05T00:00:00Z',
	dueAt: '2026-01-31T00:00:00Z',
	reference: 'INV-0001',
	refundedCents: 20_000,
	billingMode: 'by_hand',
	activePaymentId: 'payment-1',
	activePaymentKind: 'manual'
};

const byCard: Invoice = { ...byHand, billingMode: 'stripe', activePaymentKind: 'stripe', refundedCents: 0 };

async function setup(invoice: Invoice, onConfirm = vi.fn<(input: RefundPaymentInput) => Promise<void>>().mockResolvedValue()) {
	const onCancel = vi.fn();
	await render(RefundPaymentForm, { invoice, onConfirm, onCancel });
	return { onConfirm, onCancel };
}

const amountField = () => page.getByLabelText('Amount to return (USD)');
const continueButton = () => page.getByRole('button', { name: 'Continue' });
const confirmButton = () => page.getByRole('button', { name: 'Confirm and return money' });

describe('RefundPaymentForm', () => {
	it('says how much is left to return, net of earlier Refunds', async () => {
		await setup(byHand);

		await expect.element(page.getByText('Up to $300.00 is left to return on this payment.')).toBeVisible();
	});

	describe('a Payment recorded by hand', () => {
		it('asks how the money went back, and sends the method and note', async () => {
			const { onConfirm } = await setup(byHand);

			await amountField().fill('125.50');
			await page.getByRole('radio', { name: 'Bank transfer' }).click();
			await page.getByLabelText('Note (optional)').fill('  sent to her account  ');
			await continueButton().click();

			await expect.element(page.getByText('$125.50')).toBeVisible();
			await expect.element(page.getByText('Bank transfer')).toBeVisible();
			expect(page.getByText(stripeRefundFeeWarning).all()).toHaveLength(0);
			await confirmButton().click();

			expect(onConfirm).toHaveBeenCalledWith({ amountCents: 12_550, method: 'bank_transfer', note: 'sent to her account' });
		});

		it('shows a dash for an empty note on review', async () => {
			await setup(byHand);

			await amountField().fill('10');
			await continueButton().click();

			await expect.element(page.getByText('—')).toBeVisible();
		});

		it('refuses "Other" with no note, on the Note field', async () => {
			await setup(byHand);

			await amountField().fill('10');
			await page.getByRole('radio', { name: 'Other' }).click();
			await continueButton().click();

			await expect.element(page.getByText('Enter a note for "Other"')).toBeVisible();
			expect(confirmButton().all()).toHaveLength(0);
		});
	});

	describe('a card Payment Stripe collected', () => {
		it('asks no method, warns that Stripe keeps its fee before she confirms, and sends only the amount', async () => {
			const { onConfirm } = await setup(byCard);

			expect(page.getByRole('radio').all()).toHaveLength(0);
			await amountField().fill('500');
			await continueButton().click();

			await expect.element(page.getByText(stripeRefundFeeWarning)).toBeVisible();
			await expect.element(page.getByText('Stripe, to the card that paid')).toBeVisible();
			await confirmButton().click();

			expect(onConfirm).toHaveBeenCalledWith({ amountCents: 50_000 });
		});
	});

	describe('the amount', () => {
		for (const [typed, why] of [
			['', 'empty'],
			['0', 'zero'],
			['-5', 'negative']
		] as const) {
			it(`refuses an amount that is ${why}, on the Amount field`, async () => {
				await setup(byHand);

				await amountField().fill(typed);
				await continueButton().click();

				await expect.element(page.getByText('Enter an amount to return greater than $0.00')).toBeVisible();
				await expect.element(amountField()).toHaveAttribute('aria-invalid', 'true');
			});
		}

		it('refuses more than is left to return', async () => {
			await setup(byHand);

			await amountField().fill('300.01');
			await continueButton().click();

			await expect.element(page.getByText('Amount to return must be $300.00 or less')).toBeVisible();
		});
	});

	describe('a refusal from the BFF', () => {
		it('reads a field-level refusal back onto the form', async () => {
			const onConfirm = vi
				.fn<(input: RefundPaymentInput) => Promise<void>>()
				.mockRejectedValue(
					new RefusalError('amountCents must be greater than zero', {
						amountCents: 'Enter an amount to return greater than $0.00'
					})
				);
			await setup(byHand, onConfirm);

			await amountField().fill('10');
			await continueButton().click();
			await confirmButton().click();

			await expect.element(amountField()).toHaveAttribute('aria-invalid', 'true');
		});

		it('shows a refusal that names no field as a summary', async () => {
			const onConfirm = vi
				.fn<(input: RefundPaymentInput) => Promise<void>>()
				.mockRejectedValue(new RefusalError('Stripe would not issue this refund.', { unrelated: 'x' }));
			await setup(byCard, onConfirm);

			await amountField().fill('10');
			await continueButton().click();
			await confirmButton().click();

			await expect.element(page.getByRole('alert')).toHaveTextContent('Stripe would not issue this refund.');
		});

		it('falls back to a generic message when the call rejects with a non-Error', async () => {
			const onConfirm = vi.fn<(input: RefundPaymentInput) => Promise<void>>().mockRejectedValue('boom');
			await setup(byCard, onConfirm);

			await amountField().fill('10');
			await continueButton().click();
			await confirmButton().click();

			await expect.element(page.getByRole('alert')).toHaveTextContent('Failed to return this money');
		});
	});

	it('goes back to the form on Change, and hands Cancel to its caller', async () => {
		const { onCancel } = await setup(byHand);

		await amountField().fill('10');
		await continueButton().click();
		await page.getByRole('button', { name: 'Change' }).click();
		await expect.element(continueButton()).toBeVisible();

		await page.getByRole('button', { name: 'Cancel' }).click();
		expect(onCancel).toHaveBeenCalledOnce();
	});
});
