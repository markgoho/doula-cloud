import { page } from 'vitest/browser';
import { describe, expect, it } from 'vitest';
import { render } from 'vitest-browser-svelte';
import BrandLockup from './BrandLockup.svelte';

async function setup({ size }: { size?: 'sm' | 'md' | 'lg' } = {}) {
	await render(BrandLockup, { size });
}

describe('BrandLockup', () => {
	it('writes the product name once, so #338 can change it in one place', async () => {
		await setup();

		await expect.element(page.getByText('Doula Cloud')).toBeVisible();
	});

	it.each(['sm', 'md', 'lg'] as const)('renders at %s', async (size) => {
		await setup({ size });

		await expect.element(page.getByText('Doula Cloud')).toBeVisible();
	});

	/*
	 * The mark beside it is decorative here on purpose: naming it would make
	 * a screen reader say the product's name twice in a row.
	 */
	it('names the product once between the mark and the words', async () => {
		await setup();

		await expect.element(page.getByRole('img', { name: 'Doula Cloud' })).not.toBeInTheDocument();
	});
});
