import { page } from 'vitest/browser';
import { describe, expect, it, vi } from 'vitest';
import { render } from 'vitest-browser-svelte';
import CredentialField from './CredentialField.svelte';

async function setup({ error }: { error?: string } = {}) {
	const onInput = vi.fn();
	await render(CredentialField, {
		id: 'sign-in-name',
		label: 'Sign-in name',
		type: 'email',
		autocomplete: 'username',
		value: 'maya@example.test',
		onInput,
		error
	});
	return { onInput };
}

describe('CredentialField', () => {
	it('labels a required box of the type and autocomplete token it is given', async () => {
		await setup();

		const field = page.getByLabelText('Sign-in name');
		await expect.element(field).toHaveValue('maya@example.test');
		await expect.element(field).toHaveAttribute('type', 'email');
		await expect.element(field).toHaveAttribute('autocomplete', 'username');
		await expect.element(field).toBeRequired();
	});

	it('hands what was typed on', async () => {
		const { onInput } = await setup();

		await page.getByLabelText('Sign-in name').fill('lena@example.test');

		expect(onInput).toHaveBeenLastCalledWith('lena@example.test');
	});

	it('describes the box by what is wrong with it', async () => {
		await setup({ error: 'Enter your sign-in name' });

		await expect.element(page.getByLabelText('Sign-in name')).toHaveAccessibleDescription('Enter your sign-in name');
	});
});
