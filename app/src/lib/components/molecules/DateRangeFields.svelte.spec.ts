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

	it('says the pair hint and each error beside its own box', async () => {
		await render(DateRangeFields, {
			from: '',
			to: '2026-11-25',
			fromError: 'Enter a From day',
			toError: 'Choose a later To day.',
			hint: 'Both days are included.'
		});

		await expect.element(page.getByLabelText('To')).toHaveAccessibleDescription('Both days are included. Choose a later To day.');
		await expect.element(page.getByLabelText('From')).toHaveAccessibleDescription('Both days are included. Enter a From day');
		await expect.element(page.getByLabelText('To')).toBeInvalid();
	});

	it('takes a new From the same way', async () => {
		await setup();

		await page.getByLabelText('From').fill('2026-10-01');

		await expect.element(page.getByLabelText('From')).toHaveValue('2026-10-01');
	});
});
