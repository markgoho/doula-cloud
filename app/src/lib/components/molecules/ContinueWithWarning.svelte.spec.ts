import { page } from 'vitest/browser';
import { describe, expect, it, vi } from 'vitest';
import { render } from 'vitest-browser-svelte';
import ContinueWithWarning from './ContinueWithWarning.svelte';

async function setup({ warning, loading }: { warning?: string; loading?: boolean } = {}) {
	const onClick = vi.fn();
	await render(ContinueWithWarning, { warning, loading, onClick });
	return { onClick };
}

describe('ContinueWithWarning', () => {
	it('is a plain Continue until the BFF has said what continuing costs', async () => {
		const { onClick } = await setup();

		await page.getByRole('button', { name: 'Continue' }).click();

		expect(onClick).toHaveBeenCalledOnce();
		await expect.element(page.getByRole('status')).not.toBeInTheDocument();
	});

	it('shows the warning and names the cost on the button', async () => {
		await setup({ warning: 'Continuing signs you out of your Practice.' });

		await expect.element(page.getByRole('status')).toHaveTextContent('Continuing signs you out of your Practice.');
		await expect.element(page.getByRole('button', { name: 'Continue and sign out' })).toBeVisible();
	});
});
