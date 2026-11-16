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
		await setup();

		await expect.element(page.getByText('Jump to')).not.toBeInTheDocument();
	});

	// One list at every width (ADR-0018, 2026-11-07): no rail beside the
	// record, so the title starts at the gutter every Staff page starts at.
	it('lists every section once under the title, and links only to itself', async () => {
		const { container } = await setup({ isContentsShown: true });

		await expect.element(page.getByText('Jump to')).toBeVisible();
		expect(page.getByRole('link', { name: 'Visits' }).all()).toHaveLength(1);
		expect(page.getByRole('link', { name: 'Visits' }).element()).toHaveAttribute('href', '#visits');
		expect(page.getByRole('link', { name: 'Invoices' }).element()).toHaveAttribute('href', '#invoices');

		// It is not a nav: the region derives from `sections`, so there is no
		// way for a route to put a route in it. ADR-0018.
		expect(container.querySelector('nav')).toBeNull();
	});

	// #2004: the contents come after the record's own title and before its
	// sections, in reading order as on the narrow page -- GOV.UK's
	// contents list, after the title -- so a screen reader or keyboard
	// user meets "Ada Lovelace", then what is on the page, then the
	// sections. Before, they came after the last section, which is where
	// the narrow page drew them too.
	it('reads the title, then the contents, then the sections', async () => {
		await setup({ isContentsShown: true });

		const title = page.getByRole('heading', { level: 1, name: 'Ada Lovelace' }).element();
		const firstSection = page.getByRole('heading', { level: 2, name: 'Visits' }).element();
		const strip = page.getByText('Jump to').element();

		expect(title.compareDocumentPosition(strip) & Node.DOCUMENT_POSITION_FOLLOWING).toBeTruthy();
		expect(strip.compareDocumentPosition(firstSection) & Node.DOCUMENT_POSITION_FOLLOWING).toBeTruthy();
	});

	// #2257: on a phone the summary card pushed the strip below the first
	// screen, so the strip now comes before the summary.
	it('puts the jump-to strip between the title and the summary', async () => {
		await setup({ isContentsShown: true, summary: textSnippet('Due Mar 4') });

		const title = page.getByRole('heading', { level: 1, name: 'Ada Lovelace' }).element();
		const summary = page.getByText('Due Mar 4').element();
		const strip = page.getByText('Jump to').element();

		expect(title.compareDocumentPosition(strip) & Node.DOCUMENT_POSITION_FOLLOWING).toBeTruthy();
		expect(strip.compareDocumentPosition(summary) & Node.DOCUMENT_POSITION_FOLLOWING).toBeTruthy();
	});

	// #2755: "Jump to" is the label and one list of plain links, and the list
	// is where "Back to top" lands.
	it('draws "Jump to" as a named list of links that takes focus, with no box of chips', async () => {
		await setup({ isContentsShown: true });

		const list = page.getByRole('list', { name: 'Jump to' });
		await expect.element(list).toHaveAttribute('id', 'jump-to');
		await expect.element(list).toHaveAttribute('tabindex', '-1');
		expect(list.getByRole('link').all()).toHaveLength(2);
	});

	describe('with a facts region (#2755)', () => {
		it('reads the title, the status line, "Jump to", the facts, then the sections', async () => {
			await setup({
				isContentsShown: true,
				statusLine: textSnippet('Care phase Active'),
				facts: textSnippet('Contract Signed')
			});

			const title = page.getByRole('heading', { level: 1, name: 'Ada Lovelace' }).element();
			const status = page.getByText('Care phase Active').element();
			const jump = page.getByText('Jump to').element();
			const facts = page.getByText('Contract Signed').element();
			const section = page.getByRole('heading', { level: 2, name: 'Visits' }).element();
			const inOrder = [title, status, jump, facts, section];
			for (const [index, element] of inOrder.entries()) {
				const next = inOrder[index + 1];
				if (next) expect(element.compareDocumentPosition(next) & Node.DOCUMENT_POSITION_FOLLOWING).toBeTruthy();
			}
		});

		describe('the details region', () => {
			const details = createRawSnippet<[string, boolean]>((view, isShown) => ({
				render: () => `<p data-view="${view()}" data-shown="${isShown()}">Phone (585) 555-0100</p>`
			}));

			it('is drawn twice, the aside after the facts and the last block after the sections, and one of them is displayed', async () => {
				// A wide window, so the record body is two columns.
				await page.viewport(1440, 900);
				const { container } = await setup({ isContentsShown: true, facts: textSnippet('Contract Signed'), details });

				const copies = [...container.querySelectorAll('[data-view]')];
				expect(copies.map((copy) => (copy as HTMLElement).dataset.view)).toEqual(['aside', 'section']);
				const [aside, last] = copies as [HTMLElement, HTMLElement];
				const facts = page.getByText('Contract Signed').element();
				const visits = page.getByRole('heading', { level: 2, name: 'Visits' }).element();
				expect(facts.compareDocumentPosition(aside) & Node.DOCUMENT_POSITION_FOLLOWING).toBeTruthy();
				expect(aside.compareDocumentPosition(visits) & Node.DOCUMENT_POSITION_FOLLOWING).toBeTruthy();
				expect(visits.compareDocumentPosition(last) & Node.DOCUMENT_POSITION_FOLLOWING).toBeTruthy();
				// Which copy is displayed is the container query's; the snippet is told the same.
				const isAsideDisplayed = getComputedStyle(aside.closest('aside')!).display !== 'none';
				await expect.poll(() => aside.dataset.shown).toBe(String(isAsideDisplayed));
				expect(last.dataset.shown).toBe(String(!isAsideDisplayed));
				// Case 3 (a fact about the document): exactly one `details` id, on the displayed copy.
				expect(container.querySelectorAll('[id="details"]')).toHaveLength(1);
				expect(container.querySelector('#details')?.contains(isAsideDisplayed ? aside : last)).toBe(true);
				await expect.element(page.getByRole('link', { name: 'Details' })).toHaveAttribute('href', '#details');
			});

			it('shows the last block, and not the aside, where the record is one column', async () => {
				await page.viewport(600, 900);
				const { container } = await setup({ isContentsShown: true, facts: textSnippet('Contract Signed'), details });

				const last = container.querySelector<HTMLElement>('[data-view="section"]')!;
				await expect.poll(() => last.dataset.shown).toBe('true');
				expect(container.querySelector('#details')?.contains(last)).toBe(true);
			});
		});

		it('puts the summary after the sections, so the first screen holds the facts', async () => {
			await setup({
				isContentsShown: true,
				summary: textSnippet('Due Mar 4'),
				facts: textSnippet('Contract Signed')
			});

			const summary = page.getByText('Due Mar 4').element();
			const lastSection = page.getByRole('heading', { level: 2, name: 'Invoices' }).element();
			expect(lastSection.compareDocumentPosition(summary) & Node.DOCUMENT_POSITION_FOLLOWING).toBeTruthy();
		});

		it('holds the status line, the facts and the sections while loading', async () => {
			const { container } = await setup({
				isContentsShown: true,
				isStatusLinePending: true,
				factsRows: [{ label: 'Contract', value: 'Signed' }],
				read: { status: 'loading', label: 'Loading the record' }
			});

			// Bars with no role and no words: nothing for an accessible query to find (case 2).
			expect(container.querySelector('.status-line')).not.toBeNull();
			expect(container.querySelector('.facts')).not.toBeNull();
			await expect.element(page.getByRole('heading', { level: 2, name: 'Visits' })).toBeVisible();
		});

		it('draws no facts column for a record that names their shape but gives none once loaded', async () => {
			const { container } = await setup({ factsRows: [{ label: 'Contract', value: 'Signed' }] });

			expect(container.querySelector('.record-body')).not.toBeNull();
			expect(container.querySelector('.facts')).toBeNull();
		});

		it('draws no facts bars while loading when the route names no shape for them', async () => {
			const { container } = await setup({ facts: textSnippet('Contract Signed'), read: { status: 'loading', label: 'Loading' } });

			expect(container.querySelector('.record-body')).not.toBeNull();
			expect(container.querySelector('.facts')).toBeNull();
		});

		it('draws no facts column while loading a record that names none', async () => {
			const { container } = await setup({ isContentsShown: true, read: { status: 'loading', label: 'Loading' } });

			expect(container.querySelector('.facts')).toBeNull();
			expect(container.querySelector('.record-body')).toBeNull();
		});
	});

	it('draws the back link, the title and a skeleton while loading, instead of the record (#480, #2248)', async () => {
		const { container } = await setup({
			read: { status: 'loading', label: 'Loading the work' },
			backHref: '/clients',
			backLabel: 'Clients'
		});

		expect(container.querySelector('center-l')).toHaveAttribute('max', 'none');
		await expect
			.element(page.getByRole('status', { name: 'Loading the work' }))
			.toBeVisible();
		await expect.element(page.getByRole('link', { name: 'Clients' })).toBeVisible();
		await expect.element(page.getByRole('heading', { level: 1, name: 'Ada Lovelace' })).toBeVisible();
		await expect.element(page.getByRole('heading', { level: 2, name: 'Invoices' })).toBeVisible();
		await expect.element(page.getByText('Three visits booked')).not.toBeInTheDocument();
		expect(page.getByRole('status').all()).toHaveLength(1);
	});

	it('draws one block of bars while loading a record that names no sections yet', async () => {
		await setup({ read: { status: 'loading', label: 'Loading the request' }, sections: [] });

		await expect.element(page.getByRole('status', { name: 'Loading the request' })).toBeVisible();
		await expect.element(page.getByRole('heading', { level: 2 })).not.toBeInTheDocument();
	});

	it('holds a heading-shaped bar, not an empty heading, while a title that is the data loads (#2248)', async () => {
		await setup({ title: '', read: { status: 'loading', label: 'Loading the Client' } });

		await expect.element(page.getByRole('heading', { level: 1 })).not.toBeInTheDocument();
		expect(page.getByRole('status').all()).toHaveLength(1);
	});

	// #2257: the headings a route names while loading are its sections', so
	// the contents are drawn from them at once, and land where the loaded
	// ones do.
	it('draws the contents from the named sections while loading, if the loaded record will have them (#480, #2257)', async () => {
		const { container } = await setup({ read: { status: 'loading', label: 'Loading the work' }, isContentsShown: true });

		expect(page.getByRole('link', { name: 'Visits' }).element()).toHaveAttribute('href', '#visits');
		expect(page.getByRole('link', { name: 'Invoices' }).element()).toHaveAttribute('href', '#invoices');
		expect(container.querySelector('#invoices')).not.toBeNull();
		expect(page.getByRole('status').all()).toHaveLength(1);
	});

	// #2251: a SummaryList summary has a shape the route knows before its
	// data, so a card of its rows holds the summary's place. Case 2 of the
	// querySelector exception: the card is hidden from assistive technology,
	// since the section Skeleton already says the record is loading.
	it('holds a card of the summary rows a route names while loading (#2251)', async () => {
		const { container } = await setup({
			read: { status: 'loading', label: 'Loading the Staff member' },
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
			read: { status: 'loading', label: 'Loading the work' },
			summaryRows: [{ label: 'Care', value: 'Birth' }],
			summaryLinks: ['View Client']
		});

		expect(container.querySelector(':scope .summary .summary-links')?.textContent).toBe('View Client');
		expect(page.getByRole('link').all()).toHaveLength(0);
	});

	it('holds no summary card while loading a record that names no summary rows (#2251)', async () => {
		const { container } = await setup({ read: { status: 'loading', label: 'Loading the Client' }, summary: textSnippet('Due Mar 4') });

		expect(container.querySelector('.summary')).toBeNull();
	});

	// Case 2 of the querySelector exception: a box with no role, holding a line.
	it('holds the Back link line while loading a record whose Back link is its own data (#2251)', async () => {
		const { container } = await setup({ read: { status: 'loading', label: 'Loading your account' }, isBackPending: true });

		expect(container.querySelector('.back-placeholder')).not.toBeNull();
		expect(page.getByRole('link').all()).toHaveLength(0);
	});

	it('draws the actions a route says do not wait on its record, beside a title bar, while loading (#2251)', async () => {
		await setup({
			title: '',
			read: { status: 'loading', label: 'Loading the Staff member' },
			loadingActions: createRawSnippet(() => ({ render: () => '<a href="/recovery">Send a recovery code</a>' }))
		});

		await expect.element(page.getByRole('link', { name: 'Send a recovery code' })).toBeVisible();
		await expect.element(page.getByRole('heading', { level: 1 })).not.toBeInTheDocument();
	});

	it('draws the actions beside a known title while loading', async () => {
		await setup({
			read: { status: 'loading', label: 'Loading the Staff member' },
			loadingActions: createRawSnippet(() => ({ render: () => '<a href="/recovery">Send a recovery code</a>' }))
		});

		await expect.element(page.getByRole('heading', { level: 1, name: 'Ada Lovelace' })).toBeVisible();
		await expect.element(page.getByRole('link', { name: 'Send a recovery code' })).toBeVisible();
	});

	it('draws no contents while loading a record that has none', async () => {
		await setup({ read: { status: 'loading', label: 'Loading the work' } });

		await expect.element(page.getByText('Jump to')).not.toBeInTheDocument();
	});

	it('reserves the page frame for a Notice on a load failure, instead of rendering the record (#480)', async () => {
		const { container } = await setup({ read: { status: 'failed', message: 'Failed to load the Engagement' } });

		expect(container.querySelector('center-l')).toHaveAttribute('max', 'none');
		await expect.element(page.getByText('Failed to load the Engagement')).toBeVisible();
		await expect.element(page.getByRole('heading', { level: 1, name: 'Ada Lovelace' })).not.toBeInTheDocument();
	});


	// #2508: a link from another screen to one section lands on it once
	// the record is drawn, not on the loading bars it arrived on.
	it('scrolls to the section the address names once the record has loaded', async () => {
		const scrolled = vi.spyOn(HTMLElement.prototype, 'scrollIntoView').mockImplementation(function noop() {});
		history.replaceState(history.state, '', '#invoices');
		try {
			const { rerender } = await setup({ read: { status: 'loading', label: 'Loading the work' } });
			expect(scrolled).not.toHaveBeenCalled();

			await rerender({ read: undefined });
			await expect.poll(() => scrolled.mock.contexts.map((element) => (element as HTMLElement).id)).toEqual(['invoices']);

			await rerender({ read: { status: 'loading', label: 'Loading the work' } });
			await rerender({ read: undefined });
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
