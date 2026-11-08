import { page } from 'vitest/browser';
import { render } from 'vitest-browser-svelte';
import { describe, expect, it, vi } from 'vitest';
import EmailAddressField from './EmailAddressField.svelte';

interface SetupOptions {
	value?: string;
	error?: string;
}

async function setup({ value = '', error }: SetupOptions = {}) {
	const onInput = vi.fn();
	await render(EmailAddressField, { id: 'email', value, onInput, error });
	return { onInput };
}

describe('EmailAddressField', () => {
	it('asks for the email address in an email box the browser can fill', async () => {
		await setup({ value: 'maya@example.test' });

		const field = page.getByLabelText('Email address');
		await expect.element(field).toHaveValue('maya@example.test');
		await expect.element(field).toHaveAttribute('type', 'email');
		await expect.element(field).toHaveAttribute('autocomplete', 'username');
		await expect.element(field).toBeRequired();
	});

	it('hands what was typed to onInput', async () => {
		const { onInput } = await setup();

		await page.getByLabelText('Email address').fill('lena@example.test');

		expect(onInput).toHaveBeenLastCalledWith('lena@example.test');
	});

	it('says what is wrong beside the box', async () => {
		await setup({ error: 'Enter your email address' });

		await expect.element(page.getByLabelText('Email address')).toHaveAccessibleDescription(
			'Enter your email address'
		);
		await expect.element(page.getByRole('alert')).toBeVisible();
	});
});
