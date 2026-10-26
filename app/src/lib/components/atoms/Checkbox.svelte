<script lang="ts">
	import type { HTMLInputAttributes } from 'svelte/elements';

	interface Properties {
		id?: string;
		name?: string;
		checked: boolean;
		// eslint-disable-next-line unicorn/consistent-boolean-name -- mirrors the native HTMLInputElement `checked` property this atom wraps 1:1
		onChange: (checked: boolean) => void;
		disabled?: boolean;
		required?: boolean;
		invalid?: boolean;
		describedBy?: string;
		variant?: 'checkbox' | 'toggle';
		/*
		 * Whose data this is, per docs/design/govuk-alignment.md (#469).
		 * A checkbox is autofilled the same as any other control, and
		 * intake collecting a Practice's own questions ABOUT a Client
		 * (#466) is the first caller that has to say "off".
		 */
		autocomplete?: HTMLInputAttributes['autocomplete'];
	}

	const generatedId = $props.id();

	let {
		id = generatedId,
		name,
		checked,
		onChange,
		disabled = false,
		required = false,
		invalid = false,
		describedBy,
		variant = 'checkbox',
		autocomplete
	}: Properties = $props();
</script>

<input
	{id}
	type="checkbox"
	{name}
	{checked}
	{disabled}
	{required}
	{autocomplete}
	class:invalid
	class:toggle={variant === 'toggle'}
	role={variant === 'toggle' ? 'switch' : undefined}
	aria-invalid={invalid}
	aria-describedby={describedBy}
	onchange={(event_) => onChange(event_.currentTarget.checked)}
/>

<style>
	@layer components {
		/*
		 * GOV.UK's box, drawn on the real <input> (#2014), the same way
		 * RadioGroup draws its circle (#1518): `--choice-size` square, a
		 * `--border-active` edge, and a tick when chosen. The browser's
		 * own box, tinted by accent-color, was 24px beside a 40px radio
		 * on the same form, and its edge is whatever the engine draws.
		 * The tick is the box's ::before, an L of two borders turned
		 * through 45 degrees, every length a share of the box so it
		 * keeps its shape at any size.
		 */
		input:not(.toggle) {
			appearance: none;
			position: relative;
			flex-shrink: 0;
			inline-size: var(--choice-size);
			block-size: var(--choice-size);
			/* Never smaller than the text beside it, for the reason
			   RadioGroup gives: a box with `appearance: none` and no token
			   to size it would be a box of nothing. tokens:ignore -- `1em`
			   is the text's own size, a floor and not a design value. */
			min-inline-size: 1em;
			min-block-size: 1em;
			margin: 0;
			border: var(--border-active) solid var(--color-on-surface);
			border-radius: 0;
			background-color: var(--color-surface);
			cursor: pointer;
		}

		input:not(.toggle):checked {
			border-color: var(--color-primary);
			background-color: var(--color-primary);
		}

		input:not(.toggle):checked::before {
			content: '';
			position: absolute;
			inset-block-start: 22%;
			inset-inline-start: 18%;
			inline-size: 55%;
			block-size: 28%;
			border: solid var(--color-on-primary);
			border-width: 0 0 calc(var(--choice-size) / 8) calc(var(--choice-size) / 8);
			rotate: -45deg;
		}

		/* A refused box takes the error color on its edge, the way a
		   refused radio's ring does, beside the message above it. A
		   ticked one keeps its own color: it is usually the answer. */
		input.invalid:not(:checked) {
			border-color: var(--color-error);
		}

		/* Windows High Contrast drops the painted background and the
		   tick with it, so the system's own box is handed back. */
		@media (forced-colors: active) {
			input:not(.toggle) {
				appearance: auto;
			}

			input:not(.toggle):checked::before {
				content: none;
			}
		}

		input:disabled {
			cursor: not-allowed;
			opacity: var(--opacity-disabled);
		}

		input:focus-visible {
			outline: var(--focus-ring-width) solid var(--color-primary);
			outline-offset: var(--focus-ring-offset);
		}

		input.toggle {
			appearance: none;
			position: relative;
			/* tokens:ignore -- the toggle's drawn track: its width, height,
			   knob and travel are one shape and move with nothing else. */
			inline-size: 2.5rem;
			block-size: 1.5rem;
			border: var(--border-thin) solid var(--color-outline);
			border-radius: var(--radius-pill);
			background-color: var(--color-outline-variant);
		}

		input.toggle::before {
			content: '';
			position: absolute;
			inset-block-start: var(--border-thin);
			inset-inline-start: var(--border-thin);
			/* tokens:ignore -- the toggle's drawn knob, sized to its track. */
			inline-size: 1.2rem;
			block-size: 1.2rem;
			border-radius: 50%;
			background-color: var(--color-surface);
		}

		input.toggle:checked {
			background-color: var(--color-primary);
		}

		input.toggle:checked::before {
			/* tokens:ignore -- the knob's travel along its drawn track. */
			translate: 1rem 0;
		}

		@media (prefers-reduced-motion: no-preference) {
			input.toggle {
				transition: background-color var(--motion-state) var(--ease-out);
			}

			input.toggle::before {
				transition: translate var(--motion-state) var(--ease-out);
			}
		}
	}
</style>
