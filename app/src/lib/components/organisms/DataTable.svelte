<script lang="ts" generics="T">
	import type { Snippet } from 'svelte';
	import Button from '../atoms/Button.svelte';
	import Link from '../atoms/Link.svelte';

	interface Column<T> {
		label: string;
		accessor: (row: T) => string;
	}

	interface RowActions<T> {
		label: string;
		content: Snippet<[row: T]>;
	}

	interface Properties<T> {
		columns: Column<T>[];
		rows: T[];
		rowHref?: (row: T) => string;
		rowActions?: RowActions<T>;
		hasMore?: boolean;
		onLoadMore?: () => void;
		emptyMessage: string;
	}

	let { columns, rows, rowHref, rowActions, hasMore = false, onLoadMore, emptyMessage }: Properties<T> =
		$props();
</script>

<stack-l>
	<table>
		<thead>
			<tr>
				{#each columns as column (column.label)}
					<th scope="col">{column.label}</th>
				{/each}
				{#if rowActions}
					<th scope="col">{rowActions.label}</th>
				{/if}
			</tr>
		</thead>
		<tbody>
			{#if rows.length === 0}
				<tr>
					<td colspan={columns.length + (rowActions ? 1 : 0)}>{emptyMessage}</td>
				</tr>
			{:else}
				{#each rows as row, index (index)}
					<tr>
						{#each columns as column, columnIndex (column.label)}
							<td>
								{#if columnIndex === 0 && rowHref}
									<Link href={rowHref(row)} label={column.accessor(row)} />
								{:else}
									{column.accessor(row)}
								{/if}
							</td>
						{/each}
						{#if rowActions}
							<td>
								{@render rowActions.content(row)}
							</td>
						{/if}
					</tr>
				{/each}
			{/if}
		</tbody>
	</table>
	{#if hasMore && onLoadMore}
		<Button label="Load more" variant="secondary" onClick={onLoadMore} />
	{/if}
</stack-l>

<style>
	@layer components {
		table {
			inline-size: 100%;
			border-collapse: collapse;
		}

		th,
		td {
			padding: var(--space-2) var(--space-3);
			text-align: start;
			border-block-end: var(--border-thin) solid var(--color-outline-variant);
		}

		th {
			font-weight: var(--font-weight-semibold);
		}
	}
</style>
