import { page } from 'vitest/browser';
import { describe, expect, it, vi } from 'vitest';
import { render } from 'vitest-browser-svelte';
import { registerLayoutPrimitives } from '#lib/primitives/index.js';
import SecondFactorRecommendation from './SecondFactorRecommendation.svelte';

if (!customElements.get('stack-l')) registerLayoutPrimitives();

async function setup({ onClose = vi.fn(() => Promise.resolve()) }: { onClose?: () => Promise<void> } = {}) {
	await render(SecondFactorRecommendation, {
		props: { returnTo: '/practices/p-1', onClose }
	});
	return { onClose };
}

describe('SecondFactorRecommendation.svelte', () => {
	it('is a region named by its heading, and says what the second step is', async () => {
		await setup();

		await expect
			.element(page.getByRole('region', { name: 'Add a second step when you sign in' }))
			.toBeVisible();
		await expect.element(page.getByText(/you type a code from an app on your phone/)).toBeVisible();
	});

	// #1533: the five acts ADR-0026's amendment for #1492 names, in plain
	// words.
	it('names the five acts that need a second factor', async () => {
		await setup();

		const items = page.getByRole('listitem');
		await expect.element(items.nth(4)).toBeVisible();
		expect(items.elements().map((item) => item.textContent)).toEqual([
			"download all of your Practice's data",
			'delete your Practice',
			"erase a Client's records",
			'help a Staff member who is locked out get back in',
			'make two-factor authentication a must for all your Staff'
		]);
	});

	it('has one link, to enrollment, which comes back to the screen she is on', async () => {
		await setup();

		const link = page.getByRole('link', { name: 'Set up two-factor authentication' });
		await expect.element(link).toHaveAttribute('href', '/mfa/enroll?returnTo=%2Fpractices%2Fp-1');
		expect(page.getByRole('link').elements()).toHaveLength(1);
	});

	it('asks the caller to record the close', async () => {
		const { onClose } = await setup();

		await page.getByRole('button', { name: 'Close' }).click();

		expect(onClose).toHaveBeenCalledOnce();
	});

	it('says so when the close did not work, and stays', async () => {
		await setup({ onClose: () => Promise.reject(new Error('nope')) });

		await page.getByRole('button', { name: 'Close' }).click();

		await expect.element(page.getByRole('alert')).toHaveTextContent('Could not close this message. Try again.');
		await expect.element(page.getByRole('button', { name: 'Close' })).toBeVisible();
	});
});
