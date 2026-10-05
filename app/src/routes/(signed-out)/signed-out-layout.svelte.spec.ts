import { createRawSnippet } from 'svelte';
import { page } from 'vitest/browser';
import { describe, expect, it, vi } from 'vitest';
import { render } from 'vitest-browser-svelte';
import { expectPrivacyLinkAfterMain } from '#lib/testPrivacyLink.js';
import type { RootLanding } from './+page.js';
import Layout from './+layout.svelte';

// `data` stands in for `/`'s own load result (`+page.ts`'s `RootLanding`);
// every other screen in this group loads no `type` at all.
type LayoutData = Partial<Pick<RootLanding, 'type'>>;

const pageState = vi.hoisted(() => ({
	params: {},
	url: new URL('http://localhost/'),
	data: {}
}));
vi.mock('$app/state', () => ({ page: pageState }));

async function setup(data: LayoutData) {
	pageState.data = data;
	await render(Layout, {
		children: createRawSnippet(() => ({ render: () => '<p>child content</p>' }))
	});
}

describe('the signed-out shell', () => {
	it('renders its children inside the main landmark', async () => {
		await setup({});

		await expect.element(page.getByRole('main').getByText('child content')).toBeVisible();
	});

	/*
	 * #1558: `/`'s portal picker is a Portal screen -- a signed-in Client
	 * choosing an Engagement, or told she has none -- so it carries the
	 * Portal's own footer, after `<main>`, as every other Portal screen does.
	 */
	it('carries the Portal footer on the portal picker at /', async () => {
		await setup({ type: 'portal-picker' });

		await expectPrivacyLinkAfterMain();
	});

	it.each<LayoutData>([{ type: 'staff-picker' }, { type: 'signed-out' }, {}])(
		'carries no Portal footer on a screen a Client is not reading (%o)',
		async (data) => {
			await setup(data);

			await expect.element(page.getByText('child content')).toBeVisible();
			await expect.element(page.getByRole('contentinfo')).not.toBeInTheDocument();
		}
	);
});
