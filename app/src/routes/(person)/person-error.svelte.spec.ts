import { page } from 'vitest/browser';
import { describe, expect, it, vi } from 'vitest';
import { render } from 'vitest-browser-svelte';
import ErrorBoundary from './+error.svelte';

const pageState = vi.hoisted(() => ({ status: 404 }));
vi.mock('$app/state', () => ({ page: pageState }));

async function setup(status = 404) {
	pageState.status = status;
	await render(ErrorBoundary, {});
}

/*
 * #2040: the founder read page's 404 for anybody but the founder lands
 * here, inside the signed-in shell `(person)/+layout.svelte` still draws,
 * so a signed-in person is never told to sign in.
 */
describe('(person)/+error.svelte', () => {
	it('renders the state matching page.status', async () => {
		await setup(404);

		await expect.element(page.getByRole('heading', { name: 'Page not found' })).toBeVisible();
	});

	it('offers the way out to her own account, not to sign in', async () => {
		await setup();

		const link = page.getByRole('link', { name: 'Go to your account' });
		await expect.element(link).toBeVisible();
		expect(link.element()).toHaveAttribute('href', '/account');
		await expect.element(page.getByRole('link', { name: 'Sign in' })).not.toBeInTheDocument();
	});
});
