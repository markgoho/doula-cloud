import { page } from 'vitest/browser';
import { describe, expect, it } from 'vitest';
import { render } from 'vitest-browser-svelte';
import SignedOutTopBar from './SignedOutTopBar.svelte';
// The bar's height is a token, and a component rendered on its own does
// not otherwise get the stylesheet that declares one.
import '#lib/styles/tokens.css';

async function setup() {
	await render(SignedOutTopBar);
}

describe('SignedOutTopBar', () => {
	it('carries the lockup and nothing else', async () => {
		await setup();

		await expect.element(page.getByText('Doula Cloud')).toBeVisible();
		await expect.element(page.getByRole('navigation')).not.toBeInTheDocument();
		await expect.element(page.getByRole('button')).not.toBeInTheDocument();
	});

	/*
	 * The whole point of the reduced bar: it is the same height and behind
	 * the same hairline as the real shell, so signing in fills the bar in
	 * rather than making one appear and pushing the page down.
	 */
	it('is the shell height, so nothing reflows at sign-in', async () => {
		await setup();

		const banner = page.getByRole('banner');
		await expect.element(banner).toBeVisible();
		expect(getComputedStyle(banner.element()).blockSize).toBe('60px');
	});
});
