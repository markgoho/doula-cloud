import type { ComponentProps } from 'svelte';
import { page } from 'vitest/browser';
import { describe, expect, it } from 'vitest';
import { render } from 'vitest-browser-svelte';
import Link from './Link.svelte';

type SetupOptions = Partial<ComponentProps<typeof Link>>;

async function setup({ label = 'Docs', href = '/docs', ...rest }: SetupOptions = {}) {
	const { container } = await render(Link, { label, href, ...rest });
	return { container };
}

describe('Link.svelte', () => {
	it('renders the label as a visible link', async () => {
		await setup({ label: 'Docs' });

		await expect.element(page.getByRole('link', { name: 'Docs' })).toBeVisible();
	});

	it('renders the given href', async () => {
		await setup({ href: '/practices/1' });

		await expect.element(page.getByRole('link')).toHaveAttribute('href', '/practices/1');
	});

	it('defaults to the primary variant class', async () => {
		await setup();

		await expect.element(page.getByRole('link')).toHaveClass('primary');
	});

	it('reflects a non-default variant', async () => {
		await setup({ variant: 'secondary' });

		await expect.element(page.getByRole('link')).toHaveClass('secondary');
	});

	it('does not set target or rel for an internal (relative) href', async () => {
		await setup({ href: '/practices/1' });

		await expect.element(page.getByRole('link')).not.toHaveAttribute('target');
		await expect.element(page.getByRole('link')).not.toHaveAttribute('rel');
	});

	it('opens an absolute http(s) href in a new tab with a safe rel', async () => {
		await setup({ href: 'https://example.com' });

		await expect.element(page.getByRole('link')).toHaveAttribute('target', '_blank');
		await expect.element(page.getByRole('link')).toHaveAttribute('rel', 'noopener noreferrer');
	});

	it('treats a protocol-relative href as external', async () => {
		await setup({ href: '//example.com' });

		await expect.element(page.getByRole('link')).toHaveAttribute('target', '_blank');
	});

	it('shows an external-link icon and includes "(opens in new tab)" in the accessible name', async () => {
		const { container } = await setup({ href: 'https://example.com', label: 'Visit site' });

		expect(container.querySelector('svg')).toBeVisible();
		await expect
			.element(page.getByRole('link', { name: 'Visit site (opens in new tab)' }))
			.toBeVisible();
	});

	it('renders no icon and no "opens in new tab" text for an internal link', async () => {
		const { container } = await setup({ href: '/docs' });

		expect(container.querySelector('svg')).not.toBeInTheDocument();
		await expect
			.element(page.getByRole('link', { name: 'Docs (opens in new tab)' }))
			.not.toBeInTheDocument();
	});

	it('renders a leading icon and drops the underline when icon is given', async () => {
		const { container } = await setup({ icon: 'users' });

		expect(container.querySelector('svg')).toBeVisible();
		await expect.element(page.getByRole('link')).toHaveClass('has-icon');
	});

	it('has no icon and keeps the underline class off when icon is omitted', async () => {
		await setup();

		await expect.element(page.getByRole('link')).not.toHaveClass('has-icon');
	});

	it('marks the link as the current page when current is true', async () => {
		await setup({ current: true });

		await expect.element(page.getByRole('link')).toHaveAttribute('aria-current', 'page');
	});

	it('has no aria-current when current is false', async () => {
		await setup();

		await expect.element(page.getByRole('link')).not.toHaveAttribute('aria-current');
	});

	it('reflects the card variant class', async () => {
		await setup({ variant: 'card', icon: 'users' });

		await expect.element(page.getByRole('link')).toHaveClass('card');
	});

	it('reflects the rail variant class, for a contents entry', async () => {
		await setup({ variant: 'rail' });

		await expect.element(page.getByRole('link')).toHaveClass('rail');
	});

	it('reflects the chip variant class, for a jump-to target', async () => {
		await setup({ variant: 'chip' });

		await expect.element(page.getByRole('link')).toHaveClass('chip');
	});
});
