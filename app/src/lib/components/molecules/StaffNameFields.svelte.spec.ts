import { page } from 'vitest/browser';
import { describe, expect, it } from 'vitest';
import { render } from 'vitest-browser-svelte';
import StaffNameFields from './StaffNameFields.svelte';

describe('StaffNameFields', () => {
	it('asks for a first name and a last name, with GOV.UK autocomplete tokens', async () => {
		await render(StaffNameFields, { firstName: '', lastName: '' });
		const first = (await page.getByLabelText('First name').element()) as HTMLInputElement;
		const last = (await page.getByLabelText('Last name').element()) as HTMLInputElement;
		expect(first.autocomplete).toBe('given-name');
		expect(last.autocomplete).toBe('family-name');
		expect(first.required).toBe(true);
		expect(last.required).toBe(true);
	});

	it('shows the names it was given and keeps them apart', async () => {
		await render(StaffNameFields, { firstName: 'Mary Anne', lastName: 'Smith' });
		await expect.element(page.getByLabelText('First name')).toHaveValue('Mary Anne');
		await expect.element(page.getByLabelText('Last name')).toHaveValue('Smith');
	});

	it('writes what is typed back to the right field', async () => {
		await render(StaffNameFields, { firstName: '', lastName: '' });
		await page.getByLabelText('First name').fill('Lena');
		await page.getByLabelText('Last name').fill('de la Cruz');
		await expect.element(page.getByLabelText('First name')).toHaveValue('Lena');
		await expect.element(page.getByLabelText('Last name')).toHaveValue('de la Cruz');
	});

	it('shows each refusal beside its own field', async () => {
		await render(StaffNameFields, {
			firstName: '',
			lastName: '',
			firstError: 'Enter your first name',
			lastError: 'Enter your last name'
		});
		await expect.element(page.getByText('Enter your first name')).toBeVisible();
		await expect.element(page.getByText('Enter your last name')).toBeVisible();
	});
});
