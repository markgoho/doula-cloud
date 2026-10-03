<script module lang="ts">
	/**
	 * The id a group's own option carries (`id="{name}-{option.value}"`
	 * below) -- exported so a caller building a `FormError`'s `targetId`
	 * for this group's first option (#1228: `InvoiceSection`'s Method,
	 * `OfferSection`'s Doula, the same trick `endingReasonFieldId` uses on
	 * the Engagement detail page) constructs the same string this
	 * component does, rather than a third hand-rolled copy of the
	 * template literal agreeing with it by coincidence.
	 */
	export function radioFieldId(name: string, value: string): string {
		return `${name}-${value}`;
	}
</script>

<script lang="ts" generics="T extends string">
	interface Option<T> {
		value: T;
		label: string;
		/*
		 * The consequence of choosing this option, in the reader's words --
		 * GOV.UK's radio hint. Added on #464 for the intake duplicate
		 * check (#432): that page offers the Practice's existing Clients as
		 * options, and a name alone cannot tell two Sarahs apart. The
		 * history and what picking her will do belong to the option, not to
		 * the group, so a `label` string could not carry them.
		 */
		description?: string;
	}

	interface Properties<T> {
		/*
		 * Optional, like `FormPage`'s (#425), and for the same reason: a
		 * question page makes the group's name the page's own <h1>
		 * (#464), so the Template already owns the <fieldset> and its
		 * <legend>. A second one here would nest fieldsets and announce
		 * the question twice.
		 */
		legend?: string;
		name?: string;
		/* `readonly`: the group only reads its options, and a caller
		   handing it a shared constant (`EMPLOYMENT_TYPE_LABELS`, #262)
		   should not have to copy the array to pass it. */
		options: readonly Option<T>[];
		value: T;
		onChange: (value: T) => void;
		/*
		 * The refusal for this group, in the reader's words. GOV.UK asks
		 * for the message twice -- once in the error summary at the top of
		 * the page, and again against the control itself -- because a
		 * summary alone leaves a reader who has scrolled past it with
		 * nothing to act on. `LabeledField` already carries an `error` for
		 * a single input; a radio group needs its own, since the message
		 * belongs to the question rather than to any one option. Announced
		 * by role="alert", so it reaches a screen reader whether or not a
		 * legend gave it a <fieldset> to be described by.
		 */
		error?: string;
	}

	const generatedName = $props.id();

	let { legend, name = generatedName, options, value, onChange, error }: Properties<T> = $props();

	const errorId = $derived(`${name}-error`);
</script>

{#snippet errorMessage()}
	{#if error}
		<p id={errorId} class="error" role="alert">{error}</p>
	{/if}
{/snippet}

{#snippet radios()}
	<stack-l space="var(--space-5)">
		<!-- v8 ignore start: only the compiled branch for "was this keyed
		     <div> added/removed from the DOM since the last render" is
		     unreachable here (Svelte's own each-block diffing internals, not
		     app code) -- the loop body itself is exercised by
		     "renders an option for each entry in options" in
		     RadioGroup.svelte.spec.ts -->
		{#each options as option (option.value)}
			<div class="choice">
				<!--
					A two-track grid, not cluster-l (#1596): cluster-l is
					flex-wrap, and a flex row that cannot hold both items drops
					the whole label to a line of its own, under a radio that
					then reads as belonging to nobody. The same defect and the
					same answer as LabeledField's inline orientation (#510).
					Found on the Start work form at 320px, where a Doula's
					double-barreled name is the label.

					The <label> wraps the radio rather than pointing at it
					(#1518): the circle, the gap beside it and the text are
					then one click target, the whole row, with no CSS to
					stretch a hit area over the gap the way GOV.UK has to.
				-->
				<label class="option" class:invalid={error !== undefined}>
					<input
						type="radio"
						id="{name}-{option.value}"
						{name}
						value={option.value}
						checked={option.value === value}
						aria-describedby={option.description ? `${name}-${option.value}-hint` : undefined}
						onchange={() => onChange(option.value)}
					/>
					<span class="text">{option.label}</span>
				</label>
				{#if option.description}
					<p id="{name}-{option.value}-hint" class="description">{option.description}</p>
				{/if}
			</div>
		{/each}
		<!-- v8 ignore stop -->
	</stack-l>
{/snippet}

{#if legend === undefined}
	{@render errorMessage()}
	{@render radios()}
{:else}
	<!-- aria-describedby only: aria-invalid is not supported on role="group",
	     which <fieldset> carries implicitly. The refusal reaches the reader
	     through the description and through role="alert". -->
	<fieldset aria-describedby={error ? errorId : undefined}>
		<legend>{legend}</legend>
		{@render errorMessage()}
		{@render radios()}
	</fieldset>
{/if}

<style>
	@layer components {
		fieldset {
			padding: 0;
			border: none;
		}

		/* 20px from the group's name to its first option, the same gap
		   the options keep between themselves -- the brief's Density
		   section, and what a legend flush against a radio was missing. */
		legend {
			padding: 0;
			margin-block-end: var(--space-5);
			font-weight: var(--font-weight-medium);
			color: var(--color-on-surface);
		}

		/* A fieldset inside a disabled one is disabled too, so the
		   question dims with its options rather than reading as live. */
		fieldset:disabled > legend {
			opacity: var(--opacity-disabled);
		}

		/* The same weight and color LabeledField gives a refusal, so one
		   error reads the same as the next whichever control it belongs to. */
		.error {
			margin: 0 0 var(--space-3);
			color: var(--color-error);
			font-size: var(--text-body-sm-size);
		}

		/*
		 * The control takes its own width and the label takes what it
		 * needs of the rest, wrapping inside its column. The `0` minimum:
		 * a track's automatic minimum is the label's longest word, which
		 * is what would push a long unbroken name past the edge. The
		 * `max-content` maximum, not `1fr` (#1518): the label is the click
		 * target, and on a wide screen a `1fr` track made the empty space
		 * far to the right of "No Doula yet" select it too. GOV.UK ends
		 * the target where the text ends.
		 */
		.choice {
			display: grid;
			grid-template-columns: auto minmax(0, max-content);
			/* Or the free space goes to the `auto` track, and the text
			   is pushed to the far edge. */
			justify-content: start;
			column-gap: var(--space-4);
		}

		/* The label takes both tracks of its choice through a subgrid, so
		   the hint below can sit in the second track, under the label's
		   text, with no arithmetic copied from the circle's size (#1518). */
		.option {
			display: grid;
			grid-column: 1 / -1;
			grid-template-columns: subgrid;
			align-items: start;
			cursor: pointer;
		}

		/*
		 * Centered on the label's first line, whatever size the type scale
		 * gives the text (#1518): the text is pushed down by half of what
		 * the circle is taller than one line. A label that wraps keeps
		 * its first line level with the circle and grows downward.
		 */
		.text {
			padding-block: calc((var(--control-height) - 1lh) / 2);
			overflow-wrap: anywhere;
		}

		/*
		 * GOV.UK's radio, drawn on the real <input> (#1518). The browser's
		 * own circle is about 13px whatever the text beside it, and sits
		 * high against the line. This is GOV.UK's proportion: a circle the
		 * height of a text input, a 2px ring, and a dot half the circle's
		 * width. The dot is the content box painted in the ring's color,
		 * with the padding as the gap between them, so no pseudo-element
		 * is needed on an element that engines do not all give one to.
		 */
		input {
			appearance: none;
			inline-size: var(--control-height);
			block-size: var(--control-height);
			/* Never smaller than the text beside it. A radio with
			   `appearance: none` has no size of its own, so a page that
			   somehow lacked the tokens -- a unit spec mounting this
			   without tokens.css is one -- would get a circle of nothing
			   that nobody could see or press. */
			min-inline-size: 1em;
			min-block-size: 1em;
			margin: 0;
			padding: calc(var(--control-height) / 4 - var(--border-active));
			border: var(--border-active) solid var(--color-on-surface);
			border-radius: var(--radius-pill);
			background-color: var(--color-surface);
			background-clip: content-box;
			cursor: pointer;
		}

		input:checked {
			border-color: var(--color-primary);
			background-color: var(--color-primary);
		}

		/* GOV.UK's hover: a soft halo, so the row under the pointer is
		   plain without borrowing the focus ring's or the error's color. */
		.option:hover input:not(:disabled) {
			box-shadow: 0 0 0 var(--space-2) var(--color-outline-variant);
		}

		/* The same ring every other control draws (#452). */
		input:focus-visible {
			outline: var(--focus-ring-width) solid var(--color-primary);
			outline-offset: var(--focus-ring-offset);
		}

		/* A refused group: each unchosen ring in the error color, the way
		   TextInput turns its border, beside the message above. A chosen
		   one keeps its own color: the refusal belongs to the question,
		   as GOV.UK marks the group, and a choice is usually its answer. */
		.invalid input:not(:checked) {
			border-color: var(--color-error);
		}

		/*
		 * GOV.UK has no disabled radio, and no caller disables one (#1432
		 * leaves out a question with one answer instead), so there is no
		 * prop for it. A <fieldset disabled> around a form can still reach
		 * these inputs, so they look it: dimmed the way TextInput is.
		 */
		.option:has(input:disabled) {
			cursor: not-allowed;
			opacity: var(--opacity-disabled);
		}

		input:disabled {
			cursor: not-allowed;
		}

		/*
		 * Windows High Contrast drops the painted background, which would
		 * leave a checked radio without its dot. The system's own radio is
		 * drawn in the system's own colors, so it is handed back there.
		 */
		@media (forced-colors: active) {
			input {
				appearance: auto;
				padding: 0;
			}
		}

		/*
		 * Quieter than the label it belongs to, and in the label's own
		 * track so it reads as part of that option rather than as the
		 * next one -- the same treatment `LabeledField` gives a hint, which
		 * is the same job.
		 */
		.description {
			grid-column: 2;
			margin: var(--space-1) 0 0;
			color: var(--color-on-surface-muted);
			font-size: var(--text-body-sm-size);
		}
	}
</style>
