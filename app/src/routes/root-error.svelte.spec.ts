import { page } from 'vitest/browser';
import { describe, expect, it, vi } from 'vitest';
import { render } from 'vitest-browser-svelte';
import { expectPrivacyLinkAfterMain } from '#lib/testPrivacyLink.js';
import ErrorBoundary from './+error.svelte';

const pageState = vi.hoisted(() => ({ status: 404, url: new URL('http://localhost/') }));
vi.mock('$app/state', () => ({ page: pageState }));

async function setup(status = 404, pathname = '/no-such-page') {
	pageState.status = status;
	pageState.url = new URL(pathname, 'http://localhost');
	await render(ErrorBoundary, {});
}

describe('+error.svelte (root catch-all)', () => {
	it('renders its own signed-out bar, since no route matched and no layout is above it', async () => {
		await setup();

		await expect.element(page.getByText('DoulaCloud')).toBeVisible();
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

	/*
	 * #1558: a Portal 404 or 500 is still a Portal screen a Client lands
	 * on, and no Portal layout is above this boundary to draw the footer.
	 */
	it.each([
		[404, '/portal'],
		[404, '/portal/no-such-page'],
		[500, '/portal/engagements/engagement-1']
	])('carries the Portal footer on a %i under %s', async (status, pathname) => {
		await setup(status, pathname);

		await expectPrivacyLinkAfterMain();
	});

	it.each(['/no-such-page', '/practices/practice-1', '/portalish'])(
		'carries no Portal footer on an error at %s',
		async (pathname) => {
			await setup(404, pathname);

			await expect.element(page.getByRole('main')).toBeVisible();
			await expect.element(page.getByRole('contentinfo')).not.toBeInTheDocument();
		}
	);
});
