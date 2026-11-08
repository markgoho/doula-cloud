import { page } from 'vitest/browser';
import { render } from 'vitest-browser-svelte';
import { describe, expect, it, vi } from 'vitest';
import { RefusalError } from '#lib/formErrors.js';
import ReversePaymentForm from './ReversePaymentForm.svelte';

interface SetupOptions {
	onConfirm?: (reason: string) => Promise<void>;
}

async function setup({ onConfirm = vi.fn().mockResolvedValue(undefined) }: SetupOptions = {}) {
	const onCancel = vi.fn();
	await render(ReversePaymentForm, {
		label: 'Reverse payment',
		heading: 'Reverse this payment',
		amountCents: 125_000,
		reasonMissingMessage: 'Enter a reason for the reversal',
		confirmLabel: 'Confirm and reverse',
		onConfirm,
		onCancel
	});
	return { onConfirm, onCancel };
}

async function reachReview(reason = 'Entered against the wrong Invoice') {
	await page.getByLabelText('Reason').fill(reason);
	await page.getByRole('button', { name: 'Continue' }).click();
}

describe('ReversePaymentForm', () => {
	it('asks for a reason under its own heading', async () => {
		await setup();

		await expect.element(page.getByRole('region', { name: 'Reverse payment' })).toBeVisible();
		await expect.element(page.getByRole('heading', { name: 'Reverse this payment' })).toBeVisible();
		await expect.element(page.getByLabelText('Reason')).toBeVisible();
	});

	it('refuses an empty reason before the review step', async () => {
		await setup();

		await page.getByLabelText('Reason').fill(' '.repeat(3));
		await page.getByRole('button', { name: 'Continue' }).click();

		await expect
			.element(page.getByLabelText('Reason'))
			.toHaveAccessibleDescription('Enter a reason for the reversal');
		await expect.element(page.getByRole('button', { name: 'Confirm and reverse' })).not.toBeInTheDocument();
	});

	it('reviews the amount and the reason, and sends the trimmed reason on confirm', async () => {
		const { onConfirm } = await setup();

		await reachReview('  Entered against the wrong Invoice  ');

		await expect.element(page.getByText('$1,250.00')).toBeVisible();
		await expect.element(page.getByText('Entered against the wrong Invoice')).toBeVisible();

		await page.getByRole('button', { name: 'Confirm and reverse' }).click();

		expect(onConfirm).toHaveBeenCalledWith('Entered against the wrong Invoice');
	});

	it('goes back to the reason with what was typed when Change is pressed', async () => {
		await setup();
		await reachReview();

		await page.getByRole('button', { name: 'Change' }).click();

		await expect
			.element(page.getByLabelText('Reason'))
			.toHaveValue('Entered against the wrong Invoice');
	});

	it('hands canceling to onCancel', async () => {
		const { onCancel } = await setup();

		await page.getByRole('button', { name: 'Cancel' }).click();

		expect(onCancel).toHaveBeenCalledOnce();
	});

	it('reads a refusal naming the reason back onto the Reason field', async () => {
		await setup({
			onConfirm: vi
				.fn()
				.mockRejectedValue(new RefusalError('Refused', { reason: 'Reason is too long' }))
		});
		await reachReview();

		await page.getByRole('button', { name: 'Confirm and reverse' }).click();

		await expect
			.element(page.getByLabelText('Reason'))
			.toHaveAccessibleDescription('Reason is too long');
	});

	it('shows a refusal naming no field as an alert and stays on the review step', async () => {
		await setup({
			onConfirm: vi.fn().mockRejectedValue(new RefusalError('This payment was already reversed'))
		});
		await reachReview();

		await page.getByRole('button', { name: 'Confirm and reverse' }).click();

		await expect.element(page.getByRole('alert')).toHaveTextContent('This payment was already reversed');
		await expect.element(page.getByRole('button', { name: 'Confirm and reverse' })).toBeVisible();
	});

	it('falls back to a plain sentence when the failure carries no message', async () => {
		await setup({ onConfirm: vi.fn().mockRejectedValue('offline') });
		await reachReview();

		await page.getByRole('button', { name: 'Confirm and reverse' }).click();

		await expect.element(page.getByRole('alert')).toHaveTextContent('Failed to reverse this payment');
	});
});
