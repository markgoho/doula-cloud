import type { ComponentProps } from 'svelte';
import { createRawSnippet } from 'svelte';
import { page } from 'vitest/browser';
import { describe, expect, it, vi } from 'vitest';
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

	it('renders the back link above the outcome and the title (#1998)', async () => {
		await setup({ backHref: '/practices/p1/clients', backLabel: 'Clients', outcome: 'Work with Ada started.' });

		const back = page.getByRole('link', { name: 'Clients' });
		await expect.element(back).toBeVisible();
		await expect.element(back).toHaveAttribute('href', '/practices/p1/clients');
		const outcome = page.getByRole('status').element();
		const title = page.getByRole('heading', { level: 1, name: 'Ada Lovelace' }).element();
		expect(back.element().compareDocumentPosition(outcome) & Node.DOCUMENT_POSITION_FOLLOWING).toBeTruthy();
		expect(back.element().compareDocumentPosition(title) & Node.DOCUMENT_POSITION_FOLLOWING).toBeTruthy();
	});

	it('draws no back link when none is given', async () => {
		const { container } = await setup();

		expect(container.querySelector('.back')).toBeNull();
	});

	it('omits the summary and the actions when neither is given', async () => {
		await setup();

		await expect.element(page.getByText('Due Mar 4')).not.toBeInTheDocument();
		await expect.element(page.getByText('Edit')).not.toBeInTheDocument();
	});

	it('renders the summary between the title and the first section', async () => {
		await setup({ summary: textSnippet('Due Mar 4') });

		const title = page.getByRole('heading', { level: 1, name: 'Ada Lovelace' }).element();
		const summary = page.getByText('Due Mar 4').element();
		const firstSection = page.getByRole('heading', { level: 2, name: 'Visits' }).element();

		expect(title.compareDocumentPosition(summary) & Node.DOCUMENT_POSITION_FOLLOWING).toBeTruthy();
		expect(
			summary.compareDocumentPosition(firstSection) & Node.DOCUMENT_POSITION_FOLLOWING
		).toBeTruthy();
	});

	// #1611: GOV.UK puts a notification banner immediately before the h1.
	// #1710: as a status, focused on arrival.
	it('renders the outcome before the title, as a status that takes focus', async () => {
		await setup({ outcome: 'Work with Ada started.' });

		const status = page.getByRole('status').filter({ hasText: 'Work with Ada started.' });
		await expect.element(status).toBeVisible();
		await expect.element(status).toHaveFocus();
		const title = page.getByRole('heading', { level: 1, name: 'Ada Lovelace' }).element();
		expect(status.element().compareDocumentPosition(title) & Node.DOCUMENT_POSITION_FOLLOWING).toBeTruthy();
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

	// #2004: the contents come after the record's own title and before its
	// sections, in reading order as on the narrow page -- GOV.UK's
	// contents list, after the title -- so a screen reader or keyboard
	// user meets "Ada Lovelace", then what is on the page, then the
	// sections. Before, they came after the last section, which is where
	// the narrow page drew them too.
	it('reads the title, then the contents, then the sections', async () => {
		const { container } = await setup({ isContentsShown: true });

		const title = page.getByRole('heading', { level: 1, name: 'Ada Lovelace' }).element();
		const firstSection = page.getByRole('heading', { level: 2, name: 'Visits' }).element();
		const strip = container.querySelector('.contents-strip')!;

		expect(title.compareDocumentPosition(strip) & Node.DOCUMENT_POSITION_FOLLOWING).toBeTruthy();
		expect(strip.compareDocumentPosition(firstSection) & Node.DOCUMENT_POSITION_FOLLOWING).toBeTruthy();
	});

	// #2257: on a phone the summary card pushed the strip below the first
	// screen, so the strip now comes before the summary.
	it('puts the jump-to strip between the title and the summary', async () => {
		const { container } = await setup({ isContentsShown: true, summary: textSnippet('Due Mar 4') });

		const title = page.getByRole('heading', { level: 1, name: 'Ada Lovelace' }).element();
		const summary = page.getByText('Due Mar 4').element();
		// Case 1 of the querySelector exception: the strip and the rail hold the same links.
		const strip = container.querySelector('.contents-strip')!;

		expect(title.compareDocumentPosition(strip) & Node.DOCUMENT_POSITION_FOLLOWING).toBeTruthy();
		expect(strip.compareDocumentPosition(summary) & Node.DOCUMENT_POSITION_FOLLOWING).toBeTruthy();
	});

	it('draws the back link, the title and a skeleton while loading, instead of the record (#480, #2248)', async () => {
		const { container } = await setup({
			loading: 'Loading the Engagement',
			backHref: '/clients',
			backLabel: 'Clients'
		});

		expect(container.querySelector('center-l')).toHaveAttribute('max', 'none');
		await expect
			.element(page.getByRole('status', { name: 'Loading the Engagement' }))
			.toBeVisible();
		await expect.element(page.getByRole('link', { name: 'Clients' })).toBeVisible();
		await expect.element(page.getByRole('heading', { level: 1, name: 'Ada Lovelace' })).toBeVisible();
		await expect.element(page.getByRole('heading', { level: 2, name: 'Invoices' })).toBeVisible();
		await expect.element(page.getByText('Three visits booked')).not.toBeInTheDocument();
		expect(page.getByRole('status').all()).toHaveLength(1);
	});

	it('draws one block of bars while loading a record that names no sections yet', async () => {
		await setup({ loading: 'Loading the request', sections: [] });

		await expect.element(page.getByRole('status', { name: 'Loading the request' })).toBeVisible();
		await expect.element(page.getByRole('heading', { level: 2 })).not.toBeInTheDocument();
	});

	it('holds a heading-shaped bar, not an empty heading, while a title that is the data loads (#2248)', async () => {
		await setup({ title: '', loading: 'Loading the Client' });

		await expect.element(page.getByRole('heading', { level: 1 })).not.toBeInTheDocument();
		expect(page.getByRole('status').all()).toHaveLength(1);
	});

	// #2257: the headings a route names while loading are its sections', so
	// the contents are drawn from them at once, and land where the loaded
	// ones do. Case 1 of the querySelector exception: two copies of one list.
	it('draws the contents from the named sections while loading, if the loaded record will have them (#480, #2257)', async () => {
		const { container } = await setup({ loading: 'Loading the Engagement', isContentsShown: true });

		for (const region of ['.contents-rail', '.contents-strip']) {
			const hrefs = [...container.querySelectorAll(`${region} a`)].map((anchor) => anchor.getAttribute('href'));
			expect(hrefs).toEqual(['#visits', '#invoices']);
		}
		expect(container.querySelector('#invoices')).not.toBeNull();
		expect(page.getByRole('status').all()).toHaveLength(1);
	});

	// #2251: a SummaryList summary has a shape the route knows before its
	// data, so a card of its rows holds the summary's place. Case 2 of the
	// querySelector exception: the card is hidden from assistive technology,
	// since the section Skeleton already says the record is loading.
	it('holds a card of the summary rows a route names while loading (#2251)', async () => {
		const { container } = await setup({
			loading: 'Loading the Staff member',
			summaryRows: [
				{ label: 'Email address', value: 'given.family@example.com' },
				{ label: 'Roles', value: 'Doula', action: 'Change' },
				{ label: 'Works from', value: ['New York', 'Last confirmed'] }
			]
		});

		const card = container.querySelector(':scope .summary[aria-hidden="true"] dl')!;
		expect([...card.querySelectorAll('dt')].map((term) => term.textContent)).toEqual([
			'Email address',
			'Roles',
			'Works from'
		]);
		expect(card.querySelectorAll('.action')).toHaveLength(1);
		expect(card.querySelectorAll('.value-placeholder')).toHaveLength(4);
		expect(page.getByRole('status').all()).toHaveLength(1);
	});

	// Case 2 of the querySelector exception, as above.
	it('holds the line of links a summary has under its card while loading (#2258)', async () => {
		const { container } = await setup({
			loading: 'Loading the Engagement',
			summaryRows: [{ label: 'Care', value: 'Birth' }],
			summaryLinks: ['View Client']
		});

		expect(container.querySelector(':scope .summary .summary-links')?.textContent).toBe('View Client');
		expect(page.getByRole('link').all()).toHaveLength(0);
	});

	it('holds no summary card while loading a record that names no summary rows (#2251)', async () => {
		const { container } = await setup({ loading: 'Loading the Client', summary: textSnippet('Due Mar 4') });

		expect(container.querySelector('.summary')).toBeNull();
	});

	// Case 2 of the querySelector exception: a box with no role, holding a line.
	it('holds the Back link line while loading a record whose Back link is its own data (#2251)', async () => {
		const { container } = await setup({ loading: 'Loading your account', isBackPending: true });

		expect(container.querySelector('.back-placeholder')).not.toBeNull();
		expect(page.getByRole('link').all()).toHaveLength(0);
	});

	it('draws the actions a route says do not wait on its record, beside a title bar, while loading (#2251)', async () => {
		await setup({
			title: '',
			loading: 'Loading the Staff member',
			loadingActions: createRawSnippet(() => ({ render: () => '<a href="/recovery">Send a recovery code</a>' }))
		});

		await expect.element(page.getByRole('link', { name: 'Send a recovery code' })).toBeVisible();
		await expect.element(page.getByRole('heading', { level: 1 })).not.toBeInTheDocument();
	});

	it('draws the actions beside a known title while loading', async () => {
		await setup({
			loading: 'Loading the Staff member',
			loadingActions: createRawSnippet(() => ({ render: () => '<a href="/recovery">Send a recovery code</a>' }))
		});

		await expect.element(page.getByRole('heading', { level: 1, name: 'Ada Lovelace' })).toBeVisible();
		await expect.element(page.getByRole('link', { name: 'Send a recovery code' })).toBeVisible();
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

	// #2508: a link from another screen to one section lands on it once
	// the record is drawn, not on the loading bars it arrived on.
	it('scrolls to the section the address names once the record has loaded', async () => {
		const scrolled = vi.spyOn(HTMLElement.prototype, 'scrollIntoView').mockImplementation(function noop() {});
		history.replaceState(history.state, '', '#invoices');
		try {
			const { rerender } = await setup({ loading: 'Loading the Engagement' });
			expect(scrolled).not.toHaveBeenCalled();

			await rerender({ loading: undefined });
			await expect.poll(() => scrolled.mock.contexts.map((element) => (element as HTMLElement).id)).toEqual(['invoices']);

			await rerender({ loading: 'Loading the Engagement' });
			await rerender({ loading: undefined });
			expect(scrolled).toHaveBeenCalledTimes(1);
		} finally {
			history.replaceState(history.state, '', location.pathname);
			scrolled.mockRestore();
		}
	});

	it('scrolls nowhere when the address names no section', async () => {
		const scrolled = vi.spyOn(HTMLElement.prototype, 'scrollIntoView').mockImplementation(function noop() {});
		try {
			await setup();
			await expect.element(page.getByRole('heading', { level: 2, name: 'Visits' })).toBeVisible();
			expect(scrolled).not.toHaveBeenCalled();
		} finally {
			scrolled.mockRestore();
		}
	});
});
