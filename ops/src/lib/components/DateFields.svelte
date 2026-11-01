<script lang="ts">
	// GOV.UK's date input: one legend, three boxes in the order a US reader
	// writes a date, the error under the legend. The first box takes the
	// id an error summary links to.
	let {
		id,
		legend,
		hint,
		error,
		month = $bindable(''),
		day = $bindable(''),
		year = $bindable('')
	}: { id: string; legend: string; hint?: string; error?: string; month?: string; day?: string; year?: string } =
		$props();

	const describedBy = $derived(
		[hint ? `${id}-hint` : undefined, error ? `${id}-error` : undefined].filter(Boolean).join(' ') || undefined
	);
</script>

<fieldset class:invalid={error} aria-describedby={describedBy}>
	<legend>{legend}</legend>
	{#if hint}
		<p class="hint" id={`${id}-hint`}>{hint}</p>
	{/if}
	{#if error}
		<p class="error" id={`${id}-error`}>{error}</p>
	{/if}
	<div class="boxes">
		<div class="box">
			<label for={`${id}-month`}>Month</label>
			<input
				id={`${id}-month`}
				class="two"
				inputmode="numeric"
				bind:value={month}
				aria-invalid={error ? 'true' : undefined}
			/>
		</div>
		<div class="box">
			<label for={id}>Day</label>
			<input {id} class="two" inputmode="numeric" bind:value={day} aria-invalid={error ? 'true' : undefined} />
		</div>
		<div class="box">
			<label for={`${id}-year`}>Year</label>
			<input
				id={`${id}-year`}
				class="four"
				inputmode="numeric"
				bind:value={year}
				aria-invalid={error ? 'true' : undefined}
			/>
		</div>
	</div>
</fieldset>

<style>
	fieldset {
		display: flex;
		flex-direction: column;
		gap: var(--space-label);
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

	.hint {
		color: var(--color-on-surface-variant);
	}

	.error {
		color: var(--color-error);
		font-weight: bold;
	}

	.boxes {
		display: flex;
		flex-wrap: wrap;
		gap: var(--space-3);
	}

	.box {
		display: flex;
		flex-direction: column;
		gap: var(--space-label);
	}

	input {
		min-block-size: var(--control-height);
		padding: var(--space-2) var(--space-3);
		color: var(--color-on-surface);
		background-color: var(--color-surface);
		border: var(--border-thin) solid var(--color-outline);
		border-radius: var(--radius);
	}

	.two {
		inline-size: 5ch;
		box-sizing: content-box;
	}

	.four {
		inline-size: 7ch;
		box-sizing: content-box;
	}

	.invalid input {
		border-color: var(--color-error);
	}

	input:focus-visible {
		outline: var(--focus-ring-width) solid var(--color-primary);
		outline-offset: var(--focus-ring-offset);
	}
</style>
