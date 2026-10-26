<script module lang="ts">
	export interface SummaryListRow {
		label: string;
		value: string;
		/**
		 * What the row's action changes, in words -- "given name", "care
		 * phase". Its presence is what says the reader may change this
		 * row: a row without it draws no action cell. Every row's action
		 * reads the same word, "Change", so without this a screen reader
		 * user listing the page's controls hears "Change" once per row;
		 * GOV.UK's own answer is visually-hidden text after it, which is
		 * what this becomes.
		 */
		changes?: string;
		/**
		 * Whether the row's `detail` is drawn under it -- the in-place form
		 * a Change opens on a record page (#1998). The caller owns which
		 * row is open, because the caller owns what the form does.
		 */
		isDetailShown?: boolean;
	}
</script>

<script lang="ts" generics="Row extends SummaryListRow">
	/*
	 * GOV.UK's Summary list with row actions: a label, a value and, on a
	 * row the reader may change, a "Change". Extracted from `CheckAnswers`
	 * on #1998, when the Engagement page became the second page that wants
	 * it -- ADR-0058's bar, two consumers that should change together. A
	 * change to how a Change row reads should reach both the same way.
	 *
	 * Not `DescriptionList`: that molecule is a record's facts with no
	 * action column, laid out as a two-track grid, and it has many
	 * consumers under the continuum gate. Whether the two become one is
	 * the pre-launch inventory's question (#1908), not this ticket's.
	 *
	 * The action is a Snippet, not an href, because the two consumers
	 * act differently: `CheckAnswers`' Change is a link back to the
	 * question, and the Engagement's Change opens an in-place form. The
	 * Snippet gets the id of the visually-hidden "what it changes" text,
	 * so whatever it draws is described by it.
	 */
	import type { Snippet } from 'svelte';

	interface Properties {
		rows: Row[];
		action?: Snippet<[Row, string]>;
		/**
		 * Drawn under a row whose `isDetailShown` is set, across the
		 * row's whole width.
		 */
		detail?: Snippet<[Row]>;
		/**
		 * A bordered card around the list, as the Engagement's drawing has
		 * it. `CheckAnswers` leaves it off: its sections are already set
		 * apart by their own headings.
		 */
		isBordered?: boolean;
	}

	let { rows, action, detail, isBordered = false }: Properties = $props();

	const listId = $props.id();
</script>

<dl class:bordered={isBordered}>
	<!--
		Keyed on index, not on the label: two rows may share a label, the
		same defect #464 fixed in `DescriptionList`.
	-->
	{#each rows as row, index (index)}
		<div class="row">
			<dt>{row.label}</dt>
			<dd class="value" class:spans={!(action && row.changes)}>{row.value}</dd>
			{#if action && row.changes}
				<dd class="action">
					{@render action(row, `${listId}-${index}-changes`)}
					<span class="visually-hidden" id="{listId}-{index}-changes">{row.changes}</span>
				</dd>
			{/if}
			{#if detail && row.isDetailShown}
				<dd class="detail">{@render detail(row)}</dd>
			{/if}
		</div>
	{/each}
</dl>

<style>
	@layer components {
		/*
		 * One grid for the whole list, and each row a subgrid of it (#1998),
		 * so every label, value and Change sits in the same column as the
		 * row above -- what makes a run of rows read as one list, and what
		 * the drawing shows. A flex row per row, which this replaced, sized
		 * each row's columns to that row's own words, so the values
		 * zig-zagged down the card.
		 *
		 * The tracks are `DescriptionList`'s, plus the action's: the label
		 * takes `fit-content(40%)` because a Practice types some labels and
		 * one may be a pasted URL (#725); the value is `minmax(0, 1fr)`, so
		 * an unbreakable value shrinks rather than widening the page
		 * (#530); and the action is `auto`, the width of "Change". No
		 * width is named, so the list adapts to whatever room it is given
		 * (ADR-0024).
		 */
		dl {
			display: grid;
			grid-template-columns: fit-content(40%) minmax(0, 1fr) auto;
			column-gap: var(--space-4);
			margin: 0;
		}

		/*
		 * The card the Engagement's drawing puts around its summary. The
		 * brief declares a container by an edge rather than a box; the
		 * drawing asks for the box here, and the founder's priority for the
		 * pilot is that the screen matches its drawing.
		 */
		dl.bordered {
			padding-inline: var(--space-4);
			border: var(--border-thin) solid var(--color-outline-variant);
			border-radius: var(--radius);
			background-color: var(--color-surface-bright);
		}

		dl.bordered > .row:last-child {
			border-block-end: none;
		}

		dt {
			margin: 0;
			font-weight: var(--font-weight-medium);
			color: var(--color-on-surface-variant);
			overflow-wrap: anywhere;
		}

		dd {
			margin: 0;
			color: var(--color-on-surface);
		}

		/*
		 * A hairline under each row. `align-items: baseline` puts the
		 * label, the value and the Change on one text line, though the
		 * Change is a 44px target and the others are not.
		 *
		 * (CheckAnswers' rows were flex-wrap before, after a first grid of
		 * `repeat(auto-fit, minmax(min-content, 1fr))` that Chromium drops
		 * whole (#564). This grid has fixed tracks and no auto-fit, so it
		 * does not meet that restriction.)
		 */
		.row {
			display: grid;
			grid-column: 1 / -1;
			grid-template-columns: subgrid;
			align-items: baseline;
			row-gap: var(--space-1);
			padding-block: var(--space-3);
			border-block-end: var(--border-thin) solid var(--color-outline-variant);
		}

		.value {
			grid-column: 2;
			overflow-wrap: anywhere;
		}

		/* A row with no Change gives its value the action's column too, so
		   a narrow list does not wrap a date around an empty cell. */
		.value.spans {
			grid-column: 2 / -1;
		}

		.action {
			grid-column: 3;
			text-align: end;
		}

		/* The in-place form takes the row's whole width, on a line of its own. */
		.row > .detail {
			grid-column: 1 / -1;
			padding-block-start: var(--space-3);
		}
	}
</style>
