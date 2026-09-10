<script module lang="ts">
	/**
	 * Which of this component's two trees is rendering a caller's snippet
	 * (#666). Both trees exist in the DOM at every width -- the container
	 * query below picks which one is displayed, it does not choose which
	 * one is built -- so a caller's snippet runs twice per row, and an id
	 * it assigns would otherwise exist twice in the document. An id has to
	 * be unique: `aria-describedby` and every other id reference resolve
	 * to the first match in tree order, which is always the `<table>`
	 * copy, whether or not that is the copy on screen.
	 *
	 * A caller that assigns no id ignores this parameter, and a snippet
	 * that declares fewer parameters still satisfies the type, so nothing
	 * has to change for callers this does not concern.
	 */
	export type DataTableView = 'table' | 'record';
</script>

<script lang="ts" generics="T">
	import type { Snippet } from 'svelte';
	import Button from '../atoms/Button.svelte';
	import Link from '../atoms/Link.svelte';
	import Notice from '../atoms/Notice.svelte';

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
		/*
		 * The activity ledger's own signature treatment (brief.md's "One
		 * signature component", amended on #433): "a fixed date column in
		 * `meta` at tabular figures, the event in `body`, the actor in
		 * `on-surface-muted`". Independent of `numeric` -- a meta date reads
		 * start-aligned, not end-aligned like a quantity -- so a column picks
		 * at most one of the two, never both. Applies to the body cell only:
		 * the header keeps this table's one heading style regardless of what
		 * a column's own values look like, the same way `numeric`'s
		 * alignment is the one part of that flag the header shares.
		 */
		variant?: 'meta' | 'body' | 'muted';
		/*
		 * ADR-0022: "the exact instant is carried underneath and never
		 * replaced -- the rendered element keeps the full timestamp as its
		 * machine-readable value, so a screen reader, a hover and a
		 * copy-paste all get the real one." A relative or absolute display
		 * string (dates.ts's formatActivityTimestamp) is what accessor
		 * returns; datetimeAccessor is the same row's ISO instant, wrapping
		 * the cell in a native `<time datetime>` rather than plain text.
		 * Optional because most columns are not a timestamp at all.
		 *
		 * Honored on the first column while `rowHref` is set, too (#905):
		 * the `<time>` wraps the row link rather than replacing it, so the
		 * link's destination, its visible text and its accessible name are
		 * exactly what they would be without this property. A `<time>` that
		 * carries a `datetime` attribute takes phrasing content, and an
		 * `<a>` is phrasing content, so nothing about the link has to bend
		 * for the instant to ride along.
		 */
		datetimeAccessor?: (row: T) => string;
		/*
		 * #264: a Client row's Engagement rollup is more than one line per
		 * row -- Contract status, assigned Doula, Engagement status, each on
		 * its own line, none of it a single string `accessor` could return.
		 * `rowActions`'s `content` is this component's only other row-level
		 * custom-content seam, so this reuses its exact
		 * shape rather than inventing a second one: a per-COLUMN snippet
		 * alongside `rowActions`' per-ROW one. Rendered instead of
		 * `accessor`/`datetimeAccessor` in both the `<table>` and the
		 * record-view `<dl>`, never alongside them -- `accessor` stays
		 * required on a `content` column (never called, but non-optional
		 * so every other column's call site is untouched, with no new
		 * "what if this is missing" branch to test).
		 *
		 * NOT rendered on the first column while `rowHref` is set (#905):
		 * the row link wins there, and the snippet is skipped. This is a
		 * refusal, not an oversight -- a `content` snippet is arbitrary
		 * caller markup, and wrapping arbitrary markup in one row link is
		 * not a promise this component can keep, unlike a formatted
		 * timestamp string, which is exactly the shape `<time>` expects.
		 * A column that needs both puts its snippet anywhere but first.
		 *
		 * Its second parameter is the view discriminator `rowActions.content`
		 * carries, for the same reason and on the same terms -- see
		 * `DataTableView`. Keeping the two shapes identical is the point of
		 * the paragraph above; giving one of them the discriminator and not
		 * the other would make that paragraph false.
		 */
		content?: Snippet<[row: T, view: DataTableView]>;
	}

	interface RowActions<T> {
		label: string;
		/**
		 * Rendered once into each of the two trees, so anything it assigns
		 * an id to folds the view discriminator into that id -- see
		 * `DataTableView`.
		 */
		content: Snippet<[row: T, view: DataTableView]>;
	}

	interface Properties<T> {
		columns: Column<T>[];
		rows: T[];
		/**
		 * Turns each row into a destination, reached from the FIRST
		 * column's cell only -- one link per row, named by that column's
		 * `accessor(row)`, in the `<table>` and in the record view alike.
		 * The Clients route states the same rule from the caller's side;
		 * it belongs here, where every caller can read it.
		 *
		 * What it does to the other per-column seams on that first column
		 * (#905): `datetimeAccessor` is still honored -- a `<time>` wraps
		 * the link -- while `content` is not rendered at all, the link
		 * winning over it. Each property's own comment above says so.
		 */
		rowHref?: (row: T) => string;
		rowActions?: RowActions<T>;
		hasMore?: boolean;
		onLoadMore?: () => void;
		isLoadingMore?: boolean;
		loadMoreError?: string;
		emptyMessage: string;
		/**
		 * Wraps the whole table in a closed-by-default `<details>`, named by
		 * this string -- the Client portal's own placement for the activity
		 * ledger (brief.md's #433 amendment: "behind a closed disclosure in
		 * the Client portal"). Absent everywhere else this component is used
		 * today (the hub feed and the staff Engagement page render it open,
		 * per the same amendment's "sits low on the page").
		 */
		disclosure?: string;
	}

	let {
		columns,
		rows,
		rowHref,
		rowActions,
		hasMore = false,
		onLoadMore,
		isLoadingMore = false,
		loadMoreError,
		disclosure,
		emptyMessage
	}: Properties<T> = $props();

	/*
	 * Whether a given cell actually renders its column's snippet (#740),
	 * spent as a class on the `<td>` so the cell's geometry follows from
	 * the COLUMN rather than from a class name the caller's markup had to
	 * carry. `columnIndex === 0 && rowHref` is the documented refusal on
	 * `Column.content`: the row link wins there and the cell is one line
	 * of link text, so it is not a content cell for styling either.
	 *
	 * This restates a condition the `cell` snippet's own `{#if}` chain
	 * writes inline, and that is not an oversight: the inline form is what
	 * narrows `rowHref` and `column.content` from optional to callable for
	 * the compiler, and a call through this function narrows neither. The
	 * two must agree, so a spec asserts the pair together -- a linked
	 * first column renders the link AND takes no content padding -- and
	 * disagreement fails there rather than showing up on screen.
	 */
	function isContentCell(column: Column<T>, columnIndex: number): boolean {
		return Boolean(column.content) && !(columnIndex === 0 && rowHref);
	}
</script>

<!--
	One cell's content, rendered by the `<table>` body cell and by the
	record view's `<dd>` alike (#905). It used to be the same `{#if}` chain
	written out twice, which made "the two copies stay in agreement" a
	discipline; a single snippet makes it a structural fact instead.

	The first branch is the row link carrying its column's instant: a
	`<time>` with a `datetime` attribute takes phrasing content, so it
	legally wraps the `<a>`, and the link's href, text and accessible name
	come out identical to the bare-link branch below it. `content` on a
	linked first column is deliberately unreachable -- see `Column.content`.
-->
{#snippet cell(column: Column<T>, columnIndex: number, row: T, view: DataTableView)}
	{#if columnIndex === 0 && rowHref && column.datetimeAccessor}
		<time datetime={column.datetimeAccessor(row)}
			><Link href={rowHref(row)} label={column.accessor(row)} /></time
		>
	{:else if columnIndex === 0 && rowHref}
		<Link href={rowHref(row)} label={column.accessor(row)} />
	{:else if column.content}
		{@render column.content(row, view)}
	{:else if column.datetimeAccessor}
		<time datetime={column.datetimeAccessor(row)}>{column.accessor(row)}</time>
	{:else}
		{column.accessor(row)}
	{/if}
{/snippet}

{#snippet ledgerContent()}
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
							<!-- `content` has no counterpart on the record view's
							     own `<dd>` below, and that asymmetry is deliberate:
							     all it turns on is `td.content`'s vertical padding,
							     which a `<dd>` already carries for every cell. -->
							<td
								class:numeric={column.numeric}
								class:meta={column.variant === 'meta'}
								class:variant-body={column.variant === 'body'}
								class:muted={column.variant === 'muted'}
								class:content={isContentCell(column, columnIndex)}
							>
								{@render cell(column, columnIndex, row, 'table')}
							</td>
						{/each}
						{#if rowActions}
							<td>
								{@render rowActions.content(row, 'table')}
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
						<!-- Neither `variant-body` nor `content` is written here,
						     and for one reason: each would name a rule that
						     overrides nothing in this tree. See the mirror
						     block in the style below. -->
						<dd
							class:numeric={column.numeric}
							class:meta={column.variant === 'meta'}
							class:muted={column.variant === 'muted'}
						>
							{@render cell(column, columnIndex, row, 'record')}
						</dd>
					{/each}
					{#if rowActions}
						<dt>{rowActions.label}</dt>
						<dd>{@render rowActions.content(row, 'record')}</dd>
					{/if}
				</dl>
			{/each}
		{/if}
	</div>

	{#if loadMoreError}
		<Notice variant="error" message={loadMoreError} />
	{/if}

	{#if hasMore && onLoadMore}
		<Button label="Load more" variant="secondary" loading={isLoadingMore} onClick={onLoadMore} />
	{/if}
</stack-l>
{/snippet}

{#if disclosure}
	<details>
		<summary>{disclosure}</summary>
		{@render ledgerContent()}
	</details>
{:else}
	{@render ledgerContent()}
{/if}

<style>
	@layer components {
		/* A closed <details> hides every child but <summary> per the HTML
		   spec, but that is a user-agent-origin rule, and a plain author
		   rule -- whatever gives .frame's own `stack-l` its base `display`
		   -- wins over user-agent styles regardless of specificity (CSS
		   cascade origin order, not the layer above: this component's own
		   @layer components still outranks the UA layer). `!important`
		   makes the closed state explicit rather than depending on that
		   base rule happening to stay silent about it. */
		details:not([open]) > .frame {
			display: none !important;
		}

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
			/* The frame is a column flex container now (ADR-0039), which
			   stretches its items across the inline axis -- and stretching
			   is exactly the `inline-size: 100%` the paragraph above says
			   #542 deleted, arriving by another route. `start` hands the
			   table back its own shrink-to-fit width, so it still stops
			   where its columns are satisfied. */
			align-self: start;
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

		/* The activity ledger's three body-cell treatments (brief.md's
		   #433 amendment). Body-only, unlike .numeric above: the header
		   keeps this table's one heading style regardless of what a
		   column's values look like. */
		td.meta {
			font-size: var(--text-meta-size);
			font-weight: var(--text-meta-weight);
			line-height: var(--text-meta-leading);
			letter-spacing: var(--text-meta-tracking);
			font-variant-numeric: tabular-nums;
		}

		td.variant-body {
			font-size: var(--text-body-size);
		}

		td.muted {
			color: var(--color-on-surface-muted);
		}

		/* A cell rendering a caller's snippet, which is the one cell that
		   can be more than a single line (#264, #740). What it needs is
		   vertical room INSIDE the cell: `th, td` above writes
		   `padding: 0 var(--space-3)`, so a three-line rollup measured
		   75.2px tall in a 75.2px row -- every line touching the row rule
		   above or below it.

		   It does NOT need a height override. A table cell's `block-size`
		   is a MINIMUM, not a ceiling (CSS 2.1 17.5.3: the row is the
		   greater of the specified height and the content's), so the
		   2.5rem the brief's Density section fixes has always let a taller
		   cell grow -- measured, not assumed. Two rules used to say
		   otherwise here, keyed on `.rollup-list`, a class only the Clients
		   route's own snippet carried and nothing on `Column<T>` ever
		   mentioned; both overrode nothing, and the `:global` that reached
		   for that class leaked its list treatment app-wide from a
		   component that never renders it. The treatment moved to the
		   route that writes the markup, where its own scope holds it.

		   `--space-1` rather than `--space-2`, which is what the record
		   view's `dd` spends: the padding lands inside the 2.5rem floor
		   (`box-sizing: border-box`, reset.css), so a ONE-line content
		   cell -- the Clients list's Portal invite column is one -- must
		   still fit that floor or every row in the table grows and
		   Skeleton stops reserving the right space. Both tokens are
		   container-relative clamps and so is `body-sm`, so the margin is
		   not the same at every width: one line plus 2 x --space-1 fits
		   the floor everywhere the table view renders, while 2 x
		   --space-2 stops fitting as the frame widens -- measured going
		   over at the drag surface's own full width, which is enough to
		   disqualify it.

		   No counterpart for the record view's `<dd>`: it carries
		   `padding-block` for EVERY cell already (below) and sets no
		   height at all, so a rule there would override nothing. */
		td.content {
			padding-block: var(--space-1);
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
		   This used to carry `margin-block-start: 0` to cancel the frame's
		   own stack-l spacing, because `> * + *` counted the hidden
		   .table-view before it as a preceding sibling. The stack spaces
		   with `gap` now (ADR-0039), and a `display: none` child is not a
		   flex item at all, so only one of these two views is ever in the
		   flow and there is no gap to cancel. */

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

		/* The record view's own mirror of td.meta/muted above -- two of
		   the three, not all three (#740). `variant: 'body'` asks for
		   `--text-body-size`, which is what `th, td` has to be overridden
		   to give and what a `<dd>` here already inherits from `.frame >
		   *`: measured, `dd.variant-body` and its own `.record-view`
		   ancestor both compute to 15.13px at a 390px frame. A rule
		   restating that would have overridden nothing, so neither it nor
		   the class that selected it is written -- the same rule this
		   ticket applied to `td.content`'s missing `<dd>` counterpart. */
		.record-view dd.meta {
			font-size: var(--text-meta-size);
			font-weight: var(--text-meta-weight);
			line-height: var(--text-meta-leading);
			letter-spacing: var(--text-meta-tracking);
			font-variant-numeric: tabular-nums;
		}

		.record-view dd.muted {
			color: var(--color-on-surface-muted);
		}

		/* The Client-portal disclosure wrapper (brief.md's #433 amendment).
		   No marker/appearance override: the platform triangle is what GOV.UK's
		   own Details component keeps, and this repo has no established
		   disclosure treatment of its own to depart to. */
		summary {
			cursor: pointer;
			font-family: var(--font-family-base);
			font-size: var(--text-label-size);
			font-weight: var(--font-weight-medium);
			color: var(--color-on-surface-variant);
			padding-block: var(--space-2);
		}
	}
</style>
