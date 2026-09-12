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

describe('+error.svelte (root catch-all)', () => {
	it('renders its own signed-out bar, since no route matched and no layout is above it', async () => {
		await setup();

		await expect.element(page.getByText('Doula Cloud')).toBeVisible();
	});

	it('renders the state matching page.status', async () => {
		await setup(404);

		await expect.element(page.getByRole('heading', { name: 'Page not found' })).toBeVisible();
	});

	it('offers the way out to log in', async () => {
		await setup();

		const link = page.getByRole('link', { name: 'Log in' });
		await expect.element(link).toBeVisible();
		expect(link.element()).toHaveAttribute('href', '/login');
	});
});
