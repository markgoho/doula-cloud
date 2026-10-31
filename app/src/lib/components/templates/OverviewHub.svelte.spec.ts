import type { ComponentProps } from 'svelte';
import { createRawSnippet } from 'svelte';
import { page } from 'vitest/browser';
import { describe, expect, it } from 'vitest';
import { render } from 'vitest-browser-svelte';
import OverviewHub from './OverviewHub.svelte';
// The loading Skeleton reserves space with `var(--text-body-size)`, which
// only exists once the tokens are loaded -- the real app loads them in the
// root layout. See practice-landing.svelte.spec.ts's identical import.
import '#lib/styles/app.css';

function textSnippet(text: string) {
	return createRawSnippet(() => ({ render: () => `<p>${text}</p>` }));
}

type SetupOptions = Partial<ComponentProps<typeof OverviewHub>>;

async function setup(overrides: SetupOptions = {}) {
	return render(OverviewHub, {
		title: 'Willow Birth Collective',
		primary: textSnippet('Six open Offers'),
		isEmpty: false,
		empty: textSnippet('Nothing here yet'),
		...overrides
	});
}

describe('OverviewHub.svelte', () => {
	it('renders the title as the page h1', async () => {
		await setup();

		await expect
			.element(page.getByRole('heading', { level: 1, name: 'Willow Birth Collective' }))
			.toBeVisible();
	});

	// #1549: what the act that led here did, before the h1 and focused.
	it('renders the outcome as a focused status before the title', async () => {
		await setup({ outcome: 'You agreed to the Terms of Service for Willow Birth Collective.' });

		const status = page.getByRole('status');
		await expect.element(status).toHaveTextContent('You agreed to the Terms of Service for Willow Birth Collective.');
		await expect.element(status).toHaveFocus();
	});

	it('renders no outcome while the hub loads', async () => {
		await setup({ outcome: 'You agreed.', loading: 'Loading your Practice' });

		await expect.element(page.getByText('You agreed.')).not.toBeInTheDocument();
	});

	it('renders the primary region and not the empty region when there is data', async () => {
		await setup();

		await expect.element(page.getByText('Six open Offers')).toBeVisible();
		await expect.element(page.getByText('Nothing here yet')).not.toBeInTheDocument();
	});

	it('renders the empty region instead of the primary region when isEmpty', async () => {
		await setup({ isEmpty: true });

		await expect.element(page.getByText('Nothing here yet')).toBeVisible();
		await expect.element(page.getByText('Six open Offers')).not.toBeInTheDocument();
	});

	it('still renders the title when empty, so the page still looks like a page', async () => {
		await setup({ isEmpty: true });

		await expect
			.element(page.getByRole('heading', { level: 1, name: 'Willow Birth Collective' }))
			.toBeVisible();
	});

	it('renders no feed region when no feed snippet is given', async () => {
		await setup();

		await expect.element(page.getByText('Six open Offers')).toBeVisible();
	});

	// #486, #2063: the Recent-activity feed sits low on the page, last in
	// the primary column.
	it('renders the feed last in the primary column', async () => {
		await setup({ feed: textSnippet('Mark Goho sent the invoice') });

		expect(page.getByText('Six open Offers').element().nextElementSibling).toBe(
			page.getByText('Mark Goho sent the invoice').element()
		);
	});

	it('renders no feed region when empty, whatever was passed', async () => {
		await setup({ isEmpty: true, feed: textSnippet('Mark Goho sent the invoice') });

		await expect.element(page.getByText('Mark Goho sent the invoice')).not.toBeInTheDocument();
	});

	it('owns the page frame and renders no chrome', async () => {
		const { container } = await setup();

		expect(container.querySelector('center-l')).toHaveAttribute('max', 'none');
		expect(container.querySelector('center-l')).toHaveAttribute('gutters', 'var(--page-gutter)');
		expect(container.querySelector('nav')).toBeNull();
		expect(container.querySelector('header')).toBeNull();
	});

	it('requires isEmpty and empty at the type level', () => {
		// @ts-expect-error -- isEmpty and empty are required props (ADR-0018),
		// and this unused directive is what fails `bun run check` if that ever
		// stops being true.
		const withoutEmptyState: ComponentProps<typeof OverviewHub> = {
			title: 'Willow Birth Collective',
			primary: textSnippet('Six open Offers')
		};

		expect(withoutEmptyState.title).toBe('Willow Birth Collective');
	});

	it('draws the title and a skeleton while loading, instead of the content (#480, #2248)', async () => {
		const { container } = await setup({ loading: 'Loading your Practice' });

		expect(container.querySelector('center-l')).toHaveAttribute('max', 'none');
		await expect.element(page.getByRole('status', { name: 'Loading your Practice' })).toBeVisible();
		await expect
			.element(page.getByRole('heading', { level: 1, name: 'Willow Birth Collective' }))
			.toBeVisible();
		await expect.element(page.getByText('Six open Offers')).not.toBeInTheDocument();
	});

	it('reserves the page frame for a Notice on a load failure, instead of rendering the title or content (#480)', async () => {
		const { container } = await setup({ loadError: 'Failed to load your Practice' });

		expect(container.querySelector('center-l')).toHaveAttribute('max', 'none');
		await expect.element(page.getByText('Failed to load your Practice')).toBeVisible();
		await expect
			.element(page.getByRole('heading', { level: 1, name: 'Willow Birth Collective' }))
			.not.toBeInTheDocument();
	});

	it('prefers loadError over loading when both are somehow given', async () => {
		await setup({ loadError: 'Failed to load your Practice', loading: 'Loading your Practice' });

		await expect.element(page.getByText('Failed to load your Practice')).toBeVisible();
		await expect
			.element(page.getByRole('status', { name: 'Loading your Practice' }))
			.not.toBeInTheDocument();
	});
});
