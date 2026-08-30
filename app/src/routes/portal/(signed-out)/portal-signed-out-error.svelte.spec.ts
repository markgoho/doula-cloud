import { page } from 'vitest/browser';
import { describe, expect, it, vi } from 'vitest';
import { render } from 'vitest-browser-svelte';
import ErrorBoundary from './+error.svelte';

const pageState = vi.hoisted(() => ({ status: 404 }));
vi.mock('$app/state', () => ({ page: pageState }));

describe('portal/(signed-out)/+error.svelte', () => {
	it('renders the state matching page.status, inside the signed-out Portal chrome the layout above it still provides', async () => {
		pageState.status = 403;
		await render(ErrorBoundary, {});

		await expect.element(page.getByRole('heading', { name: 'You cannot view this' })).toBeVisible();
	});

	it('offers the way out to log in', async () => {
		pageState.status = 404;
		await render(ErrorBoundary, {});

		const link = page.getByRole('link', { name: 'Log in' });
		await expect.element(link).toBeVisible();
		expect(link.element()).toHaveAttribute('href', '/portal/login');
	});
});
