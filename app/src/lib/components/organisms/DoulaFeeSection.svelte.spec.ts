import { page } from 'vitest/browser';
import { describe, expect, it, vi } from 'vitest';
import { render } from 'vitest-browser-svelte';
import DoulaFeeSection from './DoulaFeeSection.svelte';
import type { DoulaFee } from '#lib/doulaFee.js';
import type { RecordPaymentInput } from '#lib/invoice.js';
import { RefusalError } from '#lib/formErrors.js';

const unpaid: DoulaFee = {
	attachmentId: 'attachment-1',
	doulaName: 'Lena Vasquez',
	amountCents: 45_000,
	acceptedOn: '2026-10-27'
};

const paid: DoulaFee = {
	attachmentId: 'attachment-2',
	doulaName: 'Hana Kim',
	amountCents: 60_000,
	acceptedOn: '2026-05-02',
	endedOn: '2026-08-14',
	paid: {
		id: 'fee-payment-1',
		amountCents: 60_000,
		method: 'cash',
		note: 'At the office',
		paidOn: '2026-08-30',
		recordedBy: 'Renata Alvarez',
		recordedAt: '2026-08-30T15:00:00Z'
	}
};

interface SetupOptions {
	fees?: DoulaFee[];
	onRecord?: (attachmentId: string, input: RecordPaymentInput) => Promise<void>;
	onRemove?: (attachmentId: string, paymentId: string, reason: string) => Promise<void>;
}

async function setup({
	fees = [unpaid, paid],
	onRecord = vi.fn().mockResolvedValue(undefined),
	onRemove = vi.fn().mockResolvedValue(undefined)
}: SetupOptions = {}) {
	await render(DoulaFeeSection, { fees, practiceTimezone: 'America/New_York', onRecord, onRemove });
	return { onRecord, onRemove };
}

/* At the spec's width DataTable draws each fee as a card, its values
   definitions. */
function fact(text: string) {
	return page.getByRole('definition').filter({ hasText: text });
}

describe('DoulaFeeSection', () => {
	it('lists each fee, open or ended, and what was paid', async () => {
		await setup();

		await expect.element(page.getByText('Lena Vasquez', { exact: true }).last()).toBeVisible();
		await expect.element(fact('Not recorded yet')).toBeVisible();
		await expect.element(fact('$600.00 on Aug 30, 2026, in cash')).toBeVisible();
		await expect.element(fact('May 2, 2026 to Aug 14, 2026')).toBeVisible();
		await expect.element(fact('Since Oct 27, 2026')).toBeVisible();
	});

	it('leaves the Note out when no record carries one', async () => {
		await setup({ fees: [unpaid, { ...paid, paid: { ...paid.paid!, note: undefined } }] });

		await expect.element(fact('$600.00 on Aug 30, 2026, in cash')).toBeVisible();
		expect(page.getByRole('term').filter({ hasText: 'Note' }).all()).toHaveLength(0);
	});

	it('says so when no Doula has a fee', async () => {
		await setup({ fees: [] });

		await expect.element(page.getByText('No Doula on this Engagement has an agreed fee.').first()).toBeVisible();
	});

	it('records the whole fee as paid, then says what it wrote', async () => {
		const { onRecord } = await setup();

		await page.getByRole('button', { name: 'Mark fee paid' }).first().click();
		await expect.element(page.getByRole('heading', { name: 'Record paying Lena Vasquez $450.00' })).toBeVisible();
		await page.getByRole('radio', { name: 'Bank transfer' }).click();
		await page.getByLabelText('Date paid').fill('2026-10-01');
		await page.getByRole('button', { name: 'Continue' }).click();
		await page.getByRole('button', { name: 'Confirm and record' }).click();

		expect(onRecord).toHaveBeenCalledWith('attachment-1', { method: 'bank_transfer', paidOn: '2026-10-01' });
		await expect
			.element(page.getByRole('status').filter({ hasText: 'Recorded: Lena Vasquez paid $450.00 on Oct 1, 2026, by bank transfer.' }))
			.toBeVisible();
	});

	it('refuses an empty date in its own words', async () => {
		await setup();

		await page.getByRole('button', { name: 'Mark fee paid' }).first().click();
		await page.getByRole('radio', { name: 'Check' }).click();
		await page.getByLabelText('Date paid').fill('');
		await page.getByRole('button', { name: 'Continue' }).click();

		await expect.element(page.getByText('Enter the date the fee was paid').first()).toBeVisible();
	});

	it('cancels without recording', async () => {
		const { onRecord } = await setup();

		await page.getByRole('button', { name: 'Mark fee paid' }).first().click();
		await page.getByRole('button', { name: 'Cancel' }).click();

		expect(page.getByRole('heading', { name: 'Record paying Lena Vasquez $450.00' }).all()).toHaveLength(0);
		expect(onRecord).not.toHaveBeenCalled();
	});

	it('removes a payment recorded by mistake, with a reason', async () => {
		const { onRemove } = await setup();

		await page.getByRole('button', { name: 'Remove payment' }).first().click();
		await page.getByLabelText('Reason').fill('Paid to the wrong Doula');
		await page.getByRole('button', { name: 'Continue' }).click();
		await page.getByRole('button', { name: 'Confirm and remove' }).click();

		expect(onRemove).toHaveBeenCalledWith('attachment-2', 'fee-payment-1', 'Paid to the wrong Doula');
		await expect.element(page.getByRole('status').filter({ hasText: 'Payment to Hana Kim removed.' })).toBeVisible();
	});

	it('reads a refusal back onto the reason, and asks for one', async () => {
		const onRemove = vi.fn().mockRejectedValue(
			new RefusalError('reason cannot be blank', { reason: 'Enter a reason for removing this payment' })
		);
		await setup({ onRemove });

		await page.getByRole('button', { name: 'Remove payment' }).first().click();
		await page.getByRole('button', { name: 'Continue' }).click();
		await expect.element(page.getByText('Enter a reason for removing this payment').first()).toBeVisible();

		await page.getByLabelText('Reason').fill('x');
		await page.getByRole('button', { name: 'Continue' }).click();
		await page.getByRole('button', { name: 'Confirm and remove' }).click();
		await expect.element(page.getByLabelText('Reason')).toBeVisible();
	});

	it('shows a refusal that names no field', async () => {
		const onRemove = vi.fn().mockRejectedValue(new Error('This payment has already been removed.'));
		await setup({ onRemove });

		await page.getByRole('button', { name: 'Remove payment' }).first().click();
		await page.getByLabelText('Reason').fill('Duplicate');
		await page.getByRole('button', { name: 'Continue' }).click();
		await page.getByRole('button', { name: 'Change' }).click();
		await page.getByRole('button', { name: 'Continue' }).click();
		await page.getByRole('button', { name: 'Confirm and remove' }).click();

		await expect.element(page.getByRole('alert').filter({ hasText: 'This payment has already been removed.' })).toBeVisible();
	});

	it('names a failure that is not an Error', async () => {
		const onRemove = vi.fn().mockRejectedValue('offline');
		await setup({ onRemove });

		await page.getByRole('button', { name: 'Remove payment' }).first().click();
		await page.getByLabelText('Reason').fill('Duplicate');
		await page.getByRole('button', { name: 'Continue' }).click();
		await page.getByRole('button', { name: 'Confirm and remove' }).click();

		await expect.element(page.getByRole('alert').filter({ hasText: 'Failed to reverse this payment' })).toBeVisible();
	});
});
