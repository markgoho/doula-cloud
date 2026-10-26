<!--
	The single-line text control, and the one decision that is not a prop.

	## It fills the space it is given

	An `<input>` with no width of its own paints at the browser's default
	`size` -- about 208px, whichever column it is put in. That is not a
	width anybody chose, and it is the reason a form holding all three
	controls did not agree with itself: `Select` and `Textarea` both
	stretch, so a full-width Select sat directly above a 208px box asking
	for a street address (#805).

	So this one stretches too, and that is govuk-frontend's own base
	rather than a departure from it. Its `_mixin.scss` sets `.govuk-input
	{ width: 100% }`, and every `govuk-input--width-N` class is a
	`max-width` that *narrows* that base. GOV.UK's Text input sizing rule
	-- a box that could hold a sentence invites one -- is the per-field
	step layered on top, never the default.

	## How a caller narrows one field: `width`, in characters

	`width` is GOV.UK's own scale -- `govuk-input--width-2/3/4/5/10/20/30`
	-- named by how many characters the field holds, never by a length:
	`width={20}` is a name, `width={10}` a six-digit code or a ZIP+4,
	`width={2}` a week of pregnancy. A caller picks the smallest step
	that holds the longest answer the field takes, and a field that holds
	a sentence or an address line passes nothing and keeps the column.

	It is a `max-inline-size`, never a width, so the 100% base above still
	holds below it: in a 320px column a `width={30}` box shrinks with the
	column instead of overflowing it (ADR-0024). The length is `ch` -- the
	advance of a zero, so it tracks the font rather than a canvas
	measurement -- with one character of slack past the count, because a
	4 is wider than a 0 and "1988" clipped at an exact 4ch (`DateFields`
	found that on the rendered page), plus this atom's own padding and
	border, which sit inside the box rather than beside it.

	This was a wrapper per caller until #1216. #805 left it there on
	purpose -- two callers each narrowing with one declaration on a
	wrapper they already rendered was below the bar for a prop -- and
	recorded the bar: "a third with the same shape". The design audit of
	#2001 found about eighteen screens with the same shape, so the widths
	are named here once, on this atom and on `Select` and `Textarea`
	beside it, and `LabeledField` passes a field's `width` through to
	whichever of the three it holds. The intake address route's two
	wrapper widths moved onto the prop with it.

	**A date sizes to its control, and takes no `width`.** A native
	`type="date"` or `type="datetime-local"` control lays its own
	segments out, so it is capped at its own content width: a column-wide
	box around one is mostly empty, and a character count would be a
	guess at the browser's segment layout.

	**`prefix` and `suffix` say the unit** -- "$" before a fee, "days"
	after a count -- where the unit is what makes the narrow box read
	right. They are `aria-hidden`, as GOV.UK's are, so the unit still has
	to be in the label or the hint: a screen reader reads the label, not
	the box beside the field.

	`LabeledField`'s `orientation="inline"` keeps its `auto` track, which
	is shrink-to-fit: the track sizes to the control rather than the
	other way round, so a percentage resolves against an intrinsic width
	the control itself contributes. That is the right answer for the
	checkbox and the toggle that row holds, and a `width` there is still a
	cap. That component's own comment records it.
-->
<script module lang="ts">
	/*
	 * GOV.UK's Text input widths, as the number of characters each holds
	 * (#1216). Exported from here because the three input atoms and
	 * `LabeledField` share one scale -- see the comment at the top of
	 * this file.
	 */
	export type InputWidth = 2 | 3 | 4 | 5 | 10 | 20 | 30;

	/** The scale in order, smallest first, for a caller that picks the
	 * smallest step that holds a known length (#1999). */
	export const INPUT_WIDTHS: readonly InputWidth[] = [2, 3, 4, 5, 10, 20, 30];
</script>

<script lang="ts">
	import type { HTMLInputAttributes } from 'svelte/elements';
	import Icon from './Icon.svelte';

	const generatedId = $props.id();

	let {
		id = generatedId,
		value,
		onInput,
		type = 'text',
		name,
		required = false,
		disabled = false,
		invalid = false,
		describedBy,
		minlength,
		maxlength,
		min,
		step,
		inputmode,
		autocomplete,
		ariaLabel,
		passwordLabel = 'password',
		passwordrules,
		width,
		prefix,
		suffix
	}: {
		id?: string;
		value: string;
		onInput: (value: string) => void;
		/*
		 * type="date" wraps the native control rather than composing a
		 * day/month/year set of its own (#404): the native control already
		 * owns locale-correct formatting, a calendar picker and keyboard
		 * support, and Chromium exposes it with the same accessible role
		 * ("textbox") as the rest of this atom, so LabeledField's id/
		 * describedBy/invalid wiring needs no special case for it. Its three
		 * segments (day, month, year) each get their own accessible name
		 * from the browser in the page's locale -- this atom's `label`
		 * names the field as a whole, not each segment. Arrow keys move
		 * within a segment, Tab moves between them, and a screen reader
		 * announces each segment's name and value as it is focused.
		 *
		 * type="datetime-local" is the same case, plus a time: a Visit's
		 * scheduled instant (#250) is chosen, not memorized, and carries a
		 * time of day GOV.UK's three-box Dates pattern has no room for.
		 */
		type?: 'text' | 'email' | 'password' | 'tel' | 'url' | 'search' | 'number' | 'date' | 'datetime-local';
		name?: string;
		/*
		 * No `placeholder` (#1751). GOV.UK's Text input guidance, adopted by
		 * ADR-0021: placeholder text is not a label, a hint or an example,
		 * and it goes away the moment a person types. A hint belongs on
		 * `LabeledField`, where it stays on screen and is announced.
		 */
		required?: boolean;
		disabled?: boolean;
		invalid?: boolean;
		describedBy?: string;
		minlength?: number;
		/*
		 * The hard cap #468 found missing: a one-character initial and a
		 * six-digit access code both silently truncate pasted text without
		 * it, which is the behavior the two callers want rather than a
		 * side effect (#492).
		 */
		maxlength?: number;
		/*
		 * The floor a type="number" field will accept -- a quantity of
		 * credits starts at 1, not at 0 or -3. Separate from `minlength`,
		 * which counts characters rather than reading the value.
		 */
		min?: number;
		/*
		 * type="number"'s increment, so a money field can accept cents
		 * (`step={0.01}`) instead of whole units only. No dedicated money
		 * control exists yet -- a prefix and rune-safe cents parsing stayed
		 * fog until a third money field justified one (#492). #1009's Refund
		 * amount was that third; #1411 builds the control.
		 */
		step?: number;
		/*
		 * Whose data this is, per docs/design/govuk-alignment.md: a WHATWG
		 * token on a field about the person filling the form in, "off" on a
		 * field about someone else (#469). Left undefined rather than
		 * defaulted, so a caller that forgets it gets the browser's own
		 * default rather than a false "off".
		 */
		autocomplete?: HTMLInputAttributes['autocomplete'];
		/*
		 * The on-screen keyboard a phone offers. Separate from `type`
		 * because the two do different jobs: `type` decides what the browser
		 * will refuse to submit, and a field that deliberately accepts what
		 * type="url" refuses -- "facebook.com/your-practice", which the
		 * server normalizes rather than rejects (#440) -- still wants the URL
		 * keyboard.
		 */
		inputmode?: 'text' | 'url' | 'email' | 'tel' | 'numeric' | 'decimal' | 'search';
		/*
		 * The only accessible name when a row has no wrapping <label> or
		 * LabeledField -- a table-style editor where the visible text
		 * belongs to the row, not the field (#492). LabeledField remains
		 * the default; this is the recorded exception to it.
		 */
		ariaLabel?: string;
		/*
		 * Names which password the reveal toggle affects, for its own
		 * accessible name ("Show password" / "Hide password") -- not the
		 * field's own <label>, which LabeledField already owns. Every
		 * password field in the app today is the only one on its screen,
		 * so the generic default reads correctly everywhere; a page that
		 * ever shows two at once (a new-password field beside a
		 * current-password one) passes something that tells them apart
		 * (#470).
		 */
		passwordLabel?: string;
		/*
		 * The rule Safari reads when it generates a password (#1813). A
		 * screen that sets a password passes `PASSWORD_RULES` from
		 * passwordRule.ts; a screen that only reads one passes nothing.
		 */
		passwordrules?: string;
		/*
		 * How many characters the field holds, on GOV.UK's scale -- see
		 * the comment at the top of this file. Unset is the whole column.
		 */
		width?: InputWidth;
		/*
		 * The unit, before or after the box ("$", "days"). Hidden from
		 * assistive technology, so the label or hint has to say it too.
		 */
		prefix?: string;
		suffix?: string;
	} = $props();

	// Defaults to hidden (#470): a password field starts masked, and only
	// this toggle -- never the browser back button, a re-render, or a
	// prop change -- reveals it.
	let isRevealed = $state(false);
	const isPassword = $derived(type === 'password');
	const effectiveType = $derived(isPassword && isRevealed ? 'text' : type);
	const isJoined = $derived(isPassword || Boolean(prefix) || Boolean(suffix));
</script>

{#snippet field()}
	<input
		{id}
		type={effectiveType}
		{name}
		{value}
		{required}
		{disabled}
		{minlength}
		{maxlength}
		{min}
		{step}
		{inputmode}
		{autocomplete}
		{passwordrules}
		aria-label={ariaLabel}
		class:invalid
		class:joined={isJoined}
		class:after-prefix={Boolean(prefix)}
		class:before-end={isPassword || Boolean(suffix)}
		class:sized={width !== undefined}
		style:--input-chars={width}
		aria-invalid={invalid}
		aria-describedby={describedBy}
		oninput={(event_) => onInput(event_.currentTarget.value)}
	/>
{/snippet}

{#if isJoined}
	<span class="joined-field">
		{#if prefix}
			<span class="affix prefix" aria-hidden="true">{prefix}</span>
		{/if}
		{@render field()}
		{#if suffix}
			<span class="affix suffix" aria-hidden="true">{suffix}</span>
		{/if}
		{#if isPassword}
			<button
				type="button"
				class="reveal-toggle"
				{disabled}
				aria-pressed={isRevealed}
				onclick={() => (isRevealed = !isRevealed)}
			>
				<Icon name={isRevealed ? 'eye-slash' : 'eye'} size={20} weight="light" />
				<span class="visually-hidden">{isRevealed ? `Hide ${passwordLabel}` : `Show ${passwordLabel}`}</span>
			</button>
		{/if}
	</span>
{:else}
	{@render field()}
{/if}

<style>
	@layer components {
		input {
			/* The width nobody chose, chosen -- see the comment at the top
			   of this file. `border-box` is global (reset.css), so the
			   padding below sits inside the 100% rather than pushing the
			   control past its column. */
			inline-size: 100%;
			min-height: var(--control-height);
			padding: var(--space-2) var(--space-3);
			color: var(--color-on-surface);
			background-color: var(--color-surface);
			border: var(--border-thin) solid var(--color-outline);
			border-radius: var(--radius);
		}

		input.invalid {
			border-color: var(--color-error);
		}

		input:disabled {
			cursor: not-allowed;
			opacity: var(--opacity-disabled);
		}

		input:focus-visible {
			outline: var(--focus-ring-width) solid var(--color-primary);
			outline-offset: var(--focus-ring-offset);
		}

		/* GOV.UK's Text input width, in characters -- see the comment at
		   the top of this file for the slack and the padding. */
		input.sized {
			--input-chrome: 0ch;

			max-inline-size: calc(
				(var(--input-chars) + 1) * 1ch + var(--space-6) + 2 * var(--border-thin) +
					var(--input-chrome)
			);
		}

		/* The browser draws its spin buttons inside the same box, so a
		   number field's step holds its digits beside them. */
		input.sized[type='number'] {
			--input-chrome: var(--space-5);
		}

		/* A native date control lays out its own segments, so it is
		   capped at its own content width rather than at a guessed
		   character count. Still under the 100% base, so a 320px column
		   wins over it. */
		input:is([type='date'], [type='datetime-local']) {
			max-inline-size: max-content;
		}

		.joined-field {
			display: flex;
			align-items: stretch;
			inline-size: 100%;
		}

		/* `min-inline-size: 0` is this atom's own, not the kind a caller
		   was made to write: the field is a flex item beside the reveal
		   toggle or a unit, and a flex item's automatic minimum would
		   otherwise stop it shrinking far enough to leave those their
		   room. `0 1 auto` rather than `1`, so a sized field stops at its
		   width and the unit sits right after it, not at the column's
		   far edge. */
		input.joined {
			flex: 0 1 auto;
			min-inline-size: 0;
		}

		input.after-prefix {
			border-start-start-radius: 0;
			border-end-start-radius: 0;
		}

		input.before-end {
			border-start-end-radius: 0;
			border-end-end-radius: 0;
		}

		/* GOV.UK's prefix and suffix: a quiet box joined to the field,
		   in the field's own outline, so the unit reads as part of it. */
		.affix {
			display: inline-flex;
			flex: none;
			align-items: center;
			padding-inline: var(--space-3);
			color: var(--color-on-surface-variant);
			background-color: var(--color-surface-container);
			border: var(--border-thin) solid var(--color-outline);
		}

		.prefix {
			border-inline-end: none;
			border-start-start-radius: var(--radius);
			border-end-start-radius: var(--radius);
		}

		.suffix {
			border-inline-start: none;
			border-start-end-radius: var(--radius);
			border-end-end-radius: var(--radius);
		}

		.reveal-toggle {
			display: inline-flex;
			align-items: center;
			justify-content: center;
			min-inline-size: var(--hit-target-min);
			padding: var(--space-2) var(--space-3);
			color: var(--color-on-surface);
			background-color: var(--color-surface);
			border: var(--border-thin) solid var(--color-outline);
			border-inline-start: none;
			border-start-end-radius: var(--radius);
			border-end-end-radius: var(--radius);
		}

		.reveal-toggle:hover {
			background-color: var(--color-outline-variant);
		}

		.reveal-toggle:focus-visible {
			outline: var(--focus-ring-width) solid var(--color-primary);
			outline-offset: var(--focus-ring-offset);
		}

		.reveal-toggle:disabled {
			cursor: not-allowed;
			opacity: var(--opacity-disabled);
		}
	}
</style>
