import type { ComponentProps } from 'svelte';
import { createRawSnippet } from 'svelte';
import { page } from 'vitest/browser';
import { describe, expect, it } from 'vitest';
import { render } from 'vitest-browser-svelte';
import OverviewHub from './OverviewHub.svelte';

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

	it('owns the page frame and renders no chrome', async () => {
		const { container } = await setup();

		expect(container.querySelector('center-l')).toHaveAttribute('max', 'var(--page-max)');
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
});
