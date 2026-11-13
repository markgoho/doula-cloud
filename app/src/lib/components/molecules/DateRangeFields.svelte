<script lang="ts">
	import TextInput from '../atoms/TextInput.svelte';
	import LabeledField from './LabeledField.svelte';

	interface Properties {
		from: string;
		to: string;
		/**
		 * What is wrong with the From day, beside its box. Absent where the
		 * screen sends whatever is typed.
		 */
		fromError?: string;
		/**
		 * What is wrong with the To day, beside its box.
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
	const describedByWithHint = (describedBy: string | undefined) =>
		[hint ? hintId : undefined, describedBy].filter(Boolean).join(' ') || undefined;
</script>

<div class="range">
	{#if hint}
		<p id={hintId} class="hint">{hint}</p>
	{/if}
	<div class="pair">
		<LabeledField label="From" error={fromError}>
			{#snippet children({ id, describedBy, invalid })}
				<TextInput
					{id}
					describedBy={describedByWithHint(describedBy)}
					{invalid}
					type="date"
					name="from"
					value={from}
					onInput={(value) => (from = value)}
				/>
			{/snippet}
		</LabeledField>
		<LabeledField label="To" error={toError}>
			{#snippet children({ id, describedBy, invalid })}
				<TextInput
					{id}
					describedBy={describedByWithHint(describedBy)}
					{invalid}
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

		/* The boxes sit on one baseline of their own, whatever an error above one of them adds. */
		.pair {
			display: flex;
			flex-wrap: wrap;
			align-items: flex-end;
			gap: var(--space-related);
		}

		/* A message wraps in its own box's width and adds nothing to it, so an
		   error never widens its column or moves the field beside it. */
		.pair :global(p[role='alert']),
		.range > .hint {
			inline-size: 0;
			min-inline-size: 100%;
		}
	}
</style>
