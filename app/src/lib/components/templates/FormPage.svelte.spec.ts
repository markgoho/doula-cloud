import type { ComponentProps } from 'svelte';
import { createRawSnippet } from 'svelte';
import { page } from 'vitest/browser';
import { describe, expect, it } from 'vitest';
import { render } from 'vitest-browser-svelte';
import FormPage from './FormPage.svelte';

function textSnippet(text: string) {
	return createRawSnippet(() => ({ render: () => `<p>${text}</p>` }));
}

type SetupOptions = Partial<ComponentProps<typeof FormPage>>;

/*
 * Nested under `props` rather than passed flat, unlike the other Template
 * specs: `intro` is also the name of a Svelte mount option (the transition
 * flag), so a flat `intro` is read as an option and every real prop beside
 * it is rejected as unknown. The region name is ADR-0018's and stays.
 */
async function setup(overrides: SetupOptions = {}) {
	return render(FormPage, {
		props: {
			title: 'New client',
			fieldsets: [
				{ legend: 'About the client', content: textSnippet('Name and contact') },
				{ legend: 'Birth preferences', content: textSnippet('Practice-defined fields') }
			],
			actions: textSnippet('Save client'),
			...overrides
		}
	});
}

describe('FormPage.svelte', () => {
	it('renders the title as the page h1', async () => {
		await setup();

		await expect.element(page.getByRole('heading', { level: 1, name: 'New client' })).toBeVisible();
	});

	it('renders each fieldset with its legend as a real fieldset element', async () => {
		const { container } = await setup();

		expect(container.querySelectorAll('fieldset')).toHaveLength(2);
		await expect.element(page.getByText('About the client')).toBeVisible();
		await expect.element(page.getByText('Practice-defined fields')).toBeVisible();
	});

	// #425: `invite` has two groups and no name for either, so an entry
	// with no legend must not print an empty one.
	it('renders a group with no legend as a plain stack, not an unnamed fieldset', async () => {
		const { container } = await setup({
			fieldsets: [{ content: textSnippet('Their email') }, { content: textSnippet('Roles') }]
		});

		expect(container.querySelectorAll('fieldset')).toHaveLength(0);
		expect(container.querySelectorAll('legend')).toHaveLength(0);
		await expect.element(page.getByText('Their email')).toBeVisible();
		await expect.element(page.getByText('Roles')).toBeVisible();
	});

	// The old key was the legend, so two groups sharing one -- or two
	// unnamed groups, both `undefined` -- collided into a single node.
	it('renders both of two fieldsets that share a legend', async () => {
		const { container } = await setup({
			fieldsets: [
				{ legend: 'Birth preferences', content: textSnippet('First section') },
				{ legend: 'Birth preferences', content: textSnippet('Second section') }
			]
		});

		expect(container.querySelectorAll('fieldset')).toHaveLength(2);
		await expect.element(page.getByText('First section')).toBeVisible();
		await expect.element(page.getByText('Second section')).toBeVisible();
	});

	it('renders a variable number of fieldsets, as ADR-0017 requires', async () => {
		const { container } = await setup({ fieldsets: [] });

		expect(container.querySelectorAll('fieldset')).toHaveLength(0);
		await expect.element(page.getByText('Save client')).toBeVisible();
	});

	it('always renders the actions region', async () => {
		await setup();

		await expect.element(page.getByText('Save client')).toBeVisible();
	});

	it('omits the intro and the error summary region when neither is given', async () => {
		await setup();

		await expect.element(page.getByText('Tell us about the birth')).not.toBeInTheDocument();
		await expect.element(page.getByText('There is a problem')).not.toBeInTheDocument();
	});

	/*
	 * ADR-0018's rule, asserted rather than trusted (#467): the region is a
	 * named Snippet prop and the Template renders nothing of its own into
	 * it -- no empty box, no hidden live region for a screen reader to trip
	 * over on a clean form. Four children: the h1, two fieldsets and the
	 * actions cluster.
	 */
	it('renders nothing in the error summary region when no snippet is passed', async () => {
		const { container } = await setup();

		expect(container.querySelectorAll(':scope center-l > stack-l > *')).toHaveLength(4);
	});

	it('renders the intro between the title and the first fieldset', async () => {
		await setup({ intro: textSnippet('Tell us about the birth') });

		const intro = page.getByText('Tell us about the birth').element();
		const legend = page.getByText('About the client').element();

		expect(intro.compareDocumentPosition(legend) & Node.DOCUMENT_POSITION_FOLLOWING).toBeTruthy();
	});

	/*
	 * Above the <h1>, which is GOV.UK's position and the one `QuestionPage`
	 * already takes. It sat below the title until #467: on a refused submit
	 * the problem is why the reader is back on this page at all, so she
	 * meets it before the page's own name.
	 */
	it('renders the error summary above the title, the intro and the fields', async () => {
		await setup({
			errorSummary: textSnippet('There is a problem'),
			intro: textSnippet('Tell us about the birth')
		});

		const summary = page.getByText('There is a problem').element();
		const title = page.getByRole('heading', { level: 1, name: 'New client' }).element();
		const intro = page.getByText('Tell us about the birth').element();
		const legend = page.getByText('About the client').element();

		expect(summary.compareDocumentPosition(title) & Node.DOCUMENT_POSITION_FOLLOWING).toBeTruthy();
		expect(summary.compareDocumentPosition(intro) & Node.DOCUMENT_POSITION_FOLLOWING).toBeTruthy();
		expect(summary.compareDocumentPosition(legend) & Node.DOCUMENT_POSITION_FOLLOWING).toBeTruthy();
	});

	it('caps the form at the form measure, not the full page width, and renders no chrome', async () => {
		const { container } = await setup();

		expect(container.querySelector('center-l')).toHaveAttribute('max', 'var(--form-max)');
		expect(container.querySelector('center-l')).toHaveAttribute('gutters', 'var(--page-gutter)');
		expect(container.querySelector('form')).toBeNull();
		expect(container.querySelector('nav')).toBeNull();
	});
});
