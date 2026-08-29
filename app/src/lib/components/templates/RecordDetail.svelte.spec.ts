import type { ComponentProps } from 'svelte';
import { createRawSnippet } from 'svelte';
import { page } from 'vitest/browser';
import { describe, expect, it } from 'vitest';
import { render } from 'vitest-browser-svelte';
import RecordDetail from './RecordDetail.svelte';

function textSnippet(text: string) {
	return createRawSnippet(() => ({ render: () => `<p>${text}</p>` }));
}

type SetupOptions = Partial<ComponentProps<typeof RecordDetail>>;

async function setup(overrides: SetupOptions = {}) {
	return render(RecordDetail, {
		title: 'Ada Lovelace',
		sections: [
			{ heading: 'Visits', content: textSnippet('Three visits booked') },
			{ heading: 'Invoices', content: textSnippet('One invoice outstanding') }
		],
		...overrides
	});
}

describe('RecordDetail.svelte', () => {
	it('renders the title as the page h1', async () => {
		await setup();

		await expect.element(page.getByRole('heading', { level: 1, name: 'Ada Lovelace' })).toBeVisible();
	});

	it('renders every section as an h2 with its content, in order', async () => {
		await setup();

		await expect.element(page.getByRole('heading', { level: 2, name: 'Visits' })).toBeVisible();
		await expect.element(page.getByRole('heading', { level: 2, name: 'Invoices' })).toBeVisible();

		const visits = page.getByRole('heading', { level: 2, name: 'Visits' }).element();
		const invoices = page.getByRole('heading', { level: 2, name: 'Invoices' }).element();
		expect(visits.compareDocumentPosition(invoices) & Node.DOCUMENT_POSITION_FOLLOWING).toBeTruthy();

		await expect.element(page.getByText('Three visits booked')).toBeVisible();
	});

	it('renders a variable number of sections, including none', async () => {
		const { container } = await setup({ sections: [] });

		expect(container.querySelectorAll('section')).toHaveLength(0);
		await expect.element(page.getByRole('heading', { level: 1, name: 'Ada Lovelace' })).toBeVisible();
	});

	it('omits the summary and the actions when neither is given', async () => {
		await setup();

		await expect.element(page.getByText('Due 4 March')).not.toBeInTheDocument();
		await expect.element(page.getByText('Edit')).not.toBeInTheDocument();
	});

	it('renders the summary between the title and the first section', async () => {
		await setup({ summary: textSnippet('Due 4 March') });

		const title = page.getByRole('heading', { level: 1, name: 'Ada Lovelace' }).element();
		const summary = page.getByText('Due 4 March').element();
		const firstSection = page.getByRole('heading', { level: 2, name: 'Visits' }).element();

		expect(title.compareDocumentPosition(summary) & Node.DOCUMENT_POSITION_FOLLOWING).toBeTruthy();
		expect(
			summary.compareDocumentPosition(firstSection) & Node.DOCUMENT_POSITION_FOLLOWING
		).toBeTruthy();
	});

	it('renders the actions alongside the title', async () => {
		await setup({ actions: textSnippet('Edit') });

		await expect.element(page.getByText('Edit')).toBeVisible();
	});

	it('owns the page frame and renders no chrome', async () => {
		const { container } = await setup();

		expect(container.querySelector('center-l')).toHaveAttribute('max', 'var(--page-max)');
		expect(container.querySelector('center-l')).toHaveAttribute('gutters', 'var(--page-gutter)');
		expect(container.querySelector('nav')).toBeNull();
		expect(container.querySelector('header')).toBeNull();
	});
});
