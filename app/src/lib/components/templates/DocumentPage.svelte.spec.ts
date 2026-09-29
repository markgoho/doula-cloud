import type { ComponentProps } from 'svelte';
import { createRawSnippet } from 'svelte';
import { page } from 'vitest/browser';
import { describe, expect, it } from 'vitest';
import { render } from 'vitest-browser-svelte';
import DocumentPage from './DocumentPage.svelte';
// The loading Skeleton reserves space with `var(--text-body-size)`, which
// only exists once the tokens are loaded -- the real app loads them in the
// root layout. See ListPage.svelte.spec.ts's identical import.
import '#lib/styles/app.css';

function textSnippet(text: string) {
	return createRawSnippet(() => ({ render: () => `<p>${text}</p>` }));
}

type SetupOptions = Partial<ComponentProps<typeof DocumentPage>>;

async function setup(overrides: SetupOptions = {}) {
	return render(DocumentPage, {
		props: {
			title: 'Contract',
			backHref: '/portal/engagements/engagement-1',
			content: textSnippet('The agreed scope of service'),
			...overrides
		}
	});
}

// "Exactly one h1" is a fact about the whole rendered tree, not about any
// one heading -- the named `querySelector` exception for a document fact
// (.claude/rules/svelte-tests.md, case 3).
function h1Count(container: HTMLElement) {
	return container.querySelectorAll('h1').length;
}

describe('DocumentPage.svelte', () => {
	it('renders the title as the one h1, and the content under it', async () => {
		const { container } = await setup();

		await expect.element(page.getByRole('heading', { level: 1, name: 'Contract' })).toBeVisible();
		await expect.element(page.getByText('The agreed scope of service')).toBeVisible();
		expect(h1Count(container)).toBe(1);
	});

	it('names the screen in the tab title, with the Practice as the service', async () => {
		await setup({ serviceName: 'Riverside Doulas' });

		await expect.poll(() => document.title).toBe('Contract — Riverside Doulas');
	});

	it('puts the back link above the h1, labeled "Back" unless told otherwise', async () => {
		await setup();

		const back = page.getByRole('link', { name: 'Back' });
		await expect.element(back).toHaveAttribute('href', '/portal/engagements/engagement-1');
		const heading = page.getByRole('heading', { level: 1 }).element();
		expect(back.element().compareDocumentPosition(heading) & Node.DOCUMENT_POSITION_FOLLOWING).toBeTruthy();
	});

	it('takes a back label that names where the link goes', async () => {
		await setup({ backLabel: 'Back to Invoices' });

		await expect.element(page.getByRole('link', { name: 'Back to Invoices' })).toBeVisible();
	});

	it('caps the page at the measure, inside the page gutters, and renders no chrome', async () => {
		const { container } = await setup();

		expect(container.querySelector('center-l')).toHaveAttribute('max', 'var(--measure)');
		expect(container.querySelector('center-l')).toHaveAttribute('gutters', 'var(--page-gutter)');
		expect(container.querySelector('nav')).toBeNull();
		expect(container.querySelector('header')).toBeNull();
	});

	it('renders the title, the back link and a skeleton while loading, instead of the content', async () => {
		const { container } = await setup({ loading: 'Loading Contract' });

		expect(container.querySelector('center-l')).toHaveAttribute('max', 'var(--measure)');
		await expect.element(page.getByRole('link', { name: 'Back' })).toBeVisible();
		await expect.element(page.getByRole('heading', { level: 1, name: 'Contract' })).toBeVisible();
		await expect.element(page.getByRole('status', { name: 'Loading Contract' })).toBeVisible();
		await expect.element(page.getByText('The agreed scope of service')).not.toBeInTheDocument();
		expect(h1Count(container)).toBe(1);
	});

	it('renders the title, the back link and a Notice on a load failure, instead of the content', async () => {
		const { container } = await setup({ loadError: 'Failed to load Contract' });

		expect(container.querySelector('center-l')).toHaveAttribute('max', 'var(--measure)');
		await expect.element(page.getByRole('link', { name: 'Back' })).toBeVisible();
		await expect.element(page.getByRole('heading', { level: 1, name: 'Contract' })).toBeVisible();
		await expect.element(page.getByText('Failed to load Contract')).toBeVisible();
		await expect.element(page.getByText('The agreed scope of service')).not.toBeInTheDocument();
		// `data-load-error` has no accessible signal by design -- it is the
		// fact accessibility.e2e.ts reads to refuse scanning this branch.
		expect(container.querySelector('[data-load-error]')).not.toBeNull();
		expect(h1Count(container)).toBe(1);
	});

	it('renders the title, the back link and the "none yet" sentence when the document does not exist yet', async () => {
		const { container } = await setup({ empty: 'No Contract has been sent for your care yet.' });

		expect(container.querySelector('center-l')).toHaveAttribute('max', 'var(--measure)');
		await expect.element(page.getByRole('link', { name: 'Back' })).toBeVisible();
		await expect.element(page.getByRole('heading', { level: 1, name: 'Contract' })).toBeVisible();
		await expect.element(page.getByText('No Contract has been sent for your care yet.')).toBeVisible();
		await expect.element(page.getByText('The agreed scope of service')).not.toBeInTheDocument();
		expect(container.querySelector('[data-load-error]')).toBeNull();
		expect(h1Count(container)).toBe(1);
	});

	it('prefers loadError over loading, and loading over empty, when more than one is given', async () => {
		await setup({ loadError: 'Failed to load Contract', loading: 'Loading Contract', empty: 'None yet.' });

		await expect.element(page.getByText('Failed to load Contract')).toBeVisible();
		await expect.element(page.getByRole('status', { name: 'Loading Contract' })).not.toBeInTheDocument();
		await expect.element(page.getByText('None yet.')).not.toBeInTheDocument();
	});

	it('prefers loading over empty', async () => {
		await setup({ loading: 'Loading Contract', empty: 'None yet.' });

		await expect.element(page.getByRole('status', { name: 'Loading Contract' })).toBeVisible();
		await expect.element(page.getByText('None yet.')).not.toBeInTheDocument();
	});
});
