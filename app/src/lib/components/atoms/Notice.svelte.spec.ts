import type { ComponentProps } from 'svelte';
import { page } from 'vitest/browser';
import { describe, expect, it } from 'vitest';
import { render } from 'vitest-browser-svelte';
import Notice from './Notice.svelte';

type SetupOptions = Partial<ComponentProps<typeof Notice>>;

async function setup({ message = 'Something happened', variant = 'info', ...rest }: SetupOptions = {}) {
	const { container } = await render(Notice, { message, variant, ...rest });
	return { container };
}

describe('Notice.svelte', () => {
	it('renders the message as visible text', async () => {
		await setup({ message: 'Missing invite token' });

		await expect.element(page.getByText('Missing invite token')).toBeVisible();
	});

	it('uses role="alert" for the error variant', async () => {
		const { container } = await setup({ variant: 'error' });

		expect(container.querySelector('[role="alert"]')).toBeInTheDocument();
	});

	it('uses role="status" for the status variant', async () => {
		const { container } = await setup({ variant: 'status' });

		expect(container.querySelector('[role="status"]')).toBeInTheDocument();
	});

	it('uses role="status" for the info variant', async () => {
		const { container } = await setup({ variant: 'info' });

		expect(container.querySelector('[role="status"]')).toBeInTheDocument();
	});

	it('applies a variant-matching class', async () => {
		const { container } = await setup({ variant: 'status' });

		expect(container.querySelector('.status')).toBeInTheDocument();
	});

	it('renders a decorative icon for each variant', async () => {
		for (const variant of ['error', 'status', 'info'] as const) {
			const { container } = await setup({ variant });

			const svg = container.querySelector('svg');
			expect(svg).toBeInTheDocument();
			expect(svg).toHaveAttribute('aria-hidden', 'true');
		}
	});
});
