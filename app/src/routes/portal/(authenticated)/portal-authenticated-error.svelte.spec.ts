import { page } from 'vitest/browser';
import { describe, expect, it, vi } from 'vitest';
import { render } from 'vitest-browser-svelte';
import ErrorBoundary from './+error.svelte';

const pageState = vi.hoisted(() => ({
	status: 404,
	params: { engagementId: 'engagement-1' } as { engagementId?: string }
}));
vi.mock('$app/state', () => ({ page: pageState }));

describe('portal/(authenticated)/+error.svelte', () => {
	it('renders the state matching page.status, inside the Portal chrome the layout above it still provides', async () => {
		pageState.status = 500;
		await render(ErrorBoundary, {});

		await expect.element(page.getByRole('heading', { name: 'Sorry, there is a problem' })).toBeVisible();
	});

	it('offers the way out to this Engagement hub', async () => {
		pageState.status = 404;
		await render(ErrorBoundary, {});

		const link = page.getByRole('link', { name: 'Go to your care' });
		await expect.element(link).toBeVisible();
		expect(link.element()).toHaveAttribute('href', '/portal/engagements/engagement-1');
	});
});
