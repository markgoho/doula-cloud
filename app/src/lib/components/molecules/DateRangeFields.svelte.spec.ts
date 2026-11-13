import { page } from 'vitest/browser';
import { render } from 'vitest-browser-svelte';
import { describe, expect, it } from 'vitest';
import DateRangeFields from './DateRangeFields.svelte';

async function setup() {
	await render(DateRangeFields, { from: '2026-10-26', to: '2026-11-25' });
}

describe('DateRangeFields', () => {
	it('asks for the first and last day, labeled From and To', async () => {
		await setup();

		await expect.element(page.getByLabelText('From')).toHaveValue('2026-10-26');
		await expect.element(page.getByLabelText('To')).toHaveValue('2026-11-25');
	});

	it('names the boxes so a GET form sends from and to', async () => {
		await setup();

		await expect.element(page.getByLabelText('From')).toHaveAttribute('name', 'from');
		await expect.element(page.getByLabelText('To')).toHaveAttribute('name', 'to');
	});

	it('takes what the person types into either box', async () => {
		await setup();

		await page.getByLabelText('To').fill('2026-12-01');

		await expect.element(page.getByLabelText('To')).toHaveValue('2026-12-01');
		await expect.element(page.getByLabelText('From')).toHaveValue('2026-10-26');
	});

	it('draws no hint and no error unless it is given them', async () => {
		await setup();

		expect(page.getByRole('alert').elements()).toHaveLength(0);
		await expect.element(page.getByLabelText('To')).not.toHaveAccessibleDescription('Both days are included.');
	});

	it('draws one hint for the pair and joins it to both boxes', async () => {
		await render(DateRangeFields, { from: '2026-10-26', to: '2026-11-25', hint: 'Both days are included.' });

		expect(page.getByText('Both days are included.').elements()).toHaveLength(1);
		await expect.element(page.getByLabelText('From')).toHaveAccessibleDescription('Both days are included.');
		await expect.element(page.getByLabelText('To')).toHaveAccessibleDescription('Both days are included.');
	});

	it('says each error once for the pair, under the hint, and joins each to its own box only', async () => {
		await render(DateRangeFields, {
			from: '',
			to: '2026-11-25',
			fromError: 'Enter a From day',
			toError: 'Choose a later To day.',
			hint: 'Both days are included.'
		});

		await expect.element(page.getByLabelText('From')).toHaveAccessibleDescription('Both days are included. Enter a From day');
		await expect.element(page.getByLabelText('To')).toHaveAccessibleDescription(
			'Both days are included. Choose a later To day.'
		);
		await expect.element(page.getByLabelText('From')).toBeInvalid();
		await expect.element(page.getByLabelText('To')).toBeInvalid();
		expect(page.getByRole('alert').elements()).toHaveLength(2);
	});

	it('marks only the box at fault, and draws one error when both say the same', async () => {
		await render(DateRangeFields, {
			from: '2026-11-25',
			to: '2026-11-10',
			toError: 'Choose a To day that is the same as the From day or later.'
		});

		await expect.element(page.getByLabelText('To')).toBeInvalid();
		await expect.element(page.getByLabelText('From')).toBeValid();
		await expect.element(page.getByLabelText('To')).toHaveAccessibleDescription(
			'Choose a To day that is the same as the From day or later.'
		);
		await expect.element(page.getByLabelText('From')).not.toHaveAttribute('aria-describedby');
		expect(page.getByRole('alert').elements()).toHaveLength(1);
	});

	it('draws one error when both boxes carry the same words', async () => {
		await render(DateRangeFields, { from: '', to: '', fromError: 'Enter a day', toError: 'Enter a day' });

		expect(page.getByRole('alert').elements()).toHaveLength(1);
		await expect.element(page.getByLabelText('From')).toBeInvalid();
		await expect.element(page.getByLabelText('To')).toBeInvalid();
		await expect.element(page.getByLabelText('From')).toHaveAccessibleDescription('Enter a day');
		await expect.element(page.getByLabelText('To')).toHaveAccessibleDescription('Enter a day');
	});

	it('takes a new From the same way', async () => {
		await setup();

		await page.getByLabelText('From').fill('2026-10-01');

		await expect.element(page.getByLabelText('From')).toHaveValue('2026-10-01');
	});
});
