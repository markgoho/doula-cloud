import { page } from 'vitest/browser';
import { describe, expect, it, vi } from 'vitest';
import { render } from 'vitest-browser-svelte';
import TotpCodeField from './TotpCodeField.svelte';

describe('TotpCodeField', () => {
	it('asks for the 6-digit authenticator code, as a single numeric field', async () => {
		const onInput = vi.fn();
		await render(TotpCodeField, { id: 'code', value: '', onInput });

		const field = page.getByLabelText('Authenticator app code');
		await expect.element(field).toBeVisible();
		const control = (await field.element()) as HTMLInputElement;
		expect(control.inputMode).toBe('numeric');
		expect(control.autocomplete).toBe('one-time-code');
		expect(control.maxLength).toBe(6);
	});

	it('reports what is typed', async () => {
		const onInput = vi.fn();
		await render(TotpCodeField, { id: 'code', value: '', onInput });

		await page.getByLabelText('Authenticator app code').fill('123456');

		expect(onInput).toHaveBeenCalledWith('123456');
	});

	it('shows the value it is given', async () => {
		await render(TotpCodeField, { id: 'code', value: '654321', onInput: vi.fn() });

		const control = (await page
			.getByLabelText('Authenticator app code')
			.element()) as HTMLInputElement;
		expect(control.value).toBe('654321');
	});

	it('links a refusal to the field, GOV.UK error-message position', async () => {
		await render(TotpCodeField, {
			id: 'code',
			value: '',
			onInput: vi.fn(),
			error: 'The code is not correct. Enter the 6-digit code from your authenticator app.'
		});

		await expect
			.element(page.getByText('The code is not correct. Enter the 6-digit code from your authenticator app.'))
			.toBeVisible();
		const control = (await page
			.getByLabelText('Authenticator app code')
			.element()) as HTMLInputElement;
		expect(control.getAttribute('aria-invalid')).toBe('true');
	});
});
