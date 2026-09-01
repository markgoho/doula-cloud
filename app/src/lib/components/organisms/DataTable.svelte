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

<stack-l class="frame">
	<table class="table-view">
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

	<!--
		The record view (#508, ADR-0024): the same columns/rows, one <dl> per
		record instead of one row of a shared column grid, for the container
		widths too narrow to hold the table without scrolling the whole
		document sideways. Generated from the same Column config as the
		<table> above, so no route ever hand-authors this second tree.
	-->
	<!-- A plain div, not <stack-l>: an unregistered custom element toggled
	     between display:block and display:none by this same @container
	     rule (below) never actually hid, in both the browser this was
	     built against and the test suite -- a real div doesn't have that
	     failure mode. -->
	<div class="record-view">
		{#if rows.length === 0}
			<p>{emptyMessage}</p>
		{:else}
			{#each rows as row, index (index)}
				<dl>
					{#each columns as column, columnIndex (column.label)}
						<dt>{column.label}</dt>
						<dd class:numeric={column.numeric}>
							{#if columnIndex === 0 && rowHref}
								<Link href={rowHref(row)} label={column.accessor(row)} />
							{:else}
								{column.accessor(row)}
							{/if}
						</dd>
					{/each}
					{#if rowActions}
						<dt>{rowActions.label}</dt>
						<dd>{@render rowActions.content(row)}</dd>
					{/if}
				</dl>
			{/each}
		{/if}
	</div>

	{#if hasMore && onLoadMore}
		<Button label="Load more" variant="secondary" onClick={onLoadMore} />
	{/if}
</stack-l>

<style>
	@layer components {
		/* The frame is a container, so the switch below reads the room
		   DataTable's own wrapper has rather than the room the window has.
		   Named for the same reason StaffTopBar names its own (#540): body
		   is a containment context too, and an unnamed query that lost this
		   declaration would silently resolve against the page instead. */
		.frame {
			container: data-table / inline-size;
		}

		/* The base size re-resolved against the frame (#544): a `cqi`
		   resolves against the nearest ANCESTOR container, so `.frame`
		   cannot answer its own. The cells below declare `body-sm` and are
		   unaffected; this covers everything in the record view that does
		   not. */
		.frame > * {
			font-size: var(--text-body-size);
		}

		/* No inline size at all, which is the whole of #542's answer: a
		   table with an auto width shrink-to-fits by the CSS table
		   algorithm -- max(min-content, min(max-content, available)) -- so
		   it grows with its columns and stops when they are satisfied. The
		   `inline-size: 100%` that used to sit here made every pixel past
		   that land inside a cell instead: measured on the drag surface,
		   the six-column demo's Email column reached 998px at 3151px
		   available for an address that fits in 184px, and reading one row
		   meant crossing the whole screen. Deleting the declaration writes
		   no width, so there is nothing here for a person to have picked
		   off a screen (ADR-0024). The table stays at the inline start
		   because that is where a table with no alignment CSS goes, and a
		   table's first column is what the eye scans down; a general rule
		   for leftover space is #543's, and a table that authors no
		   alignment inherits whatever that decides. */
		.table-view {
			display: none;
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
			/* The companion to shrink-to-fit above (#542): a cell above the
			   floor never wraps to save room, so max-content is the sum of
			   the longest unbroken value in each column, and one column
			   here is unbounded -- a Client's history renders "Birth
			   Engagement refused: <reason>", and a Practice types that
			   reason. --measure is this repo's existing answer to how wide
			   a run of prose may be and is font-relative, so it is not a
			   width chosen by looking at a screen; it is inert for every
			   bounded column, which is every other column built today.
			   `anywhere` because a pasted URL offers no break opportunity
			   for a ceiling to act on -- the same failure as #530, #548 and
			   #552 -- and unlike `break-word` it lowers the min-content
			   size, so the cap can actually take effect. */
			max-inline-size: var(--measure);
			overflow-wrap: anywhere;
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

		/* Unavoidable (#564): a <table> and one <dl> per record are
		   different DOM trees, not the same content laid out differently
		   -- a <table>'s row-and-column binding is structural, so no
		   intrinsic CSS mechanism (grid areas, flex-wrap, a fluid track
		   list) turns one into the other. Every Layout's own case against
		   container queries ("circuit breakers... I'd sooner not have
		   them anywhere I know they're not needed", #520) is about
		   REARRANGING content that stays one tree; this is the
		   documented exception, picking which of two trees renders.

		   The content floor, re-measured 2026-09-01 in the canonical
		   environment (#564): the previous 46rem (736px) was measured
		   with `overflow-wrap: anywhere` live on td/th, so the sweep
		   watched the cells rescue themselves by breaking mid-word
		   instead of watching when the table actually stopped fitting --
		   it never found a break, and 736px went in with a margin nobody
		   could verify. 48rem (768px), swept with wrapping neutralized on
		   the machine that measured it, held there but read as
		   insufficient on CI's own runner: the same font bytes rasterize
		   wider on CI's Linux/Chromium, so Staff's Members table (Name,
		   Email, Roles, Employment type, Works from, plus its Actions
		   column of three buttons) -- the widest DataTable built today --
		   needs 780px, not 768px, to stop overflowing
		   /style-guide/data-table's own demo of that exact shape. 48.75rem
		   is that fixed point, measured in CI's own Linux/Chromium, the
		   one named environment a floor's minimality is judged against
		   (CONTEXT.md's Content floor entry), with no margin added beyond
		   it. It is the frame's own inline size that is measured, never
		   the viewport (ADR-0024). A future table wider than this floor
		   moves it. */
		@container data-table (min-width: 48.75rem) {
			.table-view {
				display: table;
			}

			.record-view {
				display: none;
			}
		}

		/* One <dl> per record (#508, ADR-0024) rather than a mangled
		   <table>, which strips table semantics in Safari and Firefox.
		   margin-block-start: 0 overrides the frame's own stack-l spacing
		   (primitives.css) -- record-view is the first VISIBLE child
		   whenever it renders at all, since the hidden .table-view before
		   it still counts as "a preceding sibling" to that selector. */
		.record-view {
			margin-block-start: 0;
		}

		.record-view dl {
			display: grid;
			grid-template-columns: auto 1fr;
			gap: 0 var(--space-4);
			margin: 0;
		}

		.record-view dl + dl {
			margin-block-start: var(--space-4);
		}

		.record-view dt {
			font-weight: var(--font-weight-medium);
			color: var(--color-on-surface-variant);
			padding-block: var(--space-2);
			border-block-end: var(--border-thin) solid var(--color-outline-variant);
		}

		/* The same unbounded column, seen at the other end (#542): the
		   free-text history value put this view 62px past its frame at
		   320px, because `1fr`'s automatic minimum is the min-content size
		   and a pasted URL has none. `anywhere` gives the value break
		   opportunities, which lowers that minimum and lets the track
		   shrink -- the fix #534's exercise teaches and #552 landed. */
		.record-view dd {
			margin: 0;
			color: var(--color-on-surface);
			padding-block: var(--space-2);
			border-block-end: var(--border-thin) solid var(--color-outline-variant);
			overflow-wrap: anywhere;
		}

		.record-view dd.numeric {
			text-align: end;
			font-variant-numeric: tabular-nums;
		}
	}
</style>
