import type { ComponentProps } from 'svelte';
import { page } from 'vitest/browser';
import { describe, expect, it } from 'vitest';
import { render } from 'vitest-browser-svelte';
import ErrorPage from './ErrorPage.svelte';

type SetupOptions = Partial<ComponentProps<typeof ErrorPage>>;

async function setup({
	kind = 'notFound',
	wayOutHref = '/practices/practice-1',
	wayOutLabel = 'Go to your Practice overview',
	...rest
}: SetupOptions = {}) {
	await render(ErrorPage, { kind, wayOutHref, wayOutLabel, ...rest });
}

describe('ErrorPage.svelte', () => {
	it('renders distinct copy for a not-found page, and says trying again will not help', async () => {
		await setup({ kind: 'notFound' });

		await expect.element(page.getByRole('heading', { name: 'Page not found' })).toBeVisible();
		await expect.element(page.getByText('Trying again will not change that.')).toBeVisible();
	});

	it('renders distinct copy for a refused page, and says trying again will not help', async () => {
		await setup({ kind: 'refused' });

		await expect.element(page.getByRole('heading', { name: 'You cannot view this' })).toBeVisible();
		await expect.element(page.getByText('Trying again will not change that.')).toBeVisible();
	});

	it('renders distinct copy for an unavailable page, and says trying again will help', async () => {
		await setup({ kind: 'unavailable' });

		await expect.element(page.getByRole('heading', { name: 'Doula Cloud is unavailable' })).toBeVisible();
		await expect.element(page.getByText('Try again shortly.')).toBeVisible();
	});

	it('renders distinct copy for a problem page, and says trying again will help', async () => {
		await setup({ kind: 'problem' });

		await expect.element(page.getByRole('heading', { name: 'Sorry, there is a problem' })).toBeVisible();
		await expect.element(page.getByText('Try again in a few minutes.')).toBeVisible();
	});

	it('renders the given way-out link', async () => {
		await setup({ wayOutHref: '/portal/(signed-out)/login', wayOutLabel: 'Log in' });

		await expect.element(page.getByRole('link', { name: 'Log in' })).toBeVisible();
		expect(page.getByRole('link', { name: 'Log in' }).element()).toHaveAttribute(
			'href',
			'/portal/(signed-out)/login'
		);
	});
});
