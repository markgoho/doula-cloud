import { page } from 'vitest/browser';
import { describe, expect, it } from 'vitest';
import { render } from 'vitest-browser-svelte';
import SentPeriodFields from './SentPeriodFields.svelte';
import type { SentPeriodChoice } from '../../sentPeriod.js';

async function setup({ choice = '' }: { choice?: SentPeriodChoice } = {}) {
	await render(SentPeriodFields, { choice, from: '', to: '' });
}

describe('SentPeriodFields', () => {
	it('offers the periods and starts on any time', async () => {
		await setup();

		const select = page.getByRole('combobox', { name: 'Sent' });
		await expect.element(select).toHaveValue('');
		await expect.element(select.getByRole('option', { name: 'Last year' })).toBeInTheDocument();
		await expect.element(page.getByLabelText('From', { exact: true })).not.toBeInTheDocument();
	});

	it('opens From and To for Choose days, with the hint that both days are included', async () => {
		await setup();

		await page.getByRole('combobox', { name: 'Sent' }).selectOptions('Choose days');

		await expect.element(page.getByLabelText('From', { exact: true })).toBeVisible();
		await expect.element(page.getByLabelText('To', { exact: true })).toBeVisible();
		await expect.element(page.getByText('Both days are included.')).toBeVisible();
	});
});
