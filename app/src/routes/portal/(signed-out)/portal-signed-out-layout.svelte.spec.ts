import { createRawSnippet } from 'svelte';
import { page } from 'vitest/browser';
import { describe, expect, it } from 'vitest';
import { render } from 'vitest-browser-svelte';
import Layout from './+layout.svelte';

async function setup() {
	await render(Layout, {
		children: createRawSnippet(() => ({ render: () => '<p>signed-out child content</p>' }))
	});
}

describe('Client portal signed-out layout', () => {
	it('renders its children inside the main landmark', async () => {
		await setup();

		await expect.element(page.getByRole('main').getByText('signed-out child content')).toBeVisible();
	});

	// #1558: the screens where she first signs in carry the same link, in
	// the same footer, as every signed-in Portal screen.
	it('carries the Privacy Policy link in the footer, after the main content', async () => {
		await setup();

		const link = page.getByRole('contentinfo').getByRole('link', { name: 'Privacy Policy (opens in new tab)' });
		await expect.element(link).toBeVisible();
		await expect.element(link).toHaveAttribute('href', 'https://doula.cloud/privacy');
		const main = page.getByRole('main').element();
		expect(main.compareDocumentPosition(link.element()) & Node.DOCUMENT_POSITION_FOLLOWING).toBeTruthy();
	});
});
