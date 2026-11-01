import { describe, expect, it } from 'vitest';
import { render } from 'vitest-browser-svelte';
import { page } from 'vitest/browser';
import RadioGroup from './RadioGroup.svelte';

const options = [
	{ value: 'a', label: 'Renata Alvarez' },
	{ value: 'b', label: 'Maya Okonkwo' }
];

describe('RadioGroup', () => {
	it('offers one choice under one legend, the first radio taking the id', async () => {
		await render(RadioGroup, { id: 'owner', legend: 'Owner who asked', options });

		await expect.element(page.getByRole('group', { name: 'Owner who asked' })).toBeVisible();
		await expect.element(page.getByLabelText('Renata Alvarez')).toHaveAttribute('id', 'owner');
		await expect.element(page.getByLabelText('Maya Okonkwo')).toHaveAttribute('id', 'owner-1');
	});

	it('holds the one chosen', async () => {
		await render(RadioGroup, { id: 'owner', legend: 'Owner who asked', options, value: 'a' });

		await page.getByLabelText('Maya Okonkwo').click();

		await expect.element(page.getByLabelText('Maya Okonkwo')).toBeChecked();
		await expect.element(page.getByLabelText('Renata Alvarez')).not.toBeChecked();
	});

	it('follows a choice made from outside', async () => {
		const { rerender } = await render(RadioGroup, { id: 'owner', legend: 'Owner who asked', options, value: 'a' });

		await rerender({ value: 'b', options: [...options, { value: 'c', label: 'Dee Whitlock' }] });

		await expect.element(page.getByLabelText('Maya Okonkwo')).toBeChecked();
		await expect.element(page.getByLabelText('Dee Whitlock')).not.toBeChecked();
	});

	it('shows the error under the legend', async () => {
		await render(RadioGroup, { id: 'owner', legend: 'Owner who asked', options, error: 'Choose the Owner who asked for the refund' });

		await expect.element(page.getByText('Choose the Owner who asked for the refund')).toBeVisible();
	});
});
