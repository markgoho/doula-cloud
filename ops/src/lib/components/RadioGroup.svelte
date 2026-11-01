<script lang="ts">
	// GOV.UK's radios: one legend, one choice. The first radio takes the id
	// an error summary links to.
	let {
		id,
		legend,
		options,
		error,
		value = $bindable('')
	}: { id: string; legend: string; options: { value: string; label: string }[]; error?: string; value?: string } =
		$props();
</script>

<fieldset class:invalid={error} aria-describedby={error ? `${id}-error` : undefined}>
	<legend>{legend}</legend>
	{#if error}
		<p class="error" id={`${id}-error`}>{error}</p>
	{/if}
	{#each options as option, index (option.value)}
		<label class="option">
			<input
				type="radio"
				id={index === 0 ? id : `${id}-${index}`}
				name={id}
				value={option.value}
				checked={value === option.value}
				onchange={() => (value = option.value)}
			/>
			<span>{option.label}</span>
		</label>
	{/each}
</fieldset>

<style>
	fieldset {
		display: flex;
		flex-direction: column;
		gap: var(--space-2);
		min-inline-size: 0;
		margin: 0;
		padding: 0;
		border: 0;
	}

	fieldset.invalid {
		padding-inline-start: var(--space-3);
		border-inline-start: var(--border-active) solid var(--color-error);
	}

	legend {
		margin-block-end: var(--space-label);
		padding: 0;
		font-weight: bold;
	}

	.error {
		color: var(--color-error);
		font-weight: bold;
	}

	.option {
		display: flex;
		gap: var(--space-2);
		align-items: center;
		min-block-size: var(--control-height);
		overflow-wrap: anywhere;
	}

	input {
		flex-shrink: 0;
		inline-size: 1.5rem;
		block-size: 1.5rem;
		margin: 0;
		accent-color: var(--color-primary);
	}

	input:focus-visible {
		outline: var(--focus-ring-width) solid var(--color-primary);
		outline-offset: var(--focus-ring-offset);
	}
</style>
