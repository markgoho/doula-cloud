<script lang="ts">
	// PROTOTYPE (#372) -- throwaway. The Practice-defined layer (ADR-0017),
	// rendered live from the Practice's current template. Every variant renders
	// it the same way; what they disagree about is where it sits.
	import TextInput from '#lib/components/atoms/TextInput.svelte';
	import Checkbox from '#lib/components/atoms/Checkbox.svelte';
	import LabeledField from '#lib/components/molecules/LabeledField.svelte';
	import { practiceFields } from './fixtures.js';

	interface Properties {
		values: Record<string, string | boolean>;
		onChange: (id: string, value: string | boolean) => void;
	}

	let { values, onChange }: Properties = $props();
</script>

<stack-l>
	{#each practiceFields as field (field.id)}
		{#if field.type === 'checkbox'}
			<LabeledField label={field.label} orientation="inline">
				{#snippet children({ id })}
					<Checkbox
						{id}
						checked={Boolean(values[field.id])}
						onChange={(checked: boolean) => onChange(field.id, checked)}
					/>
				{/snippet}
			</LabeledField>
		{:else if field.type === 'single_select'}
			<LabeledField label={field.label}>
				{#snippet children({ id, describedBy })}
					<select
						{id}
						aria-describedby={describedBy}
						value={String(values[field.id] ?? '')}
						onchange={(event) => onChange(field.id, event.currentTarget.value)}
					>
						<option value="">Choose one</option>
						{#each field.options ?? [] as option (option)}
							<option value={option}>{option}</option>
						{/each}
					</select>
				{/snippet}
			</LabeledField>
		{:else if field.type === 'long_text'}
			<LabeledField label={field.label}>
				{#snippet children({ id, describedBy })}
					<textarea
						{id}
						aria-describedby={describedBy}
						rows="3"
						value={String(values[field.id] ?? '')}
						oninput={(event) => onChange(field.id, event.currentTarget.value)}
					></textarea>
				{/snippet}
			</LabeledField>
		{:else}
			<LabeledField label={field.label}>
				{#snippet children({ id, describedBy, invalid })}
					<TextInput
						{id}
						{describedBy}
						{invalid}
						value={String(values[field.id] ?? '')}
						onInput={(value) => onChange(field.id, value)}
					/>
				{/snippet}
			</LabeledField>
		{/if}
	{/each}
</stack-l>

<style>
	textarea {
		inline-size: 100%;
		font: inherit;
	}
</style>
