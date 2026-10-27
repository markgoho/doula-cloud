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
	import Details from '../atoms/Details.svelte';
	import Link from '../atoms/Link.svelte';
	import Notice from '../atoms/Notice.svelte';
	import EmptyState, { type EmptyStateAction } from '../molecules/EmptyState.svelte';
	import SummaryList, { type SummaryListRow } from '../molecules/SummaryList.svelte';

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
		 * row -- Contract status, the Doula on it, Engagement status, each on
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
		/*
		 * The one column of a table that wraps (#2091): a Practice's own
		 * prose -- a Visit's notes -- beside columns of facts. It takes the
		 * width the other columns leave and wraps its text there, while
		 * every other plain cell keeps its content width on one line, so a
		 * date, a Type or a name is never narrowed to make room for a
		 * sentence. Without it, shrink-to-fit (#542) sizes every column to
		 * its content, a sentence pushes the table's floor past the room a
		 * page has, and a lower floor broke "Prenatal" and "Change"
		 * mid-word.
		 *
		 * The floor of a table with this column is measured with the
		 * column at its minimum -- its longest word -- not its longest line
		 * (`floor.ts`'s `measureTableFit`). One per table: a second would
		 * share the room by content again, which is what this exists to
		 * stop. It does not combine with `columnWidths="even"`, whose fixed
		 * layout gives every column the same share whatever it holds.
		 */
		wraps?: boolean;
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
		/**
		 * What an empty list shows (#2015): the `EmptyState` line, drawn in
		 * place of the whole table, so no header row stands over a list with
		 * nothing in it. `emptyAction` is the one action that makes the first
		 * row, where the screen has one that is not already beside the list.
		 */
		emptyMessage: string;
		emptyAction?: EmptyStateAction;
		/**
		 * Wraps the whole table in a closed-by-default `Details` (#1520),
		 * named by this string -- the Client portal's own placement for the
		 * activity ledger (brief.md's #433 amendment: "behind a closed
		 * disclosure in the Client portal"). Absent everywhere else this
		 * component is used today (the hub feed and the staff Engagement
		 * page render it open, per the same amendment's "sits low on the
		 * page").
		 */
		disclosure?: string;
		/**
		 * How the table's columns take their widths (#2038). `content`, the
		 * default, is the shrink-to-fit #542 chose: each column as wide as
		 * its longest value, so a table stops where its columns are
		 * satisfied. `even` gives every column an equal share of one width
		 * the content does not decide, for sibling tables of the same
		 * columns read one under the next -- the portal's three Invoice
		 * sections -- so a column starts at the same place in each (Law of
		 * Similarity). It changes the table only; the record view is
		 * already one card width.
		 */
		columnWidths?: 'content' | 'even';
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
		emptyMessage,
		emptyAction,
		columnWidths = 'content'
	}: Properties<T> = $props();

	/*
	 * Where focus goes after "Load more" (#2105): to the first row it
	 * added, in whichever tree is displayed. Left alone, focus stays on
	 * the button, and when the last page removes the button it falls to
	 * the document, so the next Tab starts again at the top of the page.
	 * A row is not a control, so it takes `tabindex="-1"` only when it is
	 * focused this way, and Tab moves on from it through the rows'
	 * own links.
	 */
	let frame = $state<HTMLElement>();
	let firstNewRowIndex = $state<number>();

	function loadMore() {
		firstNewRowIndex = rows.length;
		onLoadMore?.();
	}

	$effect(() => {
		if (firstNewRowIndex === undefined || isLoadingMore) return;
		const index = firstNewRowIndex;
		// Until the rows arrive, or the load says it failed.
		if (rows.length <= index) {
			if (loadMoreError) firstNewRowIndex = undefined;
			return;
		}
		firstNewRowIndex = undefined;
		// Both trees hold the row; the container query displays one.
		const candidates = frame!.querySelectorAll<HTMLElement>(
			`.table-view > tbody > tr:nth-child(${index + 1}), .record-view > .record:nth-child(${index + 1})`
		);
		for (const candidate of candidates) {
			if (!candidate.checkVisibility()) continue;
			candidate.setAttribute('tabindex', '-1');
			candidate.focus();
		}
	});

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

	/*
	 * One record card's rows (#2021, #2022): every column after the
	 * first, which is the card's title, as a `SummaryList` row. The value
	 * text is the column's accessor; what draws is the `cell` snippet,
	 * through the list's `value` snippet below.
	 *
	 * A row whose accessor says nothing is left out (#2022): a card of
	 * twelve rows where four read blank makes the eye search the blanks
	 * for a value. So a `content` column's accessor has to say something
	 * whenever its snippet draws something -- the accessor is what decides
	 * whether the row is there at all.
	 */
	interface RecordRow extends SummaryListRow {
		column: Column<T>;
		columnIndex: number;
	}

	function recordRows(row: T): RecordRow[] {
		return columns
			.map((column, columnIndex) => ({
				label: column.label,
				value: column.accessor(row),
				column,
				columnIndex
			}))
			.filter((field) => field.columnIndex > 0 && field.value.trim() !== '');
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
<stack-l class="frame" bind:this={frame}>
	<!-- One empty state, not one per tree (#2015): the container query
	     below picks which tree shows, and both would draw it. An empty
	     message draws nothing: the portal's owed list says nothing when
	     the total above it has said it all (#982). -->
	{#if rows.length === 0}
		{#if emptyMessage}
			<EmptyState message={emptyMessage} action={emptyAction} />
		{/if}
	{:else}
		<table
			class="table-view"
			class:even={columnWidths === 'even'}
			class:has-wrapping-column={columns.some((column) => column.wraps)}
		>
			<thead>
				<tr>
					{#each columns as column (column.label)}
						<th scope="col" class:numeric={column.numeric} class:wraps={column.wraps}>{column.label}</th>
					{/each}
					{#if rowActions}
						<!-- `actions` names the column of this table's own controls for
						     the floor check's table-fit measurement (#1981, `floor.ts`);
						     no rule here selects it. -->
						<th scope="col" class="actions">{rowActions.label}</th>
					{/if}
				</tr>
			</thead>
			<tbody>
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
								class:wraps={column.wraps}
							>
								{@render cell(column, columnIndex, row, 'table')}
							</td>
						{/each}
						{#if rowActions}
							<td class="actions">
								{@render rowActions.content(row, 'table')}
							</td>
						{/if}
					</tr>
				{/each}
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
			{#each rows as row, index (index)}
				<!-- Each record is a card (#2022): its first column as the
				     title, the thing a reader finds a record by (Serial
				     Position), the rest as `SummaryList` rows (#2021), and its
				     actions at the foot. The title is a block on the
				     subheading step, not a heading: this component cannot
				     know the level its page needs, and the title is often the
				     row's link, which the `Heading` atom does not hold. -->
				{@const fields = recordRows(row)}
				<div class="record">
					<div class="record-title">{@render cell(columns[0]!, 0, row, 'record')}</div>
					{#if fields.length > 0}
						<SummaryList rows={fields}>
							{#snippet value(field)}
								<!-- Neither `variant-body` nor `content` is written
								     here, and for one reason: each would name a rule
								     that overrides nothing in this tree. See the
								     mirror block in the style below. -->
								<span
									class="cell"
									class:numeric={field.column.numeric}
									class:meta={field.column.variant === 'meta'}
									class:muted={field.column.variant === 'muted'}
									>{@render cell(field.column, field.columnIndex, row, 'record')}</span
								>
							{/snippet}
						</SummaryList>
					{/if}
					{#if rowActions}
						<!-- The actions are the card's own, not one more key and
						     value: GOV.UK's summary card puts them in its frame. -->
						<cluster-l class="record-actions" space="var(--space-related)" align="center">
							{@render rowActions.content(row, 'record')}
						</cluster-l>
					{/if}
				</div>
			{/each}
		</div>
	{/if}

	{#if loadMoreError}
		<Notice variant="error" message={loadMoreError} />
	{/if}

	{#if hasMore && onLoadMore}
		<Button label="Load more" variant="secondary" loading={isLoadingMore} onClick={loadMore} />
	{/if}
</stack-l>
{/snippet}

{#if disclosure}
	<Details summary={disclosure}>
		{@render ledgerContent()}
	</Details>
{:else}
	{@render ledgerContent()}
{/if}

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
		/* Below this component's own floor the two trees read the same two
		   properties the floor's query reads (#2063), so a route can lower
		   a table's floor as well as raise it. See the floor comment. */
		.table-view {
			display: var(--data-table-table-display, none);
			border-collapse: collapse;
			/* The frame is a column flex container now (ADR-0039), which
			   stretches its items across the inline axis -- and stretching
			   is exactly the `inline-size: 100%` the paragraph above says
			   #542 deleted, arriving by another route. `start` hands the
			   table back its own shrink-to-fit width, so it still stops
			   where its columns are satisfied. */
			align-self: start;
		}

		/* `columnWidths="even"` (#2038): `table-layout: fixed` with no
		   column widths shares the table's width equally, whatever the
		   rows hold, so two tables of the same columns line up. The width
		   is the reading width, --measure, and never past the frame: a
		   whole-frame width is #542's row that crosses the screen. */
		.table-view.even {
			inline-size: min(100%, var(--measure));
			table-layout: fixed;
		}

		/* A table with a wrapping column (#2091). The auto table layout
		   shares the room between a column's one-word width and its
		   one-line width in proportion to the gap between the two, so
		   without this every column gave up width together and a date
		   wrapped beside a sentence. A cell that cannot wrap has no gap,
		   so the wrapping column, the only one left with one, takes all of
		   the shortfall, and the table still stops where its columns are
		   satisfied (#542). A caller's snippet cell is left to wrap: it
		   may be more than one line by design (#264). A row's controls
		   still wrap between each other (#1981), but never inside a word:
		   `anywhere` below let "Change" break into "Chang" and "e". */
		.has-wrapping-column :is(th, td):not(.wraps, .content, .actions) {
			white-space: nowrap;
		}

		.has-wrapping-column .actions {
			overflow-wrap: normal;
		}

		/* Before the floor's query, which overrides it at the same
		   specificity by coming later. */
		.record-view {
			display: var(--data-table-records-display, block);
		}

		/* "Compact rows, airy forms": the brief's Density section fixes a
		   table row at 40px and body-sm, so a person scanning fifty Clients
		   sees as many as will fit, and at 44px when any input can tap
		   (#1843) -- `--row-height` in tokens.css makes that switch. The
		   height is set here rather than left to padding because a Skeleton
		   has to reserve exactly this much space before the rows arrive --
		   see Skeleton.layoutShift.svelte.spec.ts. */
		th,
		td {
			block-size: var(--row-height);
			padding: 0 var(--space-3);
			font-size: var(--text-body-sm-size);
			/* The cells are `body-sm`, so they take `body-sm`'s own line
			   height (ADR-0060: each role keeps its own leading) rather
			   than inheriting `body`'s 1.55 from the frame (#1945). */
			line-height: var(--text-body-sm-leading);
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
		   column's values look like.

		   `nowrap` is the brief's "a fixed date column" (#1945): the date
		   holds one line and the event beside it takes the wrap. A meta
		   value is a date or a date range, never a Practice's prose, so its
		   one line is short and bounded -- the longest built today is an
		   on-call window, "Oct 1, 2026 – Nov 1, 2026". The record view's
		   `dd.meta` below does not take it: a `<dd>` at 320px has to wrap. */
		td.meta {
			font-size: var(--text-meta-size);
			font-weight: var(--text-meta-weight);
			line-height: var(--text-meta-leading);
			letter-spacing: var(--text-meta-tracking);
			font-variant-numeric: tabular-nums;
			white-space: nowrap;
		}

		td.variant-body {
			font-size: var(--text-body-size);
			line-height: var(--text-body-leading);
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

		   The padding is what the row leaves around ONE line, split over
		   the two edges, not a spacing step. It lands inside the row
		   (`box-sizing: border-box`, reset.css), so a one-line content
		   cell -- the Clients list's Portal invite column is one -- must
		   still fit `--row-height` or every row in the table grows and
		   Skeleton stops reserving the right space. A spacing step cannot
		   promise that: the step, `body-sm` and the line height move on
		   the ramp at different rates, so a fixed `--space-1` fitted until
		   the 16px type floors (ADR-0060, #1943) and then pushed a one-line
		   row to 42.8px at the ramp's ceiling, over the row above a frame
		   of about 1430px (#1945). Derived from `1lh`, one line plus the
		   padding and the rule IS the row at every width and at every
		   `--row-height` (40px, or 44px where an input can tap), and a
		   cell of more lines keeps that same gap from the rule above and
		   below it (about 7px at the floor, 5px at the ceiling). Rounded
		   down to a whole pixel, the hairline's own unit: Chromium lays a
		   box out in 1/64px steps and rounds the half up, and the 5.1px
		   the ceiling leaves came out at 5.109px, a 40.02px row.

		   No counterpart for the record view's `<dd>`: it carries
		   `padding-block` for EVERY cell already (below) and sets no
		   height at all, so a rule there would override nothing. */
		td.content {
			padding-block: round(
				down,
				calc((var(--row-height) - var(--border-thin) - 1lh) / 2),
				var(--border-thin)
			);
		}

		/* A content cell that holds a link or a button drops that padding
		   (#1843). The target takes `--row-height` as its floor, so the
		   padding would make the row taller than its targets, and the
		   target's own box already keeps its words off the row rule. The
		   Clients list's "Blocked email addresses" link is this case. */
		td.content:has(:global(a), :global(button)) {
			padding-block: 0;
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
		   Email address, Roles, Employment type, Works from, plus its Actions
		   column of three buttons) -- the widest DataTable built today --
		   needs 780px, not 768px, to stop overflowing
		   /style-guide/data-table's own demo of that exact shape. 48.75rem
		   is that fixed point, measured in CI's own Linux/Chromium, the
		   one named environment a floor's minimality is judged against
		   (GLOSSARY.md's Content floor entry), with no margin added beyond
		   it. It is the frame's own inline size that is measured, never
		   the viewport (ADR-0024). A future table wider than this floor
		   moves it.

		   Re-measured 2026-10-25 (#1943), when the type floors moved to
		   16px (ADR-0060): the same demo needed 876px at 868px in CI's
		   Linux/Chromium. 55rem (880px) is the next step that holds there,
		   with no margin added beyond it.

		   Not raised by #1945, measured 2026-10-25 on every route's
		   fixture on the drag surface. At the 16px floors a table whose
		   cells wrap to more lines than at the old floors shows its records
		   instead, the founder's answer on #1945. Swept from 880px to 2560px
		   at both type ramps, a table's cells stop wrapping more at the
		   width its own natural (max-content) width fits, and no sooner:
		   that width is the table's content floor. It runs from under 880px
		   (the Billing screen, the portal's tables, a Client's first table)
		   to 2988px in CI (the Clients list), so one floor here would have to
		   be 2988px and would put every table that fits into its records on
		   every screen. So this floor stays the least any table needs, and
		   a route whose table needs more sets its own floor on these two
		   properties from its own containment context: below its floor,
		   `--data-table-table-display: none` and
		   `--data-table-records-display: block`; at it, `table` and
		   `none`, or nothing, which is these defaults. Each such floor is
		   checked against its route's own fixture by
		   `route-continuum.svelte.spec.ts`.

		   Since #1981 a route's floor is measured on its fixture's typical
		   pilot screen, not its hostile one -- the founder's answer of
		   2026-10-25: a very long name or address wraps one more line, and
		   it does not change the layout. The 2988px above was the hostile
		   Clients list's; each route's own comment has its typical
		   number. This floor is measured on the style-guide demo and stays
		   as it is.

		   Since #2063 a route can set a floor LOWER than this one, with
		   the same two properties: the base rules above and below read
		   them too, so a route that sets `table` and `none` from its own
		   floor shows its table under 55rem. The hub's activity feed is
		   the first: three short columns, in a primary column of about
		   840px at a 1280px window. */
		@container data-table (min-width: 55rem) {
			.table-view {
				display: var(--data-table-table-display, table);
			}

			.record-view {
				display: var(--data-table-records-display, none);
			}
		}

		/* One `SummaryList` per record (#508, ADR-0024, #2021) rather than
		   a mangled <table>, which strips table semantics in Safari and
		   Firefox. The list owns what this view used to write for itself:
		   the shrinkable value track (#1851, a row action's `<select>`;
		   #542, a pasted URL), and one rule per row.

		   Each record is the brief's card (#2022, Law of Common Region: a
		   record genuinely is a unit): `surface-bright` inside a one-pixel
		   `outline-variant` edge, the Engagement summary card's own
		   (`SummaryList`'s `isBordered`) drawn around the title and the
		   actions too. The measure that capped the list caps the card, so
		   a card at 1440px is a reading width, not the window's (#1981).
		   Before this, the gap between two records was the same step as
		   the gap inside one, and on Schedule at 320px the eye could not
		   find where one Visit ended. */
		.record {
			display: flex;
			flex-direction: column;
			gap: var(--space-2);
			max-inline-size: var(--measure);
			padding: var(--space-4);
			border: var(--border-thin) solid var(--color-outline-variant);
			border-radius: var(--radius);
			background-color: var(--color-surface-bright);
		}

		/* Records are a group from the next, so the group step (#2020):
		   28px at the floor, never the field step inside one. */
		.record + .record {
			margin-block-start: var(--space-group);
		}

		/* The card's title: the subheading step, the brief's "card and
		   group titles". `anywhere` because a first column can be a
		   Practice's own words, a pasted URL included (#530). */
		.record-title {
			min-inline-size: 0;
			font-size: var(--text-subheading-size);
			font-weight: var(--text-subheading-weight);
			line-height: var(--text-subheading-leading);
			letter-spacing: var(--text-subheading-tracking);
			overflow-wrap: anywhere;
		}

		/* The last row's hairline is the card's edge's job. */
		.record :global(dl > .row:last-child) {
			border-block-end: none;
		}

		.record-actions {
			padding-block-start: var(--space-2);
		}

		/* A block, so a numeric value can sit at its column's end. */
		.record-view .cell {
			display: block;
		}

		.record-view .cell.numeric {
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
		.record-view .cell.meta {
			font-size: var(--text-meta-size);
			font-weight: var(--text-meta-weight);
			line-height: var(--text-meta-leading);
			letter-spacing: var(--text-meta-tracking);
			font-variant-numeric: tabular-nums;
		}

		.record-view .cell.muted {
			color: var(--color-on-surface-muted);
		}
	}
</style>
