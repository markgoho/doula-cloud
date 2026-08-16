<script lang="ts">
	interface Props {
		id?: string;
		name?: string;
		checked: boolean;
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
	}: Props = $props();
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
	onchange={(e) => onChange(e.currentTarget.checked)}
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
