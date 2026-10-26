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

	it('omits the complementary region when no secondary snippet is given', async () => {
		const { container } = await setup();

		expect(container.querySelector('aside')).toBeNull();
	});

	it('renders the secondary region beside the primary one when given', async () => {
		const { container } = await setup({ secondary: textSnippet('Two doulas unassigned') });

		await expect.element(page.getByText('Two doulas unassigned')).toBeVisible();
		expect(container.querySelector('aside')).toContainElement(
			page.getByText('Two doulas unassigned').element()
		);
	});

	it('renders no secondary region when empty, whatever was passed', async () => {
		const { container } = await setup({ isEmpty: true, secondary: textSnippet('Two doulas unassigned') });

		expect(container.querySelector('aside')).toBeNull();
	});

	it('renders no feed region when no feed snippet is given', async () => {
		await setup();

		await expect.element(page.getByText('Six open Offers')).toBeVisible();
	});

	// #486: the Recent-activity feed sits low on the page -- not inside
	// `secondary`'s own narrow rail, which is why this gets its own prop
	// rather than becoming one more block a route appends to `secondary`.
	// #2063: last in the primary column, so it starts under that column's
	// last card and not under the whole split, where a long rail set it.
	it('renders the feed last in the primary column, outside the rail', async () => {
		await setup({
			secondary: textSnippet('Two doulas unassigned'),
			feed: textSnippet('Mark Goho sent the invoice')
		});

		const primary = page.getByText('Six open Offers');
		const feed = page.getByText('Mark Goho sent the invoice');
		await expect.element(primary).toBeVisible();
		await expect.element(page.getByText('Two doulas unassigned')).toBeVisible();
		await expect.element(feed).toBeVisible();
		await expect.element(page.getByRole('complementary').getByText('Mark Goho sent the invoice')).not.toBeInTheDocument();
		expect(feed.element().parentElement).toBe(primary.element().parentElement);
		expect(primary.element().nextElementSibling).toBe(feed.element());
	});

	it('renders the feed last in the primary column when there is no rail', async () => {
		await setup({ feed: textSnippet('Mark Goho sent the invoice') });

		expect(page.getByText('Six open Offers').element().nextElementSibling).toBe(
			page.getByText('Mark Goho sent the invoice').element()
		);
	});

	it('renders no feed region when empty, whatever was passed', async () => {
		await setup({ isEmpty: true, feed: textSnippet('Mark Goho sent the invoice') });

		await expect.element(page.getByText('Mark Goho sent the invoice')).not.toBeInTheDocument();
	});

	// #2006: the drawing's quick-link cards sit under the title, before
	// the work, at every width.
	it('renders the links region between the title and the primary region', async () => {
		const { container } = await setup({ links: textSnippet('Clients card') });

		await expect.element(page.getByText('Clients card')).toBeVisible();
		const text = container.textContent ?? '';
		expect(text.indexOf('Willow Birth Collective')).toBeLessThan(text.indexOf('Clients card'));
		expect(text.indexOf('Clients card')).toBeLessThan(text.indexOf('Six open Offers'));
	});

	it('renders no links region when empty, whatever was passed', async () => {
		await setup({ isEmpty: true, links: textSnippet('Clients card') });

		await expect.element(page.getByText('Clients card')).not.toBeInTheDocument();
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

	it('reserves the page frame for a skeleton while loading, instead of rendering the title or content (#480)', async () => {
		const { container } = await setup({ loading: 'Loading your Practice' });

		expect(container.querySelector('center-l')).toHaveAttribute('max', 'none');
		await expect.element(page.getByRole('status', { name: 'Loading your Practice' })).toBeVisible();
		await expect
			.element(page.getByRole('heading', { level: 1, name: 'Willow Birth Collective' }))
			.not.toBeInTheDocument();
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
