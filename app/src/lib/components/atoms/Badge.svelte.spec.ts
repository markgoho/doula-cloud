import type { ComponentProps } from 'svelte';
import { page } from 'vitest/browser';
import { describe, expect, it } from 'vitest';
import { render } from 'vitest-browser-svelte';
import Badge from './Badge.svelte';

type SetupOptions = Partial<ComponentProps<typeof Badge>>;

async function setup({ label = 'Active', variant = 'success', ...rest }: SetupOptions = {}) {
	const { container } = await render(Badge, { label, variant, ...rest });
	return { container };
}

describe('Badge.svelte', () => {
	it('renders the label as visible text', async () => {
		await setup({ label: 'Active' });

		await expect.element(page.getByText('Active')).toBeVisible();
	});

	it('applies a variant-matching class for each variant', async () => {
		for (const variant of ['info', 'success', 'warning', 'error', 'neutral'] as const) {
			const { container } = await setup({ variant });

			expect(container.querySelector(`span.${variant}`)).toBeInTheDocument();
		}
	});

	it('renders a decorative icon for each variant', async () => {
		for (const variant of ['info', 'success', 'warning', 'error', 'neutral'] as const) {
			const { container } = await setup({ variant });

			const svg = container.querySelector('svg');
			expect(svg).toBeInTheDocument();
			expect(svg).toHaveAttribute('aria-hidden', 'true');
		}
	});
});
