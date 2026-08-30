<script lang="ts" generics="T extends string">
	interface Option<T> {
		value: T;
		label: string;
		/*
		 * The consequence of choosing this option, in the reader's words --
		 * GOV.UK's radio hint. Added on #464 for the intake duplicate
		 * check (#432): that page offers the Practice's existing Clients as
		 * options, and a name alone cannot tell two Sarahs apart. The
		 * history and what picking her will do belong to the option, not to
		 * the group, so a `label` string could not carry them.
		 */
		description?: string;
	}

	interface Properties<T> {
		/*
		 * Optional, like `FormPage`'s (#425), and for the same reason: a
		 * question page makes the group's name the page's own <h1>
		 * (#464), so the Template already owns the <fieldset> and its
		 * <legend>. A second one here would nest fieldsets and announce
		 * the question twice.
		 */
		legend?: string;
		name?: string;
		options: Option<T>[];
		value: T;
		onChange: (value: T) => void;
	}

	const generatedName = $props.id();

	let { legend, name = generatedName, options, value, onChange }: Properties<T> = $props();
</script>

{#snippet radios()}
	<stack-l space="var(--space-5)">
		<!-- v8 ignore start: only the compiled branch for "was this keyed
		     <div> added/removed from the DOM since the last render" is
		     unreachable here (Svelte's own each-block diffing internals, not
		     app code) -- the loop body itself is exercised by
		     "renders an option for each entry in options" in
		     RadioGroup.svelte.spec.ts -->
		{#each options as option (option.value)}
			<div>
				<cluster-l>
					<input
						type="radio"
						id="{name}-{option.value}"
						{name}
						value={option.value}
						checked={option.value === value}
						aria-describedby={option.description ? `${name}-${option.value}-hint` : undefined}
						onchange={() => onChange(option.value)}
					/>
					<label for="{name}-{option.value}">{option.label}</label>
				</cluster-l>
				{#if option.description}
					<p id="{name}-{option.value}-hint" class="description">{option.description}</p>
				{/if}
			</div>
		{/each}
		<!-- v8 ignore stop -->
	</stack-l>
{/snippet}

{#if legend === undefined}
	{@render radios()}
{:else}
	<fieldset>
		<legend>{legend}</legend>
		{@render radios()}
	</fieldset>
{/if}

<style>
	@layer components {
		fieldset {
			padding: 0;
			border: none;
		}

		/* 20px from the group's name to its first option, the same gap
		   the options keep between themselves -- the brief's Density
		   section, and what a legend flush against a radio was missing. */
		legend {
			padding: 0;
			margin-block-end: var(--space-5);
			font-weight: var(--font-weight-medium);
			color: var(--color-on-surface);
		}

		input {
			accent-color: var(--color-primary);
			cursor: pointer;
		}

		input:focus-visible {
			outline: var(--focus-ring-width) solid var(--color-primary);
			outline-offset: var(--focus-ring-offset);
		}

		/*
		 * Quieter than the label it belongs to, and indented past the
		 * control so it reads as part of that option rather than as the
		 * next one -- the same treatment `LabeledField` gives a hint, which
		 * is the same job.
		 */
		.description {
			margin: var(--space-1) 0 0;
			padding-inline-start: var(--space-6);
			color: var(--color-on-surface-muted);
			font-size: var(--text-body-sm-size);
		}
	}
</style>
