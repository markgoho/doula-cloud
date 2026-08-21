import { createRawSnippet } from 'svelte';
import { page } from 'vitest/browser';
import { describe, expect, it, vi } from 'vitest';
import { render } from 'vitest-browser-svelte';
import DataTable from './DataTable.svelte';

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

interface SetupOptions {
	rows?: Row[];
	rowHref?: (row: Row) => string;
	rowActions?: { label: string; onRemove: (row: Row) => void };
	hasMore?: boolean;
	onLoadMore?: () => void;
	emptyMessage?: string;
}

async function setup({
	rows: rowsOption = rows,
	rowHref,
	rowActions,
	hasMore = false,
	onLoadMore,
	emptyMessage = 'No records yet.'
}: SetupOptions = {}) {
	const { container } = await render(DataTable<Row>, {
		columns,
		rows: rowsOption,
		rowHref,
		rowActions: rowActions && {
			label: rowActions.label,
			content: removeActionSnippet(rowActions.onRemove)
		},
		hasMore,
		onLoadMore,
		emptyMessage
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

		await expect.element(page.getByText('No records yet.')).toBeVisible();
		expect(container.querySelector('td[colspan]')).toHaveAttribute('colspan', '2');
	});

	it('renders the first cell as a link to rowHref when provided', async () => {
		const { container } = await setup({ rowHref: (row) => `/clients/${row.name}` });

		const link = page.getByRole('link', { name: 'Ada Lovelace' });
		await expect.element(link).toBeVisible();
		await expect.element(link).toHaveAttribute('href', '/clients/Ada Lovelace');
		expect(container.querySelectorAll('a')).toHaveLength(rows.length);
	});

	it('renders plain cells with no links when rowHref is omitted', async () => {
		const { container } = await setup();

		await expect.element(page.getByText('Ada Lovelace')).toBeVisible();
		expect(container.querySelectorAll('a')).toHaveLength(0);
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
});
