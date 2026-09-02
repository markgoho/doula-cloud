import type { ComponentProps } from 'svelte';
import { createRawSnippet } from 'svelte';
import { page } from 'vitest/browser';
import { describe, expect, it } from 'vitest';
import { render } from 'vitest-browser-svelte';
import ListPage from './ListPage.svelte';
// The loading Skeleton reserves space with `var(--text-body-size)`, which
// only exists once the tokens are loaded -- the real app loads them in the
// root layout. See practice-landing.svelte.spec.ts's identical import.
import '#lib/styles/app.css';

function textSnippet(text: string) {
	return createRawSnippet(() => ({ render: () => `<p>${text}</p>` }));
}

type SetupOptions = Partial<ComponentProps<typeof ListPage>>;

async function setup(overrides: SetupOptions = {}) {
	return render(ListPage, {
		props: {
			title: 'Staff',
			content: textSnippet('A table of Staff members'),
			...overrides
		}
	});
}

describe('ListPage.svelte', () => {
	it('renders the title as the page h1', async () => {
		await setup();

		await expect.element(page.getByRole('heading', { level: 1, name: 'Staff' })).toBeVisible();
	});

	it('always renders the content region', async () => {
		await setup();

		await expect.element(page.getByText('A table of Staff members')).toBeVisible();
	});

	it('omits the intro and actions regions when neither is given', async () => {
		await setup();

		await expect.element(page.getByText('Invite a Staff member')).not.toBeInTheDocument();
	});

	it('renders the intro between the title and the actions', async () => {
		await setup({
			intro: textSnippet('Work states are self-reported'),
			actions: textSnippet('Invite a Staff member')
		});

		const intro = page.getByText('Work states are self-reported').element();
		const actions = page.getByText('Invite a Staff member').element();

		expect(intro.compareDocumentPosition(actions) & Node.DOCUMENT_POSITION_FOLLOWING).toBeTruthy();
	});

	it('renders the actions region between the intro and the content', async () => {
		await setup({
			intro: textSnippet('Work states are self-reported'),
			actions: textSnippet('Invite a Staff member')
		});

		const actions = page.getByText('Invite a Staff member').element();
		const content = page.getByText('A table of Staff members').element();

		expect(
			actions.compareDocumentPosition(content) & Node.DOCUMENT_POSITION_FOLLOWING
		).toBeTruthy();
	});

	it('caps the page at no width, unlike a form, and renders no chrome', async () => {
		const { container } = await setup();

		expect(container.querySelector('center-l')).toHaveAttribute('max', 'none');
		expect(container.querySelector('center-l')).toHaveAttribute('gutters', 'var(--page-gutter)');
		expect(container.querySelector('nav')).toBeNull();
		expect(container.querySelector('header')).toBeNull();
	});

	it('renders the title and a skeleton while loading, instead of the content (#480)', async () => {
		const { container } = await setup({ loading: 'Loading Staff' });

		expect(container.querySelector('center-l')).toHaveAttribute('max', 'none');
		await expect.element(page.getByRole('heading', { level: 1, name: 'Staff' })).toBeVisible();
		await expect.element(page.getByRole('status', { name: 'Loading Staff' })).toBeVisible();
		await expect.element(page.getByText('A table of Staff members')).not.toBeInTheDocument();
	});

	it('renders the title and a Notice on a load failure, instead of the content (#480)', async () => {
		const { container } = await setup({ loadError: 'Failed to load Staff' });

		expect(container.querySelector('center-l')).toHaveAttribute('max', 'none');
		await expect.element(page.getByRole('heading', { level: 1, name: 'Staff' })).toBeVisible();
		await expect.element(page.getByText('Failed to load Staff')).toBeVisible();
		await expect.element(page.getByText('A table of Staff members')).not.toBeInTheDocument();
	});

	it('prefers loadError over loading when both are somehow given', async () => {
		await setup({ loadError: 'Failed to load Staff', loading: 'Loading Staff' });

		await expect.element(page.getByText('Failed to load Staff')).toBeVisible();
		await expect.element(page.getByRole('status', { name: 'Loading Staff' })).not.toBeInTheDocument();
	});
});
