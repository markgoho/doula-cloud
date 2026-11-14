<script lang="ts">
	/*
	 * The bar that narrows a list (#2026, the #2001 audit's root cause 13).
	 * Schedule and On call each built this form by hand, and the two copies
	 * had drifted: a different legend, a different button label, and the
	 * same fieldset, flex row and summary sentence written twice. Two
	 * consumers that should change together is ADR-0018's bar for a
	 * molecule. Named by structure, not by look.
	 *
	 * GOV.UK, NHS.UK and ONS have no filter component, so this is rule-5
	 * original work (its row in docs/design/govuk-alignment.md); the MOJ
	 * pattern library's filter is the nearest published shape.
	 *
	 * A real `<form method="get">`: Enter submits it, the browser's own
	 * controls do the entry, and the fields are named so the form still
	 * reaches the right URL with no script running at all. `onSubmit` is
	 * the route's upgrade to a client-side navigation, and it owns
	 * `preventDefault()` (Rule of Least Power). That GET submission is why
	 * this is not `StackedForm`, whose fields stack and whose submit is
	 * the page's own.
	 *
	 * The fields sit at their own content width in a cluster that wraps,
	 * never stretched across the column: a date is ten characters, and at
	 * 1440px a stretched one was 600px wide with Apply 1,200px from the
	 * first field. A date control sizes to itself (#1216); a text field or
	 * a select states its `width` on its `LabeledField`. At 320px the
	 * cluster puts one field on each row, with no width query (ADR-0024).
	 *
	 * The result count is the bar's own status line (`role="status"`, a
	 * polite live region), so a new range's result is announced and the
	 * count reads as part of the bar rather than a loose sentence under
	 * it. When the list holds nothing, its `EmptyState` says so on screen,
	 * and the line stays in the document, visually hidden, so it is still
	 * announced (#2015). It is hidden with a class rather than removed, so
	 * the live region exists before its text changes.
	 *
	 * Apply follows the fields, the applied filters' tags follow Apply, and
	 * "Clear filters" follows the tags, each wrapping on its own: a tag sits
	 * in the room already empty beside Apply and takes a line under it only
	 * where that room runs out, and nothing is held empty for it. Apply comes
	 * first, so a tag or the link never moves it. A bar with switches gives
	 * them a row above the fields, and a switch the route does not draw right
	 * now but marks `data-holds-space` keeps its box. The count has a row of
	 * its own under the controls and keeps its height while it is empty.
	 *
	 * With `foldsFieldsOnly`, `children` stay in view at every width and only
	 * `folded` folds, and "Clear filters" is drawn only while it is given. The
	 * bar that folds whole keeps the link's box while no filter is on, drawn
	 * hidden (`visibility: hidden`, out of the tab order and the accessibility
	 * tree).
	 *
	 * In a narrow column the controls fold behind a native `<details>`
	 * ("Filters", or "Filters: Only my Clients" / "Filters: 2 on" given
	 * `activeLabels`; it opens only when the person opens it) so a phone's
	 * first screen holds the list, as the MOJ filter does. A "Skip to
	 * results" link is the bar's first stop, drawn only while focused, and
	 * moves focus to the status line.
	 * The status line stays outside it. In a wide column, one that holds
	 * the fullest field row on a line, the summary is not drawn and the
	 * controls show in full. The switch is a query on the bar's own
	 * container (floor.svelte.spec.ts holds its criterion), and the closed
	 * content is shown by undoing the user agent's own
	 * `content-visibility: hidden` on `::details-content`, so it is in the
	 * accessibility tree only while shown. No script.
	 *
	 * Every bar has Apply: a date or a name is typed a character at a time,
	 * and a list that reloaded on each one would move under the person
	 * typing it, so a bar that holds one such field applies every control
	 * with it (GOV.UK's filter pattern). The bar of switches that acted
	 * the moment they changed (#2068) went when the Clients list gained a
	 * Name and a Doula field.
	 */
	import type { Snippet } from 'svelte';
	import Button from '#lib/components/atoms/Button.svelte';
	import Link from '#lib/components/atoms/Link.svelte';
	import Select from '#lib/components/atoms/Select.svelte';
	import LabeledField from '#lib/components/molecules/LabeledField.svelte';

	interface Properties {
		/**
		 * The list's own URL: where the form goes with no script running.
		 */
		action: string;
		/**
		 * The route's client-side navigation. It owns `preventDefault()`.
		 */
		onSubmit?: (event: SubmitEvent) => void;
		/**
		 * What the bar does, in a few words ("Narrow the schedule").
		 */
		legend: string;
		/**
		 * The result, in words ("Showing 2 scheduled visits.").
		 */
		status: string;
		/**
		 * True when the list under the bar holds nothing: its empty state
		 * says so on screen, and the status line is then for a screen
		 * reader only.
		 */
		empty?: boolean;
		/**
		 * Where "Clear filters" goes: the list with nothing applied. Given
		 * only while a filter is on. The link follows the tags, so Apply sits
		 * in the same place whether or not it is there; beside exactly one tag
		 * it is not drawn (`appliedCount`).
		 */
		clearHref?: string;
		/**
		 * The filters that are on, by name, for the summary a narrow column
		 * draws: "Filters: Only my Clients" for one, "Filters: 2 on" for
		 * more. The summary stays closed until the person opens it, so a
		 * phone's first screen holds the list.
		 */
		activeLabels?: string[];
		/**
		 * The id of the result line, so a table can be described by it
		 * (`DataTable`'s `describedBy`) and the skip link can land on it.
		 * Generated when not given.
		 */
		statusId?: string;
		/**
		 * With `foldsFieldsOnly`: the longest wording the result line can say, which the
		 * bar draws unseen under it. The line then reserves what that wording takes at the
		 * bar's width (one line where it fits on one, two at most), so the list does not
		 * move between results. Without it the line reserves two lines.
		 */
		statusRuler?: string;
		/**
		 * One more plain line of meta text directly under the result line, said only
		 * when there is something to say (the Schedule's time zone, when it is not the
		 * reader's). Not a control and not in the status region; when it is absent
		 * nothing is drawn and nothing is held.
		 */
		statusNote?: string;
		/**
		 * The filters that are on, as GOV.UK's "selected filters": one `<li>` for
		 * each, a removable tag, drawn after Apply on its row, in the room the row
		 * leaves beside it. Given only while a filter is on.
		 */
		applied?: Snippet;
		/**
		 * How many tags `applied` draws. Beside exactly one, its × does what
		 * "Clear filters" would, so the link is drawn only beside two or more, or
		 * while a filter is on that draws no tag (`clearHref` with none).
		 */
		appliedCount?: number;
		/**
		 * Switches that sit in a row of their own above the fields, so a
		 * switch drawn or hidden never moves a field or Apply. A group of them
		 * is a `<fieldset>` (or a `RadioGroup isInline` with its `legend`) whose
		 * legend is drawn on the row, before its boxes.
		 */
		switches?: Snippet;
		/**
		 * Keep the switches and Apply in view at every width and fold only
		 * the fields behind the summary, for a bar whose switches are the
		 * filters most often wanted. The summary then names the fields that
		 * are on (`activeLabels`: only those).
		 */
		foldsFieldsOnly?: boolean;
		/**
		 * Whether a newer result is on its way. Apply shows its loading state,
		 * so a press is answered at once and cannot be sent twice.
		 */
		isBusy?: boolean;
		/**
		 * With `foldsFieldsOnly`: the fields that fold behind the summary in a
		 * narrow column, such as a rarely wanted select. `children` then stays
		 * in view at every width, for the field a reader reaches for most (a
		 * name to find a record by).
		 */
		folded?: Snippet;
		/**
		 * The order a reader can choose for the rows (#2869): a labeled `Order`
		 * select, submitted with Apply as the form field `order`, and the last
		 * field before Apply at every width. It never folds, and a bar that
		 * folds whole draws it inside the fold as it does every field. `value`
		 * is the order the select shows, the route's draft, which it seeds from the
		 * address and puts back in step after a move; the empty value is the list's own
		 * order and is left out of the address. `onChange` reports a pick
		 * before Apply, for a route that builds its address from its own state.
		 */
		order?: { options: readonly { value: string; label: string }[]; value: string; onChange?: (value: string) => void };
		/**
		 * The fields, each a `LabeledField` with a named control. A bar of
		 * switches alone has none.
		 */
		children?: Snippet;
	}

	const uid = $props.id();
	let {
		action,
		onSubmit,
		legend,
		status,
		empty = false,
		clearHref,
		activeLabels = [],
		statusId = `filter-bar-status-${uid}`,
		statusRuler,
		statusNote,
		applied,
		appliedCount = 0,
		switches,
		foldsFieldsOnly = false,
		isBusy = false,
		folded,
		order,
		children
	}: Properties = $props();

	// What the select holds: the order the address names, until a pick.
	let pickedOrder = $derived(order?.value);

	const summary = $derived.by(() => {
		const noun = foldsFieldsOnly ? 'More filters' : 'Filters';
		if (activeLabels.length === 0) return noun;
		if (activeLabels.length === 1) return `${noun}: ${activeLabels[0]}`;
		return `${noun}: ${activeLabels.length} on`;
	});

	const clearTo = $derived(appliedCount === 1 ? undefined : clearHref);

	let statusLine = $state<HTMLParagraphElement>();

	/**
	 * Moves focus to the result line, so a keyboard user who has just applied
	 * a filter lands on its result and not at the top of the page. The route
	 * calls it; it takes tabindex="-1" so it never joins the tab order.
	 */
	export function focusStatus() {
		statusLine!.focus();
	}

	// Closed until the person opens it, then the element's own, so a rerender
	// for another reason never closes what the person just opened
	// (`atoms/Details.svelte`).
	let isOpen = $state(false);
</script>

{#snippet selected(isHeld: boolean)}
	{#if applied || clearTo || isHeld}
		<div class="selected">
			{#if applied}
				<ul class="applied" aria-label="Applied filters">
					{@render applied()}
				</ul>
			{/if}
			<!-- With `isHeld` (the bar that folds whole), always a box, drawn hidden while
			     no filter is on: out of sight, out of the tab order and out of the
			     accessibility tree, holding only its space. -->
			{#if clearTo || isHeld}
				<span class="clear" class:reserve={!clearTo} inert={!clearTo}>
					<Link href={clearTo ?? action} label="Clear filters" />
				</span>
			{/if}
		</div>
	{/if}
{/snippet}

{#snippet orderField()}
	{#if order}
		<div class="order">
			<LabeledField label="Order" width={20}>
				{#snippet children({ id, describedBy, invalid, width })}
					<Select
						{id}
						{describedBy}
						{invalid}
						{width}
						name="order"
						options={order.options}
						bind:value={pickedOrder}
						onChange={order.onChange}
					/>
				{/snippet}
			</LabeledField>
		</div>
	{/if}
{/snippet}

{#snippet result()}
	<div class="count" class:ruled={statusRuler !== undefined}>
		<p id={statusId} role="status" class="status" class:visually-hidden={empty} tabindex="-1" bind:this={statusLine}>{status}</p>
		{#if statusRuler !== undefined}
			<!-- The longest wording, unseen, out of the tab order and the accessibility tree. -->
			<span class="status ruler" aria-hidden="true" inert>{statusRuler}</span>
		{/if}
	</div>
	{#if statusNote}
		<p class="status note">{statusNote}</p>
	{/if}
{/snippet}

<!-- stacked-form:ignore: #2026 -- the one GET narrowing form. Its fields sit side by side and wrap on their own, and `StackedForm` would both stack them and take away the GET submission this form is built on. -->
<form method="get" {action} onsubmit={onSubmit} class:dense={foldsFieldsOnly}>
	<!-- The first stop in the bar, drawn only while focused: a keyboard user
	     crosses the controls only when she means to. -->
	<Link
		href={`#${statusId}`}
		label="Skip to results"
		variant="skip"
		onClick={(event) => {
			event.preventDefault();
			focusStatus();
		}}
	/>
	{#if foldsFieldsOnly}
		<!-- Switches and Apply stay in view at any width; only the fields fold,
		     behind a summary that names the ones that are on. In a wide column
		     the fold is not drawn and the fields sit beside Apply on one row. -->
		<fieldset>
			<legend class="quiet">{legend}</legend>
			<stack-l class="controls" space="var(--space-1)">
				{#if switches}
					<cluster-l class="switches" space="var(--space-related)" align="center">
						{@render switches()}
					</cluster-l>
				{/if}
				<cluster-l class="fields" space="var(--space-related)" align="flex-end">
					{@render children?.()}
					{#if folded}
						<details class="fold" open={isOpen} ontoggle={(event) => (isOpen = event.currentTarget.open)}>
							<summary class="toggle">{summary}</summary>
							{@render folded()}
						</details>
					{/if}
					<!-- The tags follow Apply in the room the row has left beside it, and
					     wrap under it only where that room runs out: nothing is held for
					     them, and Apply comes first, so a tag never moves it. -->
					{@render orderField()}
					<Button type="submit" label="Apply" variant="secondary" loading={isBusy} />
					{@render selected(false)}
				</cluster-l>
			</stack-l>
		</fieldset>
	{:else}
	<details open={isOpen} ontoggle={(event) => (isOpen = event.currentTarget.open)}>
		<summary class="toggle">{summary}</summary>
		<fieldset>
			<legend>{legend}</legend>
			<stack-l space="var(--space-related)">
				{#if switches}
					<cluster-l space="var(--space-related)" align="center">
						{@render switches()}
					</cluster-l>
				{/if}
				<cluster-l class="fields" space="var(--space-related)" align="flex-end">
					{@render children?.()}
					{@render orderField()}
					<Button type="submit" label="Apply" variant="secondary" loading={isBusy} />
					{@render selected(true)}
				</cluster-l>
			</stack-l>
		</fieldset>
	</details>
	{/if}
	<!-- On the bar's left edge, in a row of its own directly above the list, at every width,
	     and one line tall while empty: what is applied never moves Apply. -->
	{@render result()}
</form>

<style>
	@layer components {
		/* The controls from the result line under them: the group step, since
		   the result line belongs to the list it counts and sits tight to
		   it. Inside the controls, the related step (#2020). */
		form {
			display: flex;
			flex-direction: column;
			gap: var(--space-group);
			position: relative;
		}

		/* The skip link sits in the gap above the bar (the group step every list
		   screen leaves between its title row and the bar), fitted to it, so
		   focusing it moves nothing and it covers no switch, label or field. At rest
		   it is the same box, clipped to nothing. The gap is the least 28px, so the
		   link is at least 26px tall, over the 24px a target needs. */
		form > :global(a.skip) {
			inset-block-start: calc(var(--border-thin) - var(--space-group));
			box-sizing: border-box;
			display: inline-flex;
			align-items: center;
			block-size: calc(var(--space-group) - 2 * var(--border-thin));
			min-block-size: 0;
			padding-block: 0;
		}

		fieldset {
			margin: 0;
			padding: 0;
			border: 0;
			min-inline-size: 0;
		}

		/* Quiet: the size and weight of a field's own label, the shape
		   `DateFields` gives its legend, never bold body text, which read
		   as a stray sentence above the fields. `--space-label` to the
		   fields, the step from a question to its control. */
		legend {
			padding: 0;
			margin-block-end: var(--space-label);
			font-size: var(--text-label-size);
			font-weight: var(--font-weight-medium);
			color: var(--color-on-surface-variant);
		}

		/* A field may shrink below its own width in a column narrower than
		   it, rather than push the page sideways. */
		cluster-l > :global(*) {
			min-inline-size: 0;
		}

		/* The bar is its own container, so the filters collapse behind one
		   summary by the room the bar has, not the window's (ADR-0024). */
		form {
			container: filter-bar / inline-size;
		}

		/* The base size re-resolved against the bar (#544): a `cqi` resolves
		   against the nearest ancestor container, which the bar cannot be
		   for itself. */
		form > * {
			font-size: var(--text-body-size);
		}

		.toggle {
			cursor: pointer;
			min-block-size: var(--space-6);
			padding-block: var(--space-1);
			font-size: var(--text-label-size);
			font-weight: var(--font-weight-medium);
			color: var(--color-on-surface-variant);
		}

		/* In a column that holds every control on one row each, the
		   controls show in full and the summary is not drawn. A closed
		   <details> hides its content through `::details-content`, so the
		   content is shown by undoing that, and stays out of the
		   accessibility tree whenever it is hidden. */
		/* The folded fields (`foldsFieldsOnly`): their own row in a narrow
		   column, one field under the next. */
		.fold {
			min-inline-size: 0;
		}

		/* Closed, the summary shares a row with Apply; open, the fields take
		   the row to themselves. */
		.fold[open] {
			flex-basis: 100%;
		}

		/* A group of switches (the Clients list's "Show only" boxes, the Invoices
		   list's "Show" radios) draws its legend on the boxes' own row: floated,
		   an ordinary box beside them, where a rendered legend takes a line above.
		   The boxes wrap among themselves beside it. */
		.switches > :global(fieldset) {
			margin: 0;
			padding: 0;
			border: 0;
			min-inline-size: 0;
		}

		.switches > :global(fieldset) > :global(legend) {
			float: inline-start;
			display: flex;
			align-items: center;
			min-block-size: var(--hit-target-min);
			padding: 0;
			margin: 0;
			margin-inline-end: var(--space-related);
			font-size: var(--text-label-size);
			font-weight: var(--font-weight-medium);
			color: var(--color-on-surface-variant);
		}

		/* The switches are stacked at a phone's width; each keeps its own
		   target, so the rows sit close. Every choice control in the row is
		   GOV.UK's small size, its pattern for a filter beside results: a 24px
		   box or circle and a tighter gap to its words, read by `Checkbox`,
		   `RadioGroup` and `LabeledField` through the choice tokens, so no route
		   picks a size. */
		.switches {
			--choice-size: calc(var(--control-height) * 0.6);
			--choice-gap: var(--space-2);
			row-gap: var(--space-1);
		}

		/* A small box keeps the full target: its row and its label stand the
		   target tall, the box centered in it. */
		.switches :global(.inline-row) {
			grid-template-rows: minmax(var(--hit-target-min), auto);
			align-items: center;
		}

		.switches :global(.inline-row > label) {
			align-self: stretch;
			display: flex;
			align-items: center;
			padding-block: 0;
		}

		/* Where the switches are stacked, a switch that is not drawn leaves no row. */
		.switches > :global([data-holds-space]) {
			display: none;
		}

		/* Apply fills a narrow container as a screen's own action does
		   (Button.svelte); here it is one control among several in a bar, so
		   it keeps the width of its words. */
		.dense .fields > :global(button.fills-narrow) {
			inline-size: auto;
		}

		/* The tags and "Clear filters" share Apply's line only where all of them
		   fit on it, so they never make that line taller and Apply stays put;
		   otherwise they take the line under it, wrapping among themselves. */
		.selected,
		.applied {
			display: flex;
			flex-wrap: wrap;
			align-items: center;
			gap: var(--space-1) var(--space-2);
		}

		/* The tags and "Clear filters" are as tall as Apply (a medium button: a line,
		   its block padding and border, or the target floor) and centered in that
		   height, so on Apply's line they read as one row with it. */
		.dense :is(.applied, .clear) {
			display: flex;
			align-items: center;
			min-block-size: max(var(--hit-target-min), calc(1lh + 2 * (var(--space-2) + var(--border-thin))));
		}

		.fold::details-content {
			display: flex;
			flex-direction: column;
			gap: var(--space-related);
			padding-block-start: var(--space-related);
		}

		/* In a narrow column the group is named for a screen reader and not
		   drawn: its switches say what they do, and a phone's first screen
		   is the list (a legend line there is the height of a Client).
		   `foldsFieldsOnly` only; the other bar draws its legend folded. */
		legend.quiet {
			position: absolute;
			inline-size: var(--border-thin);
			block-size: var(--border-thin);
			margin: 0;
			overflow: hidden;
			clip-path: inset(50%);
			white-space: nowrap;
		}

		/* The count line sits close to the controls it reports on. */
		.dense {
			gap: var(--space-2);
		}

		.applied {
			margin: 0;
			padding: 0;
			list-style: none;
		}

		/* A tag is a target the height of a table row's (`--row-target-min`, 44px at a
		   coarse pointer), not the 44px of a link on its own: the list under it is a
		   table of such rows, and a taller tag row costs it a row. */
		.applied :global(a.chip) {
			min-block-size: var(--row-target-min);
			padding-block: var(--space-1);
		}

		/* The count and its ruler share one box, so it is as tall as the taller of the two. */
		.count {
			display: grid;
		}

		.count > * {
			grid-area: 1 / 1;
			align-self: start;
		}

		/* Two lines tall whatever it says, so applying a filter never moves the list. */
		.dense > .count {
			min-block-size: 2lh;
		}

		/* With a ruler, the longest wording the count can say, drawn unseen in the same
		   box: the box takes what it takes at this width, up to two lines, with no width
		   query. */
		.dense > .count.ruled {
			min-block-size: 1lh;
		}

		.ruler {
			visibility: hidden;
			max-block-size: 2lh;
			overflow: hidden;
		}

		/* The link's box when no filter is on: hidden, not removed, so its space
		   is held. `visibility: hidden` also takes it out of the tab order and
		   the accessibility tree. */
		.reserve {
			visibility: hidden;
		}

		@container filter-bar (min-width: 42rem) {
			/* A switch the route does not draw right now but marks
			   `data-holds-space` keeps its box in the row, unseen and out of reach
			   (the route also makes it `inert`), so the switches and everything
			   under them stay put. A real box: `display: contents` holds no space. */
			.switches > :global([data-holds-space]) {
				display: block;
				visibility: hidden;
			}

			/* The fields join the row Apply is on. */
			.fold {
				display: contents;
			}

			.fold::details-content {
				display: contents;
			}

			.toggle {
				display: none;
			}

			details::details-content {
				content-visibility: visible;
			}
		}

		/* A status line, not prose: it wraps at the bar's whole width, so the longest
		   wording takes one line wherever the bar can hold it, and every list holds the same. */
		.status {
			margin: 0;
			/* One line's height even while empty, so the list does not move when
			   the count arrives. */
			min-block-size: 1lh;
			color: var(--color-on-surface-variant);
			font-size: var(--text-meta-size);
			line-height: var(--text-meta-leading);
		}

		/* The line's own box hugs its sentence, so its focus ring does; the
		   ruler keeps the bar's width, which is where the sentence wraps. */
		p.status:not(.visually-hidden) {
			inline-size: fit-content;
		}

		/* Focus lands here after a filter is applied (`focusStatus`). */
		.status:focus-visible {
			outline: var(--focus-ring-width) solid var(--color-primary);
			outline-offset: var(--focus-ring-offset);
		}
	}
</style>
