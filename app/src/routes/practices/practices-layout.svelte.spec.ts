import { createRawSnippet } from 'svelte';
import { page } from 'vitest/browser';
import { describe, expect, it } from 'vitest';
import { render } from 'vitest-browser-svelte';
import Layout from './+layout.svelte';

function childSnippet(text: string) {
	return createRawSnippet(() => ({
		render: () => `<p>${text}</p>`
	}));
}

describe('Staff authenticated layout', () => {
	it('renders its children', async () => {
		await render(Layout, { children: childSnippet('staff child content') });

		await expect.element(page.getByText('staff child content')).toBeVisible();
	});
});
