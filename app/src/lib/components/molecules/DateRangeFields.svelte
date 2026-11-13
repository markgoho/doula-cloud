<script lang="ts">
	import TextInput from '../atoms/TextInput.svelte';
	import LabeledField from './LabeledField.svelte';

	interface Properties {
		from: string;
		to: string;
		/**
		 * What is wrong with the From day, drawn once for the pair above
		 * both boxes; the box keeps the red outline. Absent where the screen
		 * sends whatever is typed.
		 */
		fromError?: string;
		/**
		 * What is wrong with the To day, drawn as `fromError` is.
		 */
		toError?: string;
		/**
		 * What the pair is for, drawn once above both boxes and joined to
		 * each by `aria-describedby`: whether the last day is itself part of
		 * the range ("Both days are included."). Absent, no hint is drawn.
		 */
		hint?: string;
	}

	let { from = $bindable(), to = $bindable(), fromError, toError, hint }: Properties = $props();

	const generatedId = $props.id();
	const hintId = `${generatedId}-range-hint`;
	const errorId = `${generatedId}-range-error`;
	// One message when both days say the same, two when they differ.
	const errors = $derived([...new Set([fromError, toError].filter((error): error is string => Boolean(error)))]);
	const errorIds = $derived(errors.map((_, index) => `${errorId}-${index}`));
	// The hint first, then the field's own error: the order they are read on screen.
	const describedByFor = (error: string | undefined) =>
		[hint ? hintId : undefined, error ? errorIds[errors.indexOf(error)] : undefined].filter(Boolean).join(' ') ||
		undefined;
	const fromDescribedBy = $derived(describedByFor(fromError));
	const toDescribedBy = $derived(describedByFor(toError));
</script>

<div class="range">
	{#if hint}
		<p id={hintId} class="hint">{hint}</p>
	{/if}
	<!-- GOV.UK draws one error for a group of date inputs, above them, under the group's hint. -->
	{#each errors as error, index (error)}
		<p id={errorIds[index]} role="alert" class="error">{error}</p>
	{/each}
	<div class="pair">
		<LabeledField label="From">
			{#snippet children({ id })}
				<TextInput
					{id}
					describedBy={fromDescribedBy}
					invalid={Boolean(fromError)}
					type="date"
					name="from"
					value={from}
					onInput={(value) => (from = value)}
				/>
			{/snippet}
		</LabeledField>
		<LabeledField label="To">
			{#snippet children({ id })}
				<TextInput
					{id}
					describedBy={toDescribedBy}
					invalid={Boolean(toError)}
					type="date"
					name="to"
					value={to}
					onInput={(value) => (to = value)}
				/>
			{/snippet}
		</LabeledField>
	</div>
</div>

<style>
	@layer components {
		/* One item in a filter bar's row of fields: the pair's hint above both
		   boxes, so the two labels and the two boxes share their tops. */
		.range {
			display: flex;
			flex-direction: column;
			gap: var(--space-label);
			min-inline-size: 0;
		}

		.hint {
			margin: 0;
			color: var(--color-on-surface-muted);
			font-size: var(--text-body-sm-size);
		}

		.pair {
			display: flex;
			flex-wrap: wrap;
			align-items: flex-end;
			gap: var(--space-related);
		}

		/* LabeledField's error, in the same words' color and size. */
		.error {
			margin: 0;
			color: var(--color-error);
			font-size: var(--text-body-sm-size);
		}

		/* A message wraps in the pair's width and adds nothing to it, so an
		   error never widens its column or moves the field beside it. */
		.range > .error,
		.range > .hint {
			inline-size: 0;
			min-inline-size: 100%;
		}
	}
</style>
