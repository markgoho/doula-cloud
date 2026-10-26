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
		/**
		 * A word or two marking the row, drawn as a neutral `Badge` on a
		 * line under the label -- "No longer collected" on a Client's answer
		 * to a question the Practice stopped asking (#2008).
		 */
		tag?: string;
		/**
		 * Why a row has no action, said at the row's end on the same line
		 * as its tag -- "Kept, not collected" (#2008). Drawn only on a row
		 * that draws no action, so a row never carries both.
		 */
		status?: string;
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
	 * The one key/value list (#2021, decided on #2001): every record's
	 * facts, every check-answers row, each record of `DataTable`'s record
	 * view, each section of a Birth Plan and an Offer's facts render
	 * through this, and nothing else writes a `<dl>`. `DescriptionList`,
	 * the action-less twin it replaced, drew its rule twice per row (once
	 * under the key, once under the value, with the column gap between)
	 * and capped the value rather than the list, so four lists on one page
	 * ended at four right edges.
	 *
	 * The action is a Snippet, not an href, because the two consumers
	 * act differently: `CheckAnswers`' Change is a link back to the
	 * question, and the Engagement's Change opens an in-place form. The
	 * Snippet gets the id of the visually-hidden "what it changes" text,
	 * so whatever it draws is described by it.
	 */
	import type { Snippet } from 'svelte';
	import Badge from '../atoms/Badge.svelte';

	interface Properties {
		rows: Row[];
		/**
		 * Draws a row's value in place of its `value` text: a link, a
		 * `<time>`, a control -- whatever the caller's value is. A row's
		 * `value` text still names it to the caller (`DataTable` keeps its
		 * accessor's string there).
		 */
		value?: Snippet<[Row, number]>;
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
		/**
		 * Every row's key above its value at any width, as a Birth Plan's
		 * questions are: a question is a sentence, and a sentence in a key
		 * column wraps to a ribbon. Without it a row stacks only where the
		 * key and the value cannot share a line.
		 */
		isStacked?: boolean;
	}

	let { rows, value, action, detail, isBordered = false, isStacked = false }: Properties = $props();

	const listId = $props.id();
</script>

<dl class:bordered={isBordered} class:stacked={isStacked}>
	<!--
		Keyed on index, not on the label (#464): a record can show the same
		label twice -- two phone numbers, two Practice-defined fields a
		Practice named alike -- and a duplicate key collapses both rows into
		one silently.
	-->
	{#each rows as row, index (index)}
		<div class="row">
			<dt>{row.label}</dt>
			<dd class="value">
				{#if value}{@render value(row, index)}{:else}{row.value}{/if}
			</dd>
			{#if action && row.changes}
				<dd class="action">
					{@render action(row, `${listId}-${index}-changes`)}
					<span class="visually-hidden" id="{listId}-{index}-changes">{row.changes}</span>
				</dd>
			{/if}
			<!--
				#2008: the tag and the status share one line under the row, the
				tag under the label and the status at the row's end. Under the
				label itself, a 30% key column wrapped the badge inside its own
				pill; on the label's line, the status pushed past the value's
				20rem and dropped to a line of its own anyway.
			-->
			{#if row.tag || (row.status && !(action && row.changes))}
				<dd class="note">
					{#if row.tag}<Badge label={row.tag} variant="neutral" />{/if}
					{#if row.status && !(action && row.changes)}<span class="status">{row.status}</span>{/if}
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
		 * The measure caps the list (#2021), not the value: a value capped
		 * alone put the row's end at key width plus measure, so each list
		 * ended at its own right edge. `--measure` is the repo's one reading
		 * width and font-relative; at 320px it is inert.
		 */
		dl {
			max-inline-size: var(--measure);
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

		/*
		 * One hairline per row, across the key and the value (#2021), GOV.UK's
		 * grouping cue and the brief's Law of Uniform Connectedness.
		 *
		 * Each row is a wrapping flex line whose key takes a share of the
		 * list, not of its own words, so every key, value and Change sits
		 * in the same column as the row above, and in every other list of
		 * the same width: a Client hub's four lists and a check page's
		 * sections line their values up down the page. `fit-content(40%)`,
		 * the grid track this replaced, sized each list's key to its own
		 * longest label, so four lists put their values at four lines.
		 *
		 * The key is 30% of the row, GOV.UK's own key column. It names no
		 * width (ADR-0024); a key longer than its share wraps inside it, a
		 * pasted URL included (#725). The value asks for 20rem beside it --
		 * a date, a name, a short address on one line -- and where the row
		 * cannot give it that much, the value moves to its own line under
		 * the key: GOV.UK's Summary list on a narrow screen, reached by the
		 * row's own width, never the window's, and with no query at all
		 * (at 30rem, a 30% key is 9rem, where an ordinary label such as
		 * "Preferred name" starts to wrap beside its value). The break
		 * reads only the key and the value, so every row of a list stacks
		 * at the same width, with a Change or without.
		 *
		 * The Change follows the value and sits at the row's end: on the
		 * value's line where there is room, on a line of its own under it
		 * where there is not. `align-items: baseline` puts the label, the
		 * value and the Change on one text line, though the Change is a
		 * 44px target and the others are not.
		 *
		 * (#564 dropped a grid of `repeat(auto-fit, minmax(min-content,
		 * 1fr))` because Chromium drops it whole; this is a flex line with
		 * definite bases, which does not meet that restriction.)
		 */
		.row {
			display: flex;
			flex-wrap: wrap;
			align-items: baseline;
			gap: var(--space-1) var(--space-4);
			padding-block: var(--space-3);
			border-block-end: var(--border-thin) solid var(--color-outline-variant);
		}

		/*
		 * The key grows only when it is alone on its line, stacked: the
		 * value's grow factor is so much larger that, sharing a line, the
		 * key keeps its 30% to well under a pixel.
		 */
		dt {
			flex: 1 0 30%;
			min-inline-size: 0;
			margin: 0;
			font-weight: var(--font-weight-medium);
			color: var(--color-on-surface-variant);
			overflow-wrap: anywhere;
		}

		dd {
			margin: 0;
			color: var(--color-on-surface);
		}

		/* `min-inline-size: 0` and `anywhere`, so an unbreakable value (a
		   pasted URL, #530) shrinks rather than widening the page. */
		.value {
			flex: 100000 1 20rem;
			min-inline-size: 0;
			overflow-wrap: anywhere;
		}

		.action {
			flex: none;
			margin-inline-start: auto;
		}

		.row > .note {
			display: flex;
			flex-basis: 100%;
			flex-wrap: wrap;
			align-items: center;
			gap: var(--space-1) var(--space-4);
		}

		/* What stands where a Change would be is a remark, not a control. */
		.status {
			margin-inline-start: auto;
			font-size: var(--text-meta-size);
			line-height: var(--text-meta-leading);
			color: var(--color-on-surface-muted);
		}

		/* Every key above its value at every width (`isStacked`). */
		.stacked .value {
			flex-basis: 100%;
		}

		/* The in-place form takes the row's whole width, on a line of its own. */
		.row > .detail {
			flex-basis: 100%;
			padding-block-start: var(--space-3);
		}
	}
</style>
