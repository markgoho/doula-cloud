import type { ComponentProps } from 'svelte';
import { page } from 'vitest/browser';
import { describe, expect, it, vi } from 'vitest';
import { render } from 'vitest-browser-svelte';
import Checkbox from './Checkbox.svelte';

type SetupOptions = Partial<Omit<ComponentProps<typeof Checkbox>, 'onChange'>>;

async function setup({ checked = false, ...rest }: SetupOptions = {}) {
	const onChange = vi.fn();
	await render(Checkbox, { checked, onChange, ...rest });
	return { onChange };
}

describe('Checkbox.svelte', () => {
	it('renders a checkbox reflecting the given checked state', async () => {
		await setup({ checked: true });

		await expect.element(page.getByRole('checkbox')).toBeChecked();
	});

	it('calls onChange with the new checked value when clicked', async () => {
		const { onChange } = await setup({ checked: false });

		await page.getByRole('checkbox').click();

		expect(onChange).toHaveBeenCalledWith(true);
	});

	it('auto-generates a distinct id per instance, so external labels can target each safely', async () => {
		const { unmount } = await render(Checkbox, { checked: false, onChange: vi.fn() });
		const firstId = page.getByRole('checkbox').element().id;
		await unmount();

		await render(Checkbox, { checked: false, onChange: vi.fn() });
		const secondId = page.getByRole('checkbox').element().id;

		expect(firstId).not.toBe('');
		expect(secondId).not.toBe(firstId);
	});

	it('accepts a caller-supplied id', async () => {
		await setup({ id: 'consent-photos' });

		await expect.element(page.getByRole('checkbox')).toHaveAttribute('id', 'consent-photos');
	});

	it('reflects disabled', async () => {
		await setup({ disabled: true });

		await expect.element(page.getByRole('checkbox')).toBeDisabled();
	});

	it('reflects required', async () => {
		await setup({ required: true });

		await expect.element(page.getByRole('checkbox')).toHaveAttribute('required');
	});

	it('sets aria-invalid=false by default', async () => {
		await setup();

		await expect.element(page.getByRole('checkbox')).toHaveAttribute('aria-invalid', 'false');
	});

	it('sets aria-invalid=true when invalid', async () => {
		await setup({ invalid: true });

		await expect.element(page.getByRole('checkbox')).toHaveAttribute('aria-invalid', 'true');
	});

	it('wires aria-describedby to an external error message for accessible announcement', async () => {
		await setup({ invalid: true, describedBy: 'consent-error' });

		await expect
			.element(page.getByRole('checkbox'))
			.toHaveAttribute('aria-describedby', 'consent-error');
	});

	it('omits aria-describedby when not provided', async () => {
		await setup();

		await expect.element(page.getByRole('checkbox')).not.toHaveAttribute('aria-describedby');
	});
});
