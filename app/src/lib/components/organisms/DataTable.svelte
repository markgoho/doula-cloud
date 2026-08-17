<script lang="ts" generics="T">
	import Button from '../atoms/Button.svelte';

	interface Column<T> {
		label: string;
		accessor: (row: T) => string;
	}

	interface Properties<T> {
		columns: Column<T>[];
		rows: T[];
		rowHref?: (row: T) => string;
		hasMore?: boolean;
		onLoadMore?: () => void;
		emptyMessage: string;
	}

	let { columns, rows, rowHref, hasMore = false, onLoadMore, emptyMessage }: Properties<T> = $props();
</script>

<stack-l>
	<table>
		<thead>
			<tr>
				{#each columns as column (column.label)}
					<th scope="col">{column.label}</th>
				{/each}
			</tr>
		</thead>
		<tbody>
			{#if rows.length === 0}
				<tr>
					<td colspan={columns.length}>{emptyMessage}</td>
				</tr>
			{:else}
				{#each rows as row, index (index)}
					<tr>
						{#each columns as column, columnIndex (column.label)}
							<td>
								{#if columnIndex === 0 && rowHref}
									<a href={rowHref(row)}>{column.accessor(row)}</a>
								{:else}
									{column.accessor(row)}
								{/if}
							</td>
						{/each}
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
			border-block-end: var(--border-thin) solid var(--color-border);
		}

		th {
			font-weight: var(--font-weight-semibold);
		}
	}
</style>
