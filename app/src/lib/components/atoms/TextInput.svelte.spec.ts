import type { ComponentProps } from 'svelte';
import { page } from 'vitest/browser';
import { describe, expect, it, vi } from 'vitest';
import { render } from 'vitest-browser-svelte';
import TextInput from './TextInput.svelte';

type SetupOptions = Partial<Omit<ComponentProps<typeof TextInput>, 'onInput'>>;

async function setup({ value = '', ...rest }: SetupOptions = {}) {
	const onInput = vi.fn();
	await render(TextInput, { value, onInput, ...rest });
	return { onInput };
}

describe('TextInput.svelte', () => {
	it('renders a textbox with the given value', async () => {
		await setup({ value: 'Alex' });

		await expect.element(page.getByRole('textbox')).toHaveValue('Alex');
	});

	it('calls onInput with the new value when typed into', async () => {
		const { onInput } = await setup();

		await page.getByRole('textbox').fill('Jordan');

		expect(onInput).toHaveBeenCalledWith('Jordan');
	});

	it('defaults to type=text', async () => {
		await setup();

		await expect.element(page.getByRole('textbox')).toHaveAttribute('type', 'text');
	});

	it('reflects a non-default type', async () => {
		await setup({ type: 'email' });

		await expect.element(page.getByRole('textbox')).toHaveAttribute('type', 'email');
	});

	it('auto-generates a distinct id per instance, so external labels can target each safely', async () => {
		const { unmount } = await render(TextInput, { value: '', onInput: vi.fn() });
		const firstId = page.getByRole('textbox').element().id;
		await unmount();

		await render(TextInput, { value: '', onInput: vi.fn() });
		const secondId = page.getByRole('textbox').element().id;

		expect(firstId).not.toBe('');
		expect(secondId).not.toBe(firstId);
	});

	it('accepts a caller-supplied id', async () => {
		await setup({ id: 'client-first-name' });

		await expect.element(page.getByRole('textbox')).toHaveAttribute('id', 'client-first-name');
	});

	it('reflects disabled', async () => {
		await setup({ disabled: true });

		await expect.element(page.getByRole('textbox')).toBeDisabled();
	});

	it('reflects required', async () => {
		await setup({ required: true });

		await expect.element(page.getByRole('textbox')).toHaveAttribute('required');
	});

	it('sets aria-invalid=false by default', async () => {
		await setup();

		await expect.element(page.getByRole('textbox')).toHaveAttribute('aria-invalid', 'false');
	});

	it('sets aria-invalid=true when invalid', async () => {
		await setup({ invalid: true });

		await expect.element(page.getByRole('textbox')).toHaveAttribute('aria-invalid', 'true');
	});

	it('wires aria-describedby to an external error message for accessible announcement', async () => {
		await setup({ invalid: true, describedBy: 'first-name-error' });

		await expect.element(page.getByRole('textbox')).toHaveAttribute('aria-describedby', 'first-name-error');
	});

	it('omits aria-describedby when not provided', async () => {
		await setup();

		await expect.element(page.getByRole('textbox')).not.toHaveAttribute('aria-describedby');
	});
});
