<script lang="ts">
	/*
	 * A memorable date, asked in three boxes -- GOV.UK's Dates pattern,
	 * and #466's replacement for the single `type="date"` control intake
	 * carried before it. Month, day, year in that order, for a US
	 * Practice.
	 *
	 * ## Why not `TextInput type="date"`
	 *
	 * That atom's own comment argues the native control's case, and it is
	 * a good one for a date a person is *choosing* -- a due date, an
	 * appointment. GOV.UK's rule is about a date somebody already knows:
	 * a picker makes a reader navigate to 1988 to enter a fact they could
	 * have typed in four keystrokes, and a single text box makes them
	 * guess a format. Three labelled boxes ask for it the way it is
	 * remembered.
	 *
	 * ## The legend is optional, like `RadioGroup`'s
	 *
	 * On a question page the Template already owns the <fieldset> and its
	 * <legend> is the <h1> (#464), so this renders neither. Passed a
	 * legend, it renders its own group -- what a form asking a date
	 * alongside other questions needs.
	 *
	 * ## Composition is not this component's business
	 *
	 * `intakeDate.ts` owns the "YYYY-MM-DD" round trip, the two-digit
	 * year window and what counts as a real date. This renders boxes and
	 * reports what is in them.
	 */
	import TextInput from '#lib/components/atoms/TextInput.svelte';
	import type { DateField, DateParts } from '#lib/intakeDate.js';

	interface Properties {
		/**
		 * Prefixes each box's id, so an error summary entry can link to the
		 * one box that has to change.
		 */
		name: string;
		parts: DateParts;
		onChange: (parts: DateParts) => void;
		/**
		 * Omitted on a question page -- see the comment above.
		 */
		legend?: string;
		/**
		 * The refusal for the group. Announced by role="alert", the same
		 * arrangement `RadioGroup` uses for a group with no legend to be
		 * described by.
		 */
		error?: string;
		/**
		 * Which box the refusal is about, so only that one is marked.
		 */
		invalidField?: DateField;
	}

	let { name, parts, onChange, legend, error, invalidField }: Properties = $props();

	const errorId = $derived(`${name}-error`);

	const boxes = $derived([
		{ field: 'month' as const, label: 'Month', width: 2 },
		{ field: 'day' as const, label: 'Day', width: 2 },
		{ field: 'year' as const, label: 'Year', width: 4 }
	]);

	function set(field: DateField, value: string) {
		onChange({ ...parts, [field]: value });
	}
</script>

{#snippet errorMessage()}
	{#if error}
		<p id={errorId} class="error" role="alert">{error}</p>
	{/if}
{/snippet}

{#snippet dateBoxes()}
	<cluster-l space="var(--space-4)">
		<!-- v8 ignore start: only the compiled branch for "was this keyed
		     <div> added/removed from the DOM since the last render" is
		     unreachable here (Svelte's own each-block diffing internals,
		     not app code) -- the loop body itself is exercised by "asks
		     for the month, the day and the year, in that order" in
		     DateFields.svelte.spec.ts -->
		{#each boxes as box (box.field)}
			<div class="box" class:wide={box.width === 4}>
				<label for="{name}-{box.field}">{box.label}</label>
				<TextInput
					id="{name}-{box.field}"
					value={parts[box.field]}
					onInput={(value) => set(box.field, value)}
					inputmode="numeric"
					maxlength={box.width}
					invalid={invalidField === box.field}
					describedBy={error ? errorId : undefined}
					autocomplete="off"
				/>
			</div>
		{/each}
		<!-- v8 ignore stop -->
	</cluster-l>
{/snippet}

{#if legend === undefined}
	{@render errorMessage()}
	{@render dateBoxes()}
{:else}
	<fieldset aria-describedby={error ? errorId : undefined}>
		<legend>{legend}</legend>
		{@render errorMessage()}
		{@render dateBoxes()}
	</fieldset>
{/if}

<style>
	@layer components {
		fieldset {
			padding: 0;
			border: none;
			min-inline-size: 0;
		}

		legend {
			padding: 0;
			margin-block-end: var(--space-5);
			font-weight: var(--font-weight-medium);
			color: var(--color-on-surface);
		}

		/* The same weight and colour every other refusal in the app
		   carries -- `LabeledField`'s and `RadioGroup`'s. */
		.error {
			margin: 0 0 var(--space-3);
			color: var(--color-error);
			font-size: var(--text-body-sm-size);
		}

		label {
			display: block;
			font-weight: var(--font-weight-medium);
			color: var(--color-on-surface);
		}

		/*
		 * Content sizing, which #466 makes these routes' own business:
		 * the Templates guarantee a --form-max column and no field
		 * widths. A month is two digits and a year is four, and GOV.UK
		 * sizes them that way because a box that could hold a sentence
		 * invites one.
		 *
		 * `ch` is the advance width of a zero, so the number of digits is
		 * written as digits and tracks the font rather than a canvas
		 * measurement. `--space-6` is `TextInput`'s own `--space-3` of
		 * horizontal padding on each side; without it the box is sized
		 * for the text and the text has nowhere to sit. One character of
		 * slack past the digits it holds, because `ch` measures a zero
		 * and a 4 is wider -- "1988" clipped at an exact 4ch, which is
		 * the kind of thing only the rendered page says. GOV.UK's own
		 * two- and four-character widths are more generous still.
		 *
		 * `min-inline-size: 0` is what makes any of it apply. A flex
		 * item's automatic minimum is its min-content, and an <input>'s
		 * min-content is the browser's default `size` -- about 193px --
		 * so every box rendered at the full column width until this was
		 * added. Caught by looking at the rendered page: the 320px sweep
		 * measures overflow, and three boxes that are each too WIDE for
		 * their content still fit, one under the other.
		 *
		 * `cluster-l` still wraps, so at a large text size the three
		 * boxes drop onto their own lines rather than overflowing
		 * (ADR-0024 rule 1).
		 */
		.box {
			flex: 0 1 auto;
			min-inline-size: 0;
			inline-size: calc(3ch + var(--space-6));
		}

		.box.wide {
			inline-size: calc(5ch + var(--space-6));
		}

		/*
		 * `TextInput` sets no width of its own, so an <input> takes the
		 * browser's default `size` -- about 208px -- whatever column it is
		 * put in. `Select` and `Textarea` both stretch, which is why the
		 * three controls in one form do not agree; that is app-wide and is
		 * its own ticket. Here the box is the width, and the input fills
		 * it, so a two-character box holds a two-character control rather
		 * than painting over its neighbour.
		 */
		.box :global(input) {
			inline-size: 100%;
		}
	}
</style>
