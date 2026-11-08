import { page } from 'vitest/browser';
import { describe, expect, it, vi } from 'vitest';
import { render } from 'vitest-browser-svelte';
import { paymentMethodOptions } from '#lib/invoice.js';
import PaymentMethodFields from './PaymentMethodFields.svelte';

async function setup({ methodError, noteError }: { methodError?: string; noteError?: string } = {}) {
	const onMethodChange = vi.fn();
	const onNoteInput = vi.fn();
	await render(PaymentMethodFields, {
		legend: 'Method',
		methodName: 'spec-method',
		method: undefined,
		onMethodChange,
		methodError,
		noteFieldId: 'spec-note',
		note: '',
		onNoteInput,
		noteError
	});
	return { onMethodChange, onNoteInput };
}

describe('PaymentMethodFields', () => {
	it('asks the method under the legend it is given, and hands the choice on', async () => {
		const { onMethodChange } = await setup();
		const first = paymentMethodOptions[0]!;

		await page.getByRole('radio', { name: first.label }).click();

		await expect.element(page.getByRole('group', { name: 'Method' })).toBeVisible();
		expect(onMethodChange).toHaveBeenCalledWith(first.value);
	});

	it('hands a typed note on', async () => {
		const { onNoteInput } = await setup();

		await page.getByLabelText('Note (optional)').fill('Check 1042');

		expect(onNoteInput).toHaveBeenLastCalledWith('Check 1042');
	});

	it('says what is wrong beside the method and the note', async () => {
		await setup({ methodError: 'Select a method', noteError: 'Enter a note' });

		await expect.element(page.getByText('Select a method')).toBeVisible();
		await expect.element(page.getByLabelText('Note (optional)')).toHaveAccessibleDescription('Enter a note');
	});
});
