import type { ComponentProps } from 'svelte';
import { createRawSnippet } from 'svelte';
import { page } from 'vitest/browser';
import { describe, expect, it } from 'vitest';
import { render } from 'vitest-browser-svelte';
import RecordDetail from './RecordDetail.svelte';
// The loading Skeleton reserves space with `var(--text-body-size)`, which
// only exists once the tokens are loaded -- the real app loads them in the
// root layout. See practice-landing.svelte.spec.ts's identical import.
import '#lib/styles/app.css';

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

	it('labels each section as a landmark region named for its heading (#507)', async () => {
		await setup();

		await expect.element(page.getByRole('region', { name: 'Visits' })).toBeVisible();
		await expect.element(page.getByRole('region', { name: 'Invoices' })).toBeVisible();
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

		expect(container.querySelector('center-l')).toHaveAttribute('max', 'none');
		expect(container.querySelector('center-l')).toHaveAttribute('gutters', 'var(--page-gutter)');
		expect(container.querySelector('nav')).toBeNull();
		expect(container.querySelector('header')).toBeNull();
	});

	it('gives every section an anchor id derived from its heading', async () => {
		const { container } = await setup({
			sections: [{ heading: 'Birth Plan', content: textSnippet('Signed') }]
		});

		expect(container.querySelector('section')).toHaveAttribute('id', 'birth-plan');
	});

	it('renders no contents region unless it is asked for', async () => {
		const { container } = await setup();

		expect(container.querySelector('.contents-rail')).toBeNull();
		expect(container.querySelector('.contents-strip')).toBeNull();
	});

	it('lists every section in the contents region, and links only to itself', async () => {
		const { container } = await setup({ isContentsShown: true });

		const rail = container.querySelector('.contents-rail')!;
		const hrefs = [...rail.querySelectorAll('a')].map((anchor) => anchor.getAttribute('href'));
		expect(hrefs).toEqual(['#visits', '#invoices']);

		// It is not a nav: the region derives from `sections`, so there is no
		// way for a route to put a route in it. ADR-0018.
		expect(container.querySelector('nav')).toBeNull();
	});

	it('offers the same contents as a jump-to strip for a narrow viewport', async () => {
		const { container } = await setup({ isContentsShown: true });

		const strip = container.querySelector('.contents-strip')!;
		const hrefs = [...strip.querySelectorAll('a')].map((anchor) => anchor.getAttribute('href'));
		expect(hrefs).toEqual(['#visits', '#invoices']);
	});

	// DOM order deliberately differs from visual order here (#564): the
	// rail sits visually on the left (`sidebar-l`'s own `flex-direction:
	// row-reverse`, side="end" in the markup), but the record's own title
	// and sections stay first in READING order, so a screen reader or
	// keyboard user meets "Ada Lovelace" and her sections before "Jump
	// to" -- the opposite of source order matching visual order, and
	// deliberately so.
	it('reads the title and every section before the contents nav', async () => {
		const { container } = await setup({ isContentsShown: true });

		const title = page.getByRole('heading', { level: 1, name: 'Ada Lovelace' }).element();
		const lastSection = page.getByRole('heading', { level: 2, name: 'Invoices' }).element();
		const strip = container.querySelector('.contents-strip')!;

		expect(title.compareDocumentPosition(lastSection) & Node.DOCUMENT_POSITION_FOLLOWING).toBeTruthy();
		expect(lastSection.compareDocumentPosition(strip) & Node.DOCUMENT_POSITION_FOLLOWING).toBeTruthy();
	});

	it('reserves the page frame for a skeleton while loading, instead of rendering the record (#480)', async () => {
		const { container } = await setup({ loading: 'Loading the Engagement' });

		expect(container.querySelector('center-l')).toHaveAttribute('max', 'none');
		await expect
			.element(page.getByRole('status', { name: 'Loading the Engagement' }))
			.toBeVisible();
		await expect.element(page.getByRole('heading', { level: 1, name: 'Ada Lovelace' })).not.toBeInTheDocument();
	});

	it('reserves the contents rail column while loading, if the loaded record will have one (#480)', async () => {
		const { container } = await setup({ loading: 'Loading the Engagement', isContentsShown: true });

		expect(container.querySelector('.contents-rail')).not.toBeNull();
		// No links yet: `sections` does not exist during loading, and a
		// second "loading" announcement here would double the Skeleton's own.
		expect(container.querySelector(':scope .contents-rail a')).toBeNull();
	});

	it('does not reserve a contents rail while loading a record with no rail', async () => {
		const { container } = await setup({ loading: 'Loading the Engagement' });

		expect(container.querySelector('.contents-rail')).toBeNull();
	});

	it('reserves the page frame for a Notice on a load failure, instead of rendering the record (#480)', async () => {
		const { container } = await setup({ loadError: 'Failed to load the Engagement' });

		expect(container.querySelector('center-l')).toHaveAttribute('max', 'none');
		await expect.element(page.getByText('Failed to load the Engagement')).toBeVisible();
		await expect.element(page.getByRole('heading', { level: 1, name: 'Ada Lovelace' })).not.toBeInTheDocument();
	});

	it('prefers loadError over loading when both are somehow given', async () => {
		await setup({ loadError: 'Failed to load the Engagement', loading: 'Loading the Engagement' });

		await expect.element(page.getByText('Failed to load the Engagement')).toBeVisible();
		await expect.element(page.getByRole('status', { name: 'Loading the Engagement' })).not.toBeInTheDocument();
	});
});
