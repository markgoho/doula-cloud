<script lang="ts">
	import type { HTMLInputAttributes } from 'svelte/elements';

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
		min,
		inputmode,
		autocomplete
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
		 * The floor a type="number" field will accept -- a quantity of
		 * credits starts at 1, not at 0 or -3. Separate from `minlength`,
		 * which counts characters rather than reading the value.
		 */
		min?: number;
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
	} = $props();
</script>

<input
	{id}
	{type}
	{name}
	{value}
	{placeholder}
	{required}
	{disabled}
	{minlength}
	{min}
	{inputmode}
	{autocomplete}
	class:invalid
	aria-invalid={invalid}
	aria-describedby={describedBy}
	oninput={(event_) => onInput(event_.currentTarget.value)}
/>

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
	}
</style>
