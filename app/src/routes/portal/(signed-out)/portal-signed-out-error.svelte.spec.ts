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

describe('portal/(signed-out)/+error.svelte', () => {
	it('renders the state matching page.status, inside the signed-out Portal chrome the layout above it still provides', async () => {
		await setup(403);

		await expect.element(page.getByRole('heading', { name: 'You cannot view this' })).toBeVisible();
	});

	it('offers the way out to log in', async () => {
		await setup(404);

		const link = page.getByRole('link', { name: 'Log in' });
		await expect.element(link).toBeVisible();
		expect(link.element()).toHaveAttribute('href', '/portal/login');
	});
});
