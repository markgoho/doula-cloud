import { page } from 'vitest/browser';
import { render } from 'vitest-browser-svelte';
import { describe, expect, it, vi } from 'vitest';
import ReadFailure from './ReadFailure.svelte';

async function setup(onRetry = vi.fn(() => Promise.resolve())) {
	await render(ReadFailure, { message: 'Who is on call did not load.', onRetry });
	return { onRetry };
}

describe('ReadFailure', () => {
	it('says what failed in the list\'s own words, with a way to try again', async () => {
		await setup();

		await expect.element(page.getByText('Who is on call did not load.')).toBeVisible();
		await expect.element(page.getByRole('button', { name: 'Try again' })).toBeVisible();
	});

	it('runs the read that failed when "Try again" is pressed', async () => {
		const { onRetry } = await setup();

		await page.getByRole('button', { name: 'Try again' }).click();

		expect(onRetry).toHaveBeenCalledOnce();
	});

	it('shows its loading state while the read runs, and ends it when the read does', async () => {
		const read = Promise.withResolvers<void>();
		const { onRetry } = await setup(vi.fn(() => read.promise));

		await page.getByRole('button', { name: 'Try again' }).click();
		await expect.element(page.getByRole('button', { name: 'Try again' })).toHaveAttribute('aria-busy', 'true');
		read.resolve();

		await vi.waitFor(() => expect(page.getByRole('button', { name: 'Try again' }).element().getAttribute('aria-busy')).not.toBe('true'));
		expect(onRetry).toHaveBeenCalledOnce();
	});
});
