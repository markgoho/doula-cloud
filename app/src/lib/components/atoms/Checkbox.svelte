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
		describedBy
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
	aria-invalid={invalid}
	aria-describedby={describedBy}
	onchange={(event_) => onChange(event_.currentTarget.checked)}
/>

<style>
	@layer components {
		input {
			inline-size: 1.5rem;
			block-size: 1.5rem;
			accent-color: var(--color-accent);
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
			outline: 2px solid var(--color-accent);
			outline-offset: 2px;
		}
	}
</style>
