<script lang="ts">
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
		 * Overrides the accessible name for a checkbox whose wrapping
		 * <label> text isn't enough to tell it apart -- a multi-select
		 * option repeated under several different questions, each needing
		 * its own question name prefixed on (#492). LabeledField remains
		 * the default; this is the recorded exception to it.
		 */
		ariaLabel?: string;
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
		ariaLabel
	}: Properties = $props();
</script>

<input
	{id}
	type="checkbox"
	{name}
	{checked}
	{disabled}
	{required}
	class:invalid
	class:toggle={variant === 'toggle'}
	role={variant === 'toggle' ? 'switch' : undefined}
	aria-invalid={invalid}
	aria-describedby={describedBy}
	aria-label={ariaLabel}
	onchange={(event_) => onChange(event_.currentTarget.checked)}
/>

<style>
	@layer components {
		input {
			inline-size: 1.5rem;
			block-size: 1.5rem;
			accent-color: var(--color-primary);
			cursor: pointer;
		}

		input.invalid {
			outline: var(--focus-ring-width) solid var(--color-error);
			outline-offset: var(--focus-ring-offset);
		}

		input:disabled {
			cursor: not-allowed;
			opacity: 0.6;
		}

		input:focus-visible {
			outline: var(--focus-ring-width) solid var(--color-primary);
			outline-offset: var(--focus-ring-offset);
		}

		input.toggle {
			appearance: none;
			position: relative;
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
			inline-size: 1.2rem;
			block-size: 1.2rem;
			border-radius: 50%;
			background-color: var(--color-surface);
		}

		input.toggle:checked {
			background-color: var(--color-primary);
		}

		input.toggle:checked::before {
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
