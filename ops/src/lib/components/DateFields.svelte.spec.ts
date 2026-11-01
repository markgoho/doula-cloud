import { describe, expect, it } from 'vitest';
import { render } from 'vitest-browser-svelte';
import { page } from 'vitest/browser';
import DateFields from './DateFields.svelte';

describe('DateFields', () => {
	it('asks for the month, day and year under one legend, the day taking the id', async () => {
		await render(DateFields, { id: 'arrived', legend: 'Date the request arrived', hint: 'For example, 10 30 2026.' });

		await expect.element(page.getByRole('group', { name: 'Date the request arrived' })).toBeVisible();
		await expect.element(page.getByLabelText('Day')).toHaveAttribute('id', 'arrived');
		await expect.element(page.getByLabelText('Month')).toHaveAttribute('inputmode', 'numeric');
		await expect.element(page.getByText('For example, 10 30 2026.')).toBeVisible();
		await expect.element(page.getByLabelText('Year')).not.toHaveAttribute('aria-invalid');
	});

	it('keeps what is typed', async () => {
		await render(DateFields, { id: 'arrived', legend: 'Date the request arrived' });

		await page.getByLabelText('Month').fill('10');
		await page.getByLabelText('Day').fill('30');
		await page.getByLabelText('Year').fill('2026');

		await expect.element(page.getByLabelText('Year')).toHaveValue('2026');
		await expect.element(page.getByLabelText('Day')).toHaveValue('30');
	});

	it('shows the error under the legend and marks every box', async () => {
		await render(DateFields, { id: 'arrived', legend: 'Date the request arrived', error: 'Enter the date the request arrived' });

		await expect.element(page.getByText('Enter the date the request arrived')).toBeVisible();
		await expect.element(page.getByLabelText('Month')).toHaveAttribute('aria-invalid', 'true');
		await expect.element(page.getByLabelText('Day')).toHaveAttribute('aria-invalid', 'true');
		await expect.element(page.getByLabelText('Year')).toHaveAttribute('aria-invalid', 'true');
	});
});
