import { createRawSnippet } from 'svelte';
import { page } from 'vitest/browser';
import { describe, expect, it, vi } from 'vitest';
import { render } from 'vitest-browser-svelte';
import DataTable from './DataTable.svelte';
// The record-view switch depends on the frame (a <stack-l>) actually
// being display:block -- that default lives in primitives.css, same as
// RecordDetail.svelte.spec.ts's own <container-l> dependency.
import '#lib/styles/app.css';

interface Row {
	name: string;
	status: string;
}

function removeActionSnippet(onRemove: (row: Row) => void) {
	return createRawSnippet<[Row]>((row) => {
		const handler = () => onRemove(row());
		return {
			render: () => `<button type="button">Remove</button>`,
			setup: (element) => {
				const button = element as HTMLButtonElement;
				button.addEventListener('click', handler);
				return () => button.removeEventListener('click', handler);
			}
		};
	});
}

const columns = [
	{ label: 'Name', accessor: (row: Row) => row.name },
	{ label: 'Status', accessor: (row: Row) => row.status }
];

const rows: Row[] = [
	{ name: 'Ada Lovelace', status: 'Active' },
	{ name: 'Grace Hopper', status: 'Inactive' }
];

const numericColumns = [
	{ label: 'Name', accessor: (row: Row) => row.name },
	{ label: 'Quantity', accessor: (row: Row) => row.status, numeric: true }
];

interface SetupOptions {
	columns?: typeof columns;
	rows?: Row[];
	rowHref?: (row: Row) => string;
	rowActions?: { label: string; onRemove: (row: Row) => void };
	hasMore?: boolean;
	onLoadMore?: () => void;
	isLoadingMore?: boolean;
	loadMoreError?: string;
	emptyMessage?: string;
	disclosure?: string;
}

/*
 * The frame is a container at 46rem (DataTable.svelte, ADR-0024), so the
 * viewport is pinned wide here rather than left to the runner's own
 * default -- a default narrower than that would make every assertion
 * about the <table> view fail for a reason that has nothing to do with
 * what the test actually checks. The record-view describe block below
 * sets it back down on purpose.
 */
const WIDE = [1440, 900] as const;
const NARROW = [390, 844] as const;

async function setup({
	columns: columnsOption = columns,
	rows: rowsOption = rows,
	rowHref,
	rowActions,
	hasMore = false,
	onLoadMore,
	isLoadingMore = false,
	loadMoreError,
	emptyMessage = 'No records yet.',
	disclosure
}: SetupOptions = {}) {
	await page.viewport(...WIDE);
	const { container } = await render(DataTable<Row>, {
		columns: columnsOption,
		rows: rowsOption,
		rowHref,
		rowActions: rowActions && {
			label: rowActions.label,
			content: removeActionSnippet(rowActions.onRemove)
		},
		hasMore,
		onLoadMore,
		isLoadingMore,
		loadMoreError,
		emptyMessage,
		disclosure
	});
	return { container };
}

describe('DataTable.svelte', () => {
	it('renders a scoped column header for each column', async () => {
		await setup();

		await expect.element(page.getByRole('columnheader', { name: 'Name' })).toBeVisible();
		await expect.element(page.getByRole('columnheader', { name: 'Status' })).toBeVisible();
	});

	it('renders each row via the column accessors', async () => {
		await setup();

		await expect.element(page.getByRole('cell', { name: 'Ada Lovelace' })).toBeVisible();
		await expect.element(page.getByRole('cell', { name: 'Active', exact: true })).toBeVisible();
		await expect.element(page.getByRole('cell', { name: 'Grace Hopper' })).toBeVisible();
		await expect.element(page.getByRole('cell', { name: 'Inactive' })).toBeVisible();
	});

	it('renders the empty message spanning all columns when rows is empty', async () => {
		const { container } = await setup({ rows: [] });

		// getByRole, not getByText: the record view carries the same
		// message in a hidden <p>, and only a role query excludes it.
		await expect.element(page.getByRole('cell', { name: 'No records yet.' })).toBeVisible();
		expect(container.querySelector('td[colspan]')).toHaveAttribute('colspan', '2');
	});

	it('renders the first cell as a link to rowHref when provided', async () => {
		const { container } = await setup({ rowHref: (row) => `/clients/${row.name}` });

		const link = page.getByRole('link', { name: 'Ada Lovelace' });
		await expect.element(link).toBeVisible();
		await expect.element(link).toHaveAttribute('href', '/clients/Ada Lovelace');
		// Scoped to .table-view: the record view links the same rows, and
		// counting the whole container would count both trees' anchors.
		expect(container.querySelector('.table-view')!.querySelectorAll('a')).toHaveLength(rows.length);
	});

	it('renders plain cells with no links when rowHref is omitted', async () => {
		const { container } = await setup();

		await expect.element(page.getByRole('cell', { name: 'Ada Lovelace' })).toBeVisible();
		expect(container.querySelector('.table-view')!.querySelectorAll('a')).toHaveLength(0);
	});

	it('renders a load-more button and calls onLoadMore when clicked', async () => {
		const onLoadMore = vi.fn();
		await setup({ hasMore: true, onLoadMore });

		const button = page.getByRole('button', { name: 'Load more' });
		await expect.element(button).toBeVisible();
		await button.click();

		expect(onLoadMore).toHaveBeenCalledOnce();
	});

	it('renders no load-more button when hasMore is false', async () => {
		await setup();

		await expect.element(page.getByRole('button', { name: 'Load more' })).not.toBeInTheDocument();
	});

	it('renders no load-more button when hasMore is true but onLoadMore is omitted', async () => {
		await setup({ hasMore: true });

		await expect.element(page.getByRole('button', { name: 'Load more' })).not.toBeInTheDocument();
	});

	it('shows the load-more button in a loading state while isLoadingMore is true', async () => {
		await setup({ hasMore: true, onLoadMore: vi.fn(), isLoadingMore: true });

		const button = page.getByRole('button', { name: 'Load more' });
		await expect.element(button).toBeDisabled();
		await expect.element(button).toHaveAttribute('aria-busy', 'true');
	});

	it('renders loadMoreError next to the load-more button without hiding the existing rows', async () => {
		await setup({
			hasMore: true,
			onLoadMore: vi.fn(),
			loadMoreError: 'Failed to load more records'
		});

		await expect.element(page.getByRole('alert')).toHaveTextContent('Failed to load more records');
		await expect.element(page.getByRole('cell', { name: 'Ada Lovelace' })).toBeVisible();
		await expect.element(page.getByRole('button', { name: 'Load more' })).toBeVisible();
	});

	it('renders no loadMoreError notice when it is omitted', async () => {
		await setup({ hasMore: true, onLoadMore: vi.fn() });

		await expect.element(page.getByRole('alert')).not.toBeInTheDocument();
	});

	it('renders no trailing action column when rowActions is omitted', async () => {
		const { container } = await setup();

		expect(container.querySelectorAll('th')).toHaveLength(2);
	});

	it('renders a trailing header and per-row action content when rowActions is provided', async () => {
		const onRemove = vi.fn();
		await setup({ rowActions: { label: 'Actions', onRemove } });

		await expect.element(page.getByRole('columnheader', { name: 'Actions' })).toBeVisible();

		const buttons = page.getByRole('button', { name: 'Remove' });
		await expect.element(buttons.nth(0)).toBeVisible();
		await buttons.nth(1).click();

		expect(onRemove).toHaveBeenCalledExactlyOnceWith(rows[1]);
	});

	it('spans the action column too when rowActions is provided and rows is empty', async () => {
		const { container } = await setup({ rows: [], rowActions: { label: 'Actions', onRemove: vi.fn() } });

		expect(container.querySelector('td[colspan]')).toHaveAttribute('colspan', '3');
	});

	it('right-aligns a numeric column, header and body cells alike, and leaves a text column start-aligned', async () => {
		await setup({ columns: numericColumns });

		const header = page.getByRole('columnheader', { name: 'Quantity' });
		const cell = page.getByRole('cell', { name: 'Active', exact: true });
		expect(getComputedStyle(header.element()).textAlign).toBe('end');
		expect(getComputedStyle(cell.element()).textAlign).toBe('end');

		const nameHeader = page.getByRole('columnheader', { name: 'Name' });
		expect(getComputedStyle(nameHeader.element()).textAlign).toBe('start');
	});
});

/*
 * The activity ledger's own signature treatment (brief.md's #433
 * amendment): a meta date column at tabular figures, the event in body
 * text, the actor muted -- and, on the Client portal only, the whole
 * table behind a closed disclosure. Class presence is what's asserted,
 * the same way Text.svelte.spec.ts checks its own type-step/tone classes
 * rather than a resolved CSS custom property -- the class is the
 * component's own contract; the token behind it is tokens.spec.ts's job.
 */
describe('the activity ledger treatment (#486)', () => {
	const ledgerColumns = [
		{ label: 'When', accessor: (row: Row) => row.name, variant: 'meta' as const },
		{ label: 'What', accessor: (row: Row) => row.status, variant: 'body' as const },
		{ label: 'Who', accessor: (row: Row) => row.name, variant: 'muted' as const }
	];

	it('marks the meta column body cells, header excluded', async () => {
		const { container } = await setup({ columns: ledgerColumns });

		expect(container.querySelectorAll(':scope .table-view td.meta')).toHaveLength(rows.length);
		expect(container.querySelector(':scope .table-view th.meta')).toBeNull();
	});

	it('marks the body-variant column body cells', async () => {
		const { container } = await setup({ columns: ledgerColumns });

		expect(container.querySelectorAll(':scope .table-view td.variant-body')).toHaveLength(rows.length);
	});

	it('marks the muted column body cells', async () => {
		const { container } = await setup({ columns: ledgerColumns });

		expect(container.querySelectorAll(':scope .table-view td.muted')).toHaveLength(rows.length);
	});

	it('applies no variant class when a column asks for none', async () => {
		const { container } = await setup();

		expect(
			container.querySelector(':scope .table-view td.meta, :scope .table-view td.muted, :scope .table-view td.variant-body')
		).toBeNull();
	});

	// ADR-0022: "the exact instant is carried underneath and never
	// replaced -- the rendered element keeps the full timestamp as its
	// machine-readable value". `<time datetime>` is that carrier.
	it('wraps a datetimeAccessor column in a <time> element carrying the raw instant', async () => {
		const datetimeColumns = [
			{ label: 'Name', accessor: (row: Row) => row.name },
			{
				label: 'When',
				accessor: () => '2 hours ago',
				datetimeAccessor: () => '2027-01-01T10:00:00Z'
			}
		];
		const { container } = await setup({ columns: datetimeColumns });

		const time = container.querySelector(':scope .table-view time')!;
		expect(time).toHaveTextContent('2 hours ago');
		expect(time).toHaveAttribute('datetime', '2027-01-01T10:00:00Z');
	});

	it('renders plain text with no <time> element when datetimeAccessor is omitted', async () => {
		const { container } = await setup();

		expect(container.querySelector(':scope .table-view time')).toBeNull();
	});

	it('renders open with no disclosure wrapper when disclosure is omitted', async () => {
		const { container } = await setup();

		await expect.element(page.getByRole('cell', { name: 'Ada Lovelace' })).toBeVisible();
		expect(container.querySelector('details')).toBeNull();
	});

	// A closed <details> removes its hidden content from the accessibility
	// tree entirely (not merely `not visible`), so `getByRole` -- which
	// queries that tree -- can never resolve a cell inside it; this is the
	// same "no accessible signal for a visual fact" case svelte-tests.md's
	// rule 1 already carves out for the table-view/record-view switch
	// above, checked here by computed style instead.
	it('wraps the table in a closed disclosure named by the disclosure string, when given', async () => {
		const { container } = await setup({ disclosure: 'Everything that has happened' });

		await expect.element(page.getByText('Everything that has happened')).toBeVisible();
		expect(getComputedStyle(container.querySelector('.frame')!).display).toBe('none');

		await page.getByText('Everything that has happened').click();
		await expect.element(page.getByRole('cell', { name: 'Ada Lovelace' })).toBeVisible();
		expect(getComputedStyle(container.querySelector('.frame')!).display).not.toBe('none');
	});
});

/*
 * #508, ADR-0024: below the frame's own content floor, DataTable renders
 * one <dl> per row instead of a <table> that would scroll the whole
 * document sideways. `.table-view`/`.record-view` are queried with
 * `querySelector` rather than an accessible query -- the sanctioned
 * exception in svelte-tests.md's rule 1: both trees carry the same
 * accessible content, and which one CSS hides is exactly the fact under
 * test, so a role/text query can't tell them apart on its own.
 *
 * setup() always pins WIDE first (see the comment above it), so every
 * narrow case here sets the viewport back down AFTER setup() rather than
 * before it -- setting it before would just get overridden.
 */
describe('the record view (#508, ADR-0024)', () => {
	it('shows the record view and hides the table when the container is narrower than the content floor', async () => {
		const { container } = await setup();
		await page.viewport(...NARROW);

		expect(getComputedStyle(container.querySelector('.table-view')!).display).toBe('none');
		expect(getComputedStyle(container.querySelector('.record-view')!).display).not.toBe('none');
		expect(container.querySelectorAll(':scope .record-view dl')).toHaveLength(rows.length);
	});

	it('shows the table and hides the record view when the container is wide enough', async () => {
		const { container } = await setup();

		expect(getComputedStyle(container.querySelector('.table-view')!).display).not.toBe('none');
		expect(getComputedStyle(container.querySelector('.record-view')!).display).toBe('none');
	});

	it('is driven by the frame width, not the viewport: it stacks at a wide viewport when its parent is narrow', async () => {
		await page.viewport(...WIDE);
		const parent = document.createElement('div');
		parent.style.inlineSize = '300px';
		document.body.append(parent);
		try {
			const { container } = await render(
				DataTable<Row>,
				{ columns, rows, emptyMessage: 'No records yet.' },
				{ baseElement: parent }
			);

			expect(getComputedStyle(container.querySelector('.table-view')!).display).toBe('none');
			expect(getComputedStyle(container.querySelector('.record-view')!).display).not.toBe('none');
		} finally {
			parent.remove();
		}
	});

	it('keeps each column label next to its value, one dl per row', async () => {
		const { container } = await setup();
		await page.viewport(...NARROW);

		const [firstRecord] = container.querySelectorAll(':scope .record-view dl');
		expect(firstRecord.querySelector('dt')?.textContent).toBe('Name');
		expect(firstRecord.querySelector('dd')?.textContent).toBe('Ada Lovelace');
	});

	it('keeps rowHref reachable in the record view', async () => {
		const { container } = await setup({ rowHref: (row) => `/clients/${row.name}` });
		await page.viewport(...NARROW);

		const recordView = page.elementLocator(container.querySelector('.record-view')!);
		const link = recordView.getByRole('link', { name: 'Ada Lovelace' });
		await expect.element(link).toBeVisible();
		await expect.element(link).toHaveAttribute('href', '/clients/Ada Lovelace');
	});

	it('keeps rowActions present and operable in the record view', async () => {
		const onRemove = vi.fn();
		const { container } = await setup({ rowActions: { label: 'Actions', onRemove } });
		await page.viewport(...NARROW);

		const recordView = page.elementLocator(container.querySelector('.record-view')!);
		const buttons = recordView.getByRole('button', { name: 'Remove' });
		await expect.element(buttons.nth(0)).toBeVisible();
		await buttons.nth(1).click();

		expect(onRemove).toHaveBeenCalledExactlyOnceWith(rows[1]);
	});
});
