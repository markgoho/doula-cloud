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
		placeholder,
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
		passwordLabel = 'password'
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
		 */
		type?: 'text' | 'email' | 'password' | 'tel' | 'url' | 'search' | 'number' | 'date';
		name?: string;
		placeholder?: string;
		required?: boolean;
		disabled?: boolean;
		invalid?: boolean;
		describedBy?: string;
		minlength?: number;
		/*
		 * The hard cap #468 found missing: a one-character initial and a
		 * six-digit access code both silently truncate pasted text without
		 * it, which is the behaviour the two callers want rather than a
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
		 * control exists yet -- a prefix and rune-safe cents parsing stay
		 * fog until a third money field justifies one (#492).
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
	} = $props();

	// Defaults to hidden (#470): a password field starts masked, and only
	// this toggle -- never the browser back button, a re-render, or a
	// prop change -- reveals it.
	let isRevealed = $state(false);
	const isPassword = $derived(type === 'password');
	const effectiveType = $derived(isPassword && isRevealed ? 'text' : type);
</script>

{#snippet field()}
	<input
		{id}
		type={effectiveType}
		{name}
		{value}
		{placeholder}
		{required}
		{disabled}
		{minlength}
		{maxlength}
		{min}
		{step}
		{inputmode}
		{autocomplete}
		aria-label={ariaLabel}
		class:invalid
		class:password-input={isPassword}
		aria-invalid={invalid}
		aria-describedby={describedBy}
		oninput={(event_) => onInput(event_.currentTarget.value)}
	/>
{/snippet}

{#if isPassword}
	<span class="password-field">
		{@render field()}
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
	</span>
{:else}
	{@render field()}
{/if}

<style>
	@layer components {
		input {
			min-height: 2.5rem;
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
			opacity: 0.6;
		}

		input:focus-visible {
			outline: var(--focus-ring-width) solid var(--color-primary);
			outline-offset: var(--focus-ring-offset);
		}

		.password-field {
			display: flex;
			align-items: stretch;
			inline-size: 100%;
		}

		input.password-input {
			flex: 1;
			min-inline-size: 0;
			border-start-end-radius: 0;
			border-end-end-radius: 0;
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
			opacity: 0.6;
		}
	}
</style>
