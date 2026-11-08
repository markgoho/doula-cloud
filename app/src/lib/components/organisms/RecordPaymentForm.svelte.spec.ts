import { page } from 'vitest/browser';
import { render } from 'vitest-browser-svelte';
import { describe, expect, it, vi } from 'vitest';
import { RefusalError } from '#lib/formErrors.js';
import type { RecordPaymentInput } from '#lib/invoice.js';
import RecordPaymentForm from './RecordPaymentForm.svelte';

const PRACTICE_TIMEZONE = 'America/New_York';

interface SetupOptions {
	onConfirm?: (input: RecordPaymentInput) => Promise<void>;
}

async function setup({ onConfirm = vi.fn().mockResolvedValue(undefined) }: SetupOptions = {}) {
	const onCancel = vi.fn();
	await render(RecordPaymentForm, {
		label: 'Record payment',
		heading: 'Record a payment',
		amountCents: 450_000,
		dateLabel: 'Date received',
		dateMissingMessage: 'Enter the date the payment was received',
		practiceTimezone: PRACTICE_TIMEZONE,
		onConfirm,
		onCancel
	});
	return { onConfirm, onCancel };
}

interface Answers {
	method?: string;
	note?: string;
	date?: string;
}

async function answer({ method = 'Check', note, date = '2025-01-05' }: Answers = {}) {
	await page.getByRole('radio', { name: method }).click();
	if (note !== undefined) await page.getByLabelText('Note (optional)').fill(note);
	await page.getByLabelText('Date received').fill(date);
}

async function reachReview(answers: Answers = {}) {
	await answer(answers);
	await page.getByRole('button', { name: 'Continue' }).click();
	await expect.element(page.getByRole('button', { name: 'Confirm and record' })).toBeVisible();
}

describe('RecordPaymentForm', () => {
	it('asks how it was paid with no method chosen, a note, and a date defaulting to today in the Practice zone', async () => {
		await setup();

		await expect.element(page.getByRole('region', { name: 'Record payment' })).toBeVisible();
		await expect.element(page.getByRole('heading', { name: 'Record a payment' })).toBeVisible();
		for (const name of ['Check', 'Bank transfer', 'Cash', 'Other']) {
			await expect.element(page.getByRole('radio', { name })).not.toBeChecked();
		}
		const today = new Intl.DateTimeFormat('en-CA', { timeZone: PRACTICE_TIMEZONE }).format(new Date());
		await expect.element(page.getByLabelText('Date received')).toHaveValue(today);
	});

	it('refuses a missing method', async () => {
		await setup();

		await page.getByRole('button', { name: 'Continue' }).click();

		await expect.element(page.getByText('Select how the payment was made').first()).toBeVisible();
		await expect.element(page.getByRole('button', { name: 'Confirm and record' })).not.toBeInTheDocument();
	});

	it('refuses a missing date with the caller-supplied message', async () => {
		await setup();

		await answer({ date: '' });
		await page.getByRole('button', { name: 'Continue' }).click();

		await expect.element(page.getByText('Enter the date the payment was received').first()).toBeVisible();
	});

	it('refuses "Other" without a note', async () => {
		await setup();

		await answer({ method: 'Other', note: '  ' });
		await page.getByRole('button', { name: 'Continue' }).click();

		await expect.element(page.getByText('Enter a note for "Other"').first()).toBeVisible();
	});

	it('refuses a date in the future', async () => {
		await setup();

		await answer({ date: '2999-01-01' });
		await page.getByRole('button', { name: 'Continue' }).click();

		await expect.element(page.getByText('The date cannot be in the future').first()).toBeVisible();
	});

	it('reviews every answer before anything is written, with the amount', async () => {
		const { onConfirm } = await setup();

		await reachReview({ method: 'Bank transfer', note: 'Wire from the Client', date: '2025-01-05' });

		await expect.element(page.getByText('$4,500.00')).toBeVisible();
		await expect.element(page.getByText('Bank transfer', { exact: true })).toBeVisible();
		await expect.element(page.getByText('Wire from the Client')).toBeVisible();
		await expect.element(page.getByText('Jan 5, 2025')).toBeVisible();
		expect(onConfirm).not.toHaveBeenCalled();
	});

	it('shows a dash for an empty note and sends it as undefined', async () => {
		const { onConfirm } = await setup();

		await reachReview({ method: 'Cash' });
		await expect.element(page.getByText('—')).toBeVisible();
		await page.getByRole('button', { name: 'Confirm and record' }).click();

		expect(onConfirm).toHaveBeenCalledWith({ method: 'cash', note: undefined, paidOn: '2025-01-05' });
	});

	it('sends the trimmed note on confirm', async () => {
		const { onConfirm } = await setup();

		await reachReview({ method: 'Other', note: '  Venmo  ' });
		await page.getByRole('button', { name: 'Confirm and record' }).click();

		expect(onConfirm).toHaveBeenCalledWith({ method: 'other', note: 'Venmo', paidOn: '2025-01-05' });
	});

	it('goes back to the answers when Change is pressed', async () => {
		await setup();
		await reachReview({ note: 'Check no. 1042' });

		await page.getByRole('button', { name: 'Change' }).click();

		await expect.element(page.getByLabelText('Note (optional)')).toHaveValue('Check no. 1042');
		await expect.element(page.getByRole('radio', { name: 'Check' })).toBeChecked();
	});

	it('hands canceling to onCancel', async () => {
		const { onCancel } = await setup();

		await page.getByRole('button', { name: 'Cancel' }).click();

		expect(onCancel).toHaveBeenCalledOnce();
	});

	it('reads a refusal naming fields back onto the entry step', async () => {
		await setup({
			onConfirm: vi.fn().mockRejectedValue(
				new RefusalError('Refused', {
					method: 'Method is not accepted',
					note: 'Note is too long',
					paidOn: 'Date is before the Invoice was sent'
				})
			)
		});
		await reachReview();

		await page.getByRole('button', { name: 'Confirm and record' }).click();

		await expect.element(page.getByLabelText('Note (optional)')).toHaveAccessibleDescription('Note is too long');
		await expect
			.element(page.getByLabelText('Date received'))
			.toHaveAccessibleDescription('Date is before the Invoice was sent');
		await expect.element(page.getByText('Method is not accepted').first()).toBeVisible();
	});

	it('shows a refusal naming no field in the summary and stays on the review step', async () => {
		await setup({
			onConfirm: vi.fn().mockRejectedValue(new RefusalError('This Invoice is already paid'))
		});
		await reachReview();

		await page.getByRole('button', { name: 'Confirm and record' }).click();

		await expect.element(page.getByText('This Invoice is already paid')).toBeVisible();
		await expect.element(page.getByRole('button', { name: 'Confirm and record' })).toBeVisible();
	});

	it('falls back to a plain sentence when the failure carries no message', async () => {
		await setup({ onConfirm: vi.fn().mockRejectedValue('offline') });
		await reachReview();

		await page.getByRole('button', { name: 'Confirm and record' }).click();

		await expect.element(page.getByText('Failed to record payment')).toBeVisible();
	});
});
