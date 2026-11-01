import { describe, expect, it } from 'vitest';
import { render } from 'vitest-browser-svelte';
import { page } from 'vitest/browser';
import StepUpFields from './StepUpFields.svelte';
import { STEP_UP_CODE_FIELD, STEP_UP_PASSWORD_FIELD } from '../stepUp.ts';

describe('StepUpFields', () => {
	it('asks for the password and a code from the authenticator app', async () => {
		render(StepUpFields, { errors: [] });

		await expect.element(page.getByLabelText('Password')).toHaveAttribute('id', STEP_UP_PASSWORD_FIELD);
		await expect.element(page.getByLabelText('Password')).toHaveAttribute('autocomplete', 'current-password');
		const code = page.getByLabelText('Authenticator app code');
		await expect.element(code).toHaveAttribute('id', STEP_UP_CODE_FIELD);
		await expect.element(code).toHaveAttribute('autocomplete', 'one-time-code');
		await expect.element(code).toHaveAttribute('inputmode', 'numeric');
		await expect.element(page.getByText('This act needs a new code, even though you are signed in.')).toBeVisible();
	});

	it('shows a refused code in plain words beside the code', async () => {
		render(StepUpFields, {
			errors: [{ message: 'Enter the 6-digit code your authenticator app shows now', field: STEP_UP_CODE_FIELD }]
		});

		const code = page.getByLabelText('Authenticator app code');
		await expect.element(code).toHaveAttribute('aria-invalid', 'true');
		await expect.element(page.getByText('Enter the 6-digit code your authenticator app shows now')).toBeVisible();
		await expect.element(page.getByLabelText('Password')).not.toHaveAttribute('aria-invalid');
	});

	it('keeps what the operator types', async () => {
		render(StepUpFields, { errors: [] });

		await page.getByLabelText('Password').fill('a password');
		await page.getByLabelText('Authenticator app code').fill('123456');

		await expect.element(page.getByLabelText('Password')).toHaveValue('a password');
		await expect.element(page.getByLabelText('Authenticator app code')).toHaveValue('123456');
	});

	it('shows a refused password beside the password', async () => {
		render(StepUpFields, { errors: [{ message: 'Enter the correct password', field: STEP_UP_PASSWORD_FIELD }] });

		await expect.element(page.getByLabelText('Password')).toHaveAttribute('aria-invalid', 'true');
		await expect.element(page.getByText('Enter the correct password')).toBeVisible();
	});
});
