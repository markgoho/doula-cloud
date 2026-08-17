import type { ComponentProps } from 'svelte';
import { page } from 'vitest/browser';
import { describe, expect, it, vi } from 'vitest';
import { render } from 'vitest-browser-svelte';
import Button from './Button.svelte';

type SetupOptions = Partial<ComponentProps<typeof Button>>;

async function setup({ label = 'Save', ...rest }: SetupOptions = {}) {
	const onClick = vi.fn();
	const { container } = await render(Button, { label, onClick, ...rest });
	return { onClick, container };
}

function icon(container: HTMLElement) {
	return container.querySelector('svg');
}

describe('Button.svelte', () => {
	it('renders the label as visible, accessible text by default', async () => {
		await setup({ label: 'Save' });

		await expect.element(page.getByRole('button', { name: 'Save' })).toBeVisible();
	});

	it('calls onClick when clicked', async () => {
		const { onClick } = await setup();

		await page.getByRole('button').click();

		expect(onClick).toHaveBeenCalledOnce();
	});

	it('defaults to type=button', async () => {
		await setup();

		await expect.element(page.getByRole('button')).toHaveAttribute('type', 'button');
	});

	it('reflects a non-default type', async () => {
		await setup({ type: 'submit' });

		await expect.element(page.getByRole('button')).toHaveAttribute('type', 'submit');
	});

	it('reflects disabled', async () => {
		await setup({ disabled: true });

		await expect.element(page.getByRole('button')).toBeDisabled();
	});

	it('renders an icon alongside the label when icon is given', async () => {
		const { container } = await setup({ icon: 'check' });

		await expect.element(page.getByRole('button', { name: 'Save' })).toBeVisible();
		expect(icon(container)).toBeVisible();
	});

	it('renders no icon when icon is omitted', async () => {
		const { container } = await setup();

		expect(icon(container)).not.toBeInTheDocument();
	});

	it('sizes the icon down for a small button', async () => {
		const { container } = await setup({ size: 'sm', icon: 'check' });

		expect(icon(container)).toHaveAttribute('width', '16');
	});

	it('sizes the icon up for a large button', async () => {
		const { container } = await setup({ size: 'lg', icon: 'check' });

		expect(icon(container)).toHaveAttribute('width', '24');
	});

	it('renders the light weight even at a size that would otherwise default to duotone', async () => {
		const { container } = await setup({ size: 'lg', icon: 'check' });

		expect(container.querySelectorAll(':scope svg path')).toHaveLength(1);
	});

	it('hides the visible label and exposes it as the accessible name when iconOnly', async () => {
		const { container } = await setup({ icon: 'check', iconOnly: true, label: 'Confirm' });

		await expect.element(page.getByRole('button', { name: 'Confirm' })).toBeVisible();
		expect(container.querySelector(':scope button > span')).not.toBeInTheDocument();
	});

	it('omits aria-label when not iconOnly', async () => {
		await setup();

		await expect.element(page.getByRole('button')).not.toHaveAttribute('aria-label');
	});

	it('is disabled and marked aria-busy when loading', async () => {
		await setup({ loading: true });

		const button = page.getByRole('button');
		await expect.element(button).toBeDisabled();
		await expect.element(button).toHaveAttribute('aria-busy', 'true');
	});

	it('sets aria-busy=false when not loading', async () => {
		await setup();

		await expect.element(page.getByRole('button')).toHaveAttribute('aria-busy', 'false');
	});

	it('renders a spinner instead of the icon when loading', async () => {
		const { container } = await setup({ icon: 'check', loading: true });

		expect(container.querySelector('.spinner')).toBeVisible();
		expect(icon(container)).not.toBeInTheDocument();
	});
});
