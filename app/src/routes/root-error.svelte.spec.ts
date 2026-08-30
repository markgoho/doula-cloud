import { page } from 'vitest/browser';
import { describe, expect, it, vi } from 'vitest';
import { render } from 'vitest-browser-svelte';
import ErrorBoundary from './+error.svelte';

const pageState = vi.hoisted(() => ({ status: 404 }));
vi.mock('$app/state', () => ({ page: pageState }));

describe('+error.svelte (root catch-all)', () => {
	it('renders its own signed-out bar, since no route matched and no layout is above it', async () => {
		await render(ErrorBoundary, {});

		await expect.element(page.getByText('Doula Cloud')).toBeVisible();
	});

	it('renders the state matching page.status', async () => {
		pageState.status = 404;
		await render(ErrorBoundary, {});

		await expect.element(page.getByRole('heading', { name: 'Page not found' })).toBeVisible();
	});

	it('offers the way out to log in', async () => {
		await render(ErrorBoundary, {});

		const link = page.getByRole('link', { name: 'Log in' });
		await expect.element(link).toBeVisible();
		expect(link.element()).toHaveAttribute('href', '/login');
	});
});
