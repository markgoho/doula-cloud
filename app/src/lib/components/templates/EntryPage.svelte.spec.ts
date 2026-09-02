import type { ComponentProps } from 'svelte';
import { createRawSnippet } from 'svelte';
import { page } from 'vitest/browser';
import { describe, expect, it } from 'vitest';
import { render } from 'vitest-browser-svelte';
import EntryPage from './EntryPage.svelte';

function textSnippet(text: string) {
	return createRawSnippet(() => ({ render: () => `<p>${text}</p>` }));
}

type SetupOptions = Partial<ComponentProps<typeof EntryPage>>;

async function setup(overrides: SetupOptions = {}) {
	return render(EntryPage, {
		props: {
			title: 'Log in',
			content: textSnippet('The credentials form'),
			...overrides
		}
	});
}

describe('EntryPage.svelte', () => {
	it('renders the title as the page h1', async () => {
		await setup();

		await expect.element(page.getByRole('heading', { level: 1, name: 'Log in' })).toBeVisible();
	});

	it('always renders the content region', async () => {
		await setup();

		await expect.element(page.getByText('The credentials form')).toBeVisible();
	});

	// ADR-0018's rule, asserted rather than trusted: the region is a named
	// Snippet prop and the Template renders nothing of its own into it -- no
	// empty box, no hidden live region for a screen reader to trip over on a
	// clean form.
	it('renders nothing in the error summary region when no snippet is passed', async () => {
		await setup();

		await expect.element(page.getByText('There is a problem')).not.toBeInTheDocument();
	});

	// Above the <h1>, GOV.UK's position and the one every other Template in
	// this layer already takes (#467).
	it('renders the error summary above the title and the content', async () => {
		await setup({
			errorSummary: textSnippet('There is a problem'),
			content: textSnippet('The credentials form')
		});

		const summary = page.getByText('There is a problem').element();
		const title = page.getByRole('heading', { level: 1, name: 'Log in' }).element();
		const content = page.getByText('The credentials form').element();

		expect(summary.compareDocumentPosition(title) & Node.DOCUMENT_POSITION_FOLLOWING).toBeTruthy();
		expect(summary.compareDocumentPosition(content) & Node.DOCUMENT_POSITION_FOLLOWING).toBeTruthy();
	});

	// Top-aligned in a --form-max column, not centred in the viewport (see
	// the Template's own comment for the reasoning) -- the same token and
	// gutter FormPage already spends on a form.
	it('caps the page at the form measure, not the full page width, and renders no chrome', async () => {
		const { container } = await setup();

		expect(container.querySelector('center-l')).toHaveAttribute('max', 'var(--form-max)');
		expect(container.querySelector('center-l')).toHaveAttribute('gutters', 'var(--page-gutter)');
		expect(container.querySelector('nav')).toBeNull();
	});
});
