import { page } from 'vitest/browser';
import { describe, expect, it } from 'vitest';
import { render } from 'vitest-browser-svelte';
import Skeleton from './Skeleton.svelte';

interface SetupOptions {
	lines?: number;
	variant?: 'row' | 'text';
	label?: string;
}

async function setup({ lines, variant, label = 'Loading Clients' }: SetupOptions = {}) {
	await render(Skeleton, { lines, variant, label });
	const placeholder = page.getByRole('status', { name: label });

	async function bars(): Promise<HTMLElement[]> {
		const element = await placeholder.element();
		return [...element.querySelectorAll('span')];
	}

	async function barHeight(): Promise<number> {
		const drawn = await bars();
		return drawn[0].getBoundingClientRect().height;
	}

	return { placeholder, bars, barHeight };
}

describe('Skeleton', () => {
	it('announces what is loading, rather than only that something is', async () => {
		const { placeholder } = await setup({ label: 'Loading the Client list' });
		await expect.element(placeholder).toBeVisible();
		await expect.element(placeholder).toHaveAttribute('aria-busy', 'true');
	});

	it('draws one bar per line asked for', async () => {
		const { bars } = await setup({ lines: 7 });
		const drawn = await bars();
		expect(drawn).toHaveLength(7);
	});

	it('draws three bars when no count is given', async () => {
		const { bars } = await setup();
		const drawn = await bars();
		expect(drawn).toHaveLength(3);
	});

	it('reserves a full table row per line in the row variant', async () => {
		const { barHeight } = await setup({ lines: 1, variant: 'row' });
		// 40px: the row height the brief's Density section fixes for a table,
		// and the number that makes this component worth having at all.
		expect(await barHeight()).toBe(40);
	});

	it('reserves only a line of body copy in the text variant', async () => {
		const { barHeight } = await setup({ lines: 1, variant: 'text' });
		expect(await barHeight()).toBeLessThan(40);
	});

	it('hides the bars themselves from assistive technology, since only the status matters', async () => {
		const { bars } = await setup({ lines: 2 });
		const drawn = await bars();
		expect(drawn.every((bar) => bar.getAttribute('aria-hidden') === 'true')).toBe(true);
	});
});
