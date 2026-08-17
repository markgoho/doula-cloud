<script lang="ts">
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
		minlength
	}: {
		id?: string;
		value: string;
		onInput: (value: string) => void;
		type?: 'text' | 'email' | 'password' | 'tel' | 'url' | 'search' | 'number';
		name?: string;
		placeholder?: string;
		required?: boolean;
		disabled?: boolean;
		invalid?: boolean;
		describedBy?: string;
		minlength?: number;
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
			color: var(--color-text);
			background-color: var(--color-bg);
			border: var(--border-thin) solid var(--color-input-border);
			border-radius: var(--radius-sm);
		}

		input.invalid {
			border-color: var(--color-error);
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
