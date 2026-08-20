import type { ComponentProps } from 'svelte';
import { page } from 'vitest/browser';
import { describe, expect, it } from 'vitest';
import { render } from 'vitest-browser-svelte';
import Text from './Text.svelte';

type SetupOptions = Partial<ComponentProps<typeof Text>>;

async function setup({ text = 'Body copy', ...rest }: SetupOptions = {}) {
	const { container } = await render(Text, { text, ...rest });
	return { container };
}

describe('Text.svelte', () => {
	it('renders the text as visible content', async () => {
		await setup({ text: 'Body copy' });

		await expect.element(page.getByText('Body copy')).toBeVisible();
	});

	it('applies a size-matching class for each size', async () => {
		for (const size of ['sm', 'base', 'lg'] as const) {
			const { container } = await setup({ size });

			expect(container.querySelector(`p.size-${size}`)).toBeInTheDocument();
		}
	});

	it('applies the muted class when muted is true', async () => {
		const { container } = await setup({ muted: true });

		expect(container.querySelector('p.muted')).toBeInTheDocument();
	});

	it('omits the muted class when muted is false', async () => {
		const { container } = await setup({ muted: false });

		expect(container.querySelector('p.muted')).not.toBeInTheDocument();
	});
});
