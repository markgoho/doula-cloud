import type { ComponentProps } from 'svelte';
import { page } from 'vitest/browser';
import { describe, expect, it } from 'vitest';
import { render } from 'vitest-browser-svelte';
import Heading from './Heading.svelte';

type SetupOptions = Partial<ComponentProps<typeof Heading>>;

async function setup({ level = 1, text = 'Page title', ...rest }: SetupOptions = {}) {
	const { container } = await render(Heading, { level, text, ...rest });
	return { container };
}

describe('Heading.svelte', () => {
	it('renders the text as visible content', async () => {
		await setup({ text: 'Page title' });

		await expect.element(page.getByText('Page title')).toBeVisible();
	});

	it('renders the matching heading tag and level class for each level', async () => {
		for (const level of [1, 2, 3, 4, 5, 6] as const) {
			const { container } = await setup({ level });

			const element = container.querySelector(`h${level}.level-${level}`);
			expect(element).toBeInTheDocument();
		}
	});
});
