<script lang="ts">
	interface Properties {
		id?: string;
		name?: string;
		options: string[];
		value?: string;
		placeholder?: string;
		disabled?: boolean;
		required?: boolean;
		invalid?: boolean;
		describedBy?: string;
	}

	const uid = $props.id();

	let {
		id = uid,
		name,
		options,
		value = $bindable(''),
		placeholder,
		disabled = false,
		required = false,
		invalid = false,
		describedBy
	}: Properties = $props();
</script>

<select
	{id}
	{name}
	bind:value
	{disabled}
	{required}
	aria-invalid={invalid}
	aria-describedby={describedBy}
>
	{#if placeholder}
		<option value="" disabled>{placeholder}</option>
	{/if}
	<!-- v8 ignore start: only the compiled branch for "was this keyed <option>
	     added/removed from the DOM since the last render" is unreachable here
	     (Svelte's own each-block diffing internals, not app code) -- the
	     <option> line itself is fully exercised by "renders an option for
	     each entry in options" in Select.svelte.spec.ts -->
	{#each options as option (option)}
		<option value={option}>{option}</option>
	{/each}
	<!-- v8 ignore stop -->
</select>

<style>
	@layer components {
		/* Customizable <select> (appearance: base-select) is Chrome/Edge 135+
		   only. Safari drops the unrecognized value/pseudo-elements below and
		   falls back to its native OS picker for the dropdown -- the closed
		   control (this rule) stays fully themed everywhere either way. */
		select {
			appearance: base-select;
			width: 100%;
			padding: var(--space-2) var(--space-3);
			font-family: var(--font-family-base);
			font-size: var(--text-body-size);
			color: var(--color-on-surface);
			background-color: var(--color-surface);
			border: var(--border-thin) solid var(--color-outline);
			border-radius: var(--radius);
		}

		select::picker(select) {
			appearance: base-select;
			margin-top: var(--space-1);
			padding: var(--space-1);
			font-family: var(--font-family-base);
			font-size: var(--text-body-size);
			color: var(--color-on-surface);
			background-color: var(--color-surface);
			border: var(--border-thin) solid var(--color-outline);
			border-radius: var(--radius);
		}

		select::picker-icon {
			color: var(--color-on-surface-variant);
		}

		select option {
			padding: var(--space-2) var(--space-3);
			border-radius: var(--radius);
		}

		select option:hover {
			background-color: var(--color-outline-variant);
		}

		select option::checkmark {
			color: var(--color-primary);
		}

		select:focus-visible {
			outline: 2px solid var(--color-primary);
			outline-offset: 2px;
		}

		select[aria-invalid='true'] {
			border-color: var(--color-error);
		}

		select:disabled {
			opacity: 0.6;
			cursor: not-allowed;
		}
	}
</style>
