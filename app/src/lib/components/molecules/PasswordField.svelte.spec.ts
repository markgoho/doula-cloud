import { page } from 'vitest/browser';
import { render } from 'vitest-browser-svelte';
import { describe, expect, it, vi } from 'vitest';
import PasswordField from './PasswordField.svelte';

interface SetupOptions {
	value?: string;
	error?: string;
}

async function setup({ value = '', error }: SetupOptions = {}) {
	const onInput = vi.fn();
	await render(PasswordField, { id: 'password', value, onInput, error });
	return { onInput };
}

describe('PasswordField', () => {
	it('asks for the password in a masked box the password manager can fill', async () => {
		await setup();

		const field = page.getByLabelText('Password');
		await expect.element(field).toHaveAttribute('type', 'password');
		await expect.element(field).toHaveAttribute('autocomplete', 'current-password');
		await expect.element(field).toBeRequired();
	});

	it('hands what was typed to onInput', async () => {
		const { onInput } = await setup();

		await page.getByLabelText('Password').fill('correct horse');

		expect(onInput).toHaveBeenLastCalledWith('correct horse');
	});

	it('says what is wrong beside the box', async () => {
		await setup({ error: 'Enter your password' });

		await expect.element(page.getByLabelText('Password')).toHaveAccessibleDescription('Enter your password');
		await expect.element(page.getByRole('alert')).toBeVisible();
	});
});
