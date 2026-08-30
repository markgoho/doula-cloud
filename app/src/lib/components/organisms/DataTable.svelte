<script lang="ts" generics="T">
	import type { Snippet } from 'svelte';
	import Button from '../atoms/Button.svelte';
	import Link from '../atoms/Link.svelte';

	interface Column<T> {
		label: string;
		accessor: (row: T) => string;
		/*
		 * Explicit, not inferred from what accessor(row) returns (#509):
		 * accessor already formats to a string -- Billing's Quantity column
		 * returns "+3", not 3 -- so there is no numeric value left at render
		 * time to infer from. The caller knows a column is a quantity or an
		 * amount before it is ever formatted, so it says so here. GOV.UK's
		 * Table guidance: right-align numbers so digits compare by place
		 * value; tabular figures are what actually lines them up in a
		 * proportional typeface, and the brief asks for both together
		 * wherever a number is compared (brief.md's Typography section), so
		 * one flag turns on both.
		 */
		numeric?: boolean;
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
					<th scope="col" class:numeric={column.numeric}>{column.label}</th>
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
							<td class:numeric={column.numeric}>
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

		/* "Compact rows, airy forms": the brief's Density section fixes a
		   table row at 40px and body-sm, so a person scanning fifty Clients
		   sees as many as will fit. The height is set here rather than left
		   to padding because a Skeleton has to reserve exactly this much
		   space before the rows arrive -- see Skeleton.layoutShift.svelte.spec.ts. */
		th,
		td {
			block-size: 2.5rem;
			padding: 0 var(--space-3);
			font-size: var(--text-body-sm-size);
			text-align: start;
			border-block-end: var(--border-thin) solid var(--color-outline-variant);
		}

		th {
			font-weight: var(--font-weight-semibold);
		}

		/* A numeric column's header moves with its body cells (#509) --
		   both carry the same class off the same Column, so they can never
		   drift apart the way two separately-set rules could. */
		th.numeric,
		td.numeric {
			text-align: end;
			font-variant-numeric: tabular-nums;
		}
	}
</style>
