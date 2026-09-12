import { page } from 'vitest/browser';
import { describe, expect, it, vi } from 'vitest';
import { render } from 'vitest-browser-svelte';
import ErrorBoundary from './+error.svelte';

const pageState = vi.hoisted(() => ({
	status: 404,
	params: { engagementId: 'engagement-1' } as { engagementId?: string }
}));
vi.mock('$app/state', () => ({ page: pageState }));

async function setup(status = 404) {
	pageState.status = status;
	await render(ErrorBoundary, {});
}

describe('portal/(authenticated)/+error.svelte', () => {
	it('renders the state matching page.status, inside the Portal chrome the layout above it still provides', async () => {
		await setup(500);

		await expect.element(page.getByRole('heading', { name: 'Sorry, there is a problem' })).toBeVisible();
	});

	it('offers the way out to this Engagement hub', async () => {
		await setup(404);

		const link = page.getByRole('link', { name: 'Go to your care' });
		await expect.element(link).toBeVisible();
		expect(link.element()).toHaveAttribute('href', '/portal/engagements/engagement-1');
	});
});
