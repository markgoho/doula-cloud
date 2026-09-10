import type { ComponentProps } from 'svelte';
import { createRawSnippet } from 'svelte';
import { page } from 'vitest/browser';
import { describe, expect, it, vi } from 'vitest';
import { render } from 'vitest-browser-svelte';
import HistoryDisclosure from './HistoryDisclosure.svelte';

interface Entry {
	id: string;
	text: string;
}

const twoEntries: Entry[] = [
	{ id: 'event-2', text: 'Roles changed from Doula to Admin' },
	{ id: 'event-1', text: 'Joined' }
];

function entrySnippet() {
	return createRawSnippet<[Entry]>((item) => ({
		render: () => `<span></span>`,
		setup: (element) => {
			element.textContent = item().text;
		}
	}));
}

interface SetupOptions {
	items?: Entry[];
	error?: string;
	hasMore?: boolean;
	isLoadingMore?: boolean;
}

async function setup({ items, error, hasMore, isLoadingMore }: SetupOptions = {}) {
	const onOpen = vi.fn();
	const onLoadMore = vi.fn();
	// Annotated rather than inferred: a generic component's type parameter
	// is resolved from the `render` call's own argument, so lifting the
	// props into a variable to reuse them on `rerender` would otherwise
	// widen `T` to `unknown`.
	const properties: ComponentProps<typeof HistoryDisclosure<Entry>> = {
		label: 'Membership history',
		subjectName: 'Renata Alvarez',
		items,
		key: (item: Entry) => item.id,
		entry: entrySnippet(),
		error,
		emptyMessage: 'Nothing recorded.',
		hasMore,
		isLoadingMore,
		loadMoreLabel: 'Show older changes',
		idPrefix: 'table-staff-1-membership',
		onOpen,
		onLoadMore
	};
	const { rerender } = await render(HistoryDisclosure<Entry>, properties);
	return {
		onOpen,
		onLoadMore,
		update: (next: SetupOptions) => rerender({ ...properties, ...next })
	};
}

/**
 * The disclosure starts closed, which is the whole point of it -- so a
 * browser hides everything inside it until somebody opens it, and an
 * assertion about the content has to open it first.
 */
async function openDisclosure() {
	await page.getByText('Membership history for Renata Alvarez').click();
}

describe('HistoryDisclosure.svelte', () => {
	it('names whose history it is, so two rows’ disclosures are told apart', async () => {
		await setup({ items: twoEntries });

		// The summary computes its accessible name from its own content,
		// so the visually-hidden name is part of what a screen reader
		// announces for this control -- which is the whole mechanism that
		// tells one row's disclosure from the next (#667).
		await expect
			.element(page.getByText('Membership history for Renata Alvarez'))
			.toBeVisible();
	});

	it('asks for the history the first time it is opened', async () => {
		const { onOpen } = await setup();

		await page.getByText('Membership history for Renata Alvarez').click();

		expect(onOpen).toHaveBeenCalledTimes(1);
	});

	it('does not ask again when it is closed and reopened', async () => {
		// An append-only trail that was correct a second ago is still
		// correct, so the caller records what it has asked for -- but the
		// component must not ask on a *close*, or the caller would never
		// get the chance to.
		const { onOpen } = await setup({ items: twoEntries });

		const summary = page.getByText('Membership history for Renata Alvarez');
		await summary.click();
		await summary.click();

		expect(onOpen).toHaveBeenCalledTimes(1);
	});

	it('says it is loading while the first page is still in flight', async () => {
		await setup();
		await openDisclosure();

		await expect.element(page.getByText('Loading...')).toBeVisible();
	});

	it('says so when there is no history at all', async () => {
		await setup({ items: [] });
		await openDisclosure();

		await expect.element(page.getByText('Nothing recorded.')).toBeVisible();
	});

	it('renders each entry through the caller’s own snippet, in order', async () => {
		await setup({ items: twoEntries });
		await openDisclosure();

		const entries = page.getByRole('listitem').elements();
		expect(entries.map((entry) => entry.textContent)).toEqual([
			'Roles changed from Doula to Admin',
			'Joined'
		]);
	});

	it('shows the failure on its own when the first page is what failed', async () => {
		await setup({ error: 'Failed to load membership history' });
		await openDisclosure();

		await expect.element(page.getByText('Failed to load membership history')).toBeVisible();
		// Not "Loading...": that request is over, and saying otherwise
		// promises one that is no longer in flight.
		expect(page.getByText('Loading...').elements()).toHaveLength(0);
	});

	// A history pages, so the request that fails is usually the second
	// one. Answering "show me older changes" by taking away the changes
	// she can already see loses the very thing she opened this for.
	it('keeps the entries already on screen when a later page fails', async () => {
		await setup({ items: twoEntries, error: 'Failed to load membership history' });
		await openDisclosure();

		await expect.element(page.getByText('Failed to load membership history')).toBeVisible();
		expect(page.getByRole('listitem').elements()).toHaveLength(2);
	});

	it('offers the next page only when one exists, and names whose it is', async () => {
		const { onLoadMore } = await setup({ items: twoEntries, hasMore: true });
		await openDisclosure();

		const button = page.getByRole('button', { name: 'Show older changes' });
		await expect.element(button).toBeVisible();
		// The button's own words name every row's button alike, so the
		// person it belongs to is carried on the description (#667).
		await expect
			.element(button)
			.toHaveAttribute('aria-describedby', 'table-staff-1-membership-subject-name');

		await button.click();
		expect(onLoadMore).toHaveBeenCalledTimes(1);
	});

	// The button and the name it is described by both survive the update
	// that arrives when the next page is asked for -- the description is
	// what tells one row's button from the next (#667), and it has to
	// stay wired across the very interaction it exists for.
	it('shows the next page as in flight without losing the name that describes it', async () => {
		const { update } = await setup({ items: twoEntries, hasMore: true });
		await openDisclosure();

		await update({ items: twoEntries, hasMore: true, isLoadingMore: true });

		const button = page.getByRole('button', { name: 'Show older changes' });
		await expect.element(button).toHaveAttribute('aria-busy', 'true');
		await expect
			.element(button)
			.toHaveAttribute('aria-describedby', 'table-staff-1-membership-subject-name');
		await expect
			.element(page.getByText('Renata Alvarez', { exact: true }))
			.toBeInTheDocument();
	});

	it('offers no next page when the history is complete', async () => {
		await setup({ items: twoEntries });
		await openDisclosure();

		expect(page.getByRole('button', { name: 'Show older changes' }).elements()).toHaveLength(0);
	});
});
