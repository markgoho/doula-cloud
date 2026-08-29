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
		variant = 'checkbox'
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
			outline: 2px solid var(--color-error);
			outline-offset: 2px;
		}

		input:disabled {
			cursor: not-allowed;
			opacity: 0.6;
		}

		input:focus-visible {
			outline: 2px solid var(--color-primary);
			outline-offset: 2px;
		}

		input.toggle {
			appearance: none;
			position: relative;
			inline-size: 2.5rem;
			block-size: 1.5rem;
			border: var(--border-thin) solid var(--color-outline);
			border-radius: 999px;
			background-color: var(--color-outline-variant);
		}

		input.toggle::before {
			content: '';
			position: absolute;
			inset-block-start: 1px;
			inset-inline-start: 1px;
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
				transition: background-color 150ms ease;
			}

			input.toggle::before {
				transition: translate 150ms ease;
			}
		}
	}
</style>
