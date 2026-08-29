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
		minlength,
		inputmode
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
	{inputmode}
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
			outline: 2px solid var(--color-primary);
			outline-offset: 2px;
		}
	}
</style>
