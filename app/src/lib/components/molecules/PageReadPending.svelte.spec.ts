import type { ComponentProps } from 'svelte';
import { page } from 'vitest/browser';
import { describe, expect, it } from 'vitest';
import { render } from 'vitest-browser-svelte';
import PageReadPending from './PageReadPending.svelte';

type Properties = ComponentProps<typeof PageReadPending>;

async function setup(read: Properties['read']) {
	await render(PageReadPending, { props: { title: 'Clients', read, skeleton: 'row' } });
}

describe('PageReadPending.svelte', () => {
	it('titles a failed read and says why it failed', async () => {
		await setup({ status: 'failed', message: 'Your Clients did not load.' });

		await expect.element(page.getByRole('heading', { level: 1, name: 'Clients' })).toBeVisible();
		await expect.element(page.getByRole('alert')).toHaveTextContent('Your Clients did not load.');
	});

	it('titles a loading read and names what is loading', async () => {
		await setup({ status: 'loading', label: 'Loading your Clients' });

		await expect.element(page.getByRole('heading', { level: 1, name: 'Clients' })).toBeVisible();
		await expect
			.element(page.getByRole('status', { name: 'Loading your Clients' }))
			.toHaveAttribute('aria-busy', 'true');
	});
});
