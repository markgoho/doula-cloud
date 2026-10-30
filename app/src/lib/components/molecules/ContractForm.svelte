<script lang="ts">
	import { mergeFieldLabel } from '#lib/contract.js';
	import { DATE_MERGE_FIELDS } from '#lib/contractTemplate.js';
	import TextInput from '#lib/components/atoms/TextInput.svelte';
	import LabeledField from './LabeledField.svelte';

	let {
		mergeFields,
		values,
		readOnly,
		onValueChange
	}: {
		mergeFields: string[];
		values: Record<string, string>;
		readOnly: boolean;
		onValueChange: (key: string, value: string) => void;
	} = $props();
</script>

<!-- A run of fields, not a list (#2017): StackedForm's own gap between them. -->
<stack-l space="var(--space-5)">
	{#each mergeFields as key (key)}
		<!-- v8 ignore start: Svelte's compiled null-guard on this label text is unreachable -- mergeFieldLabel always returns a string -->
		<LabeledField label={mergeFieldLabel(key)}>
			<!-- v8 ignore stop -->
			{#snippet children({ id, describedBy, invalid })}
				<!-- A date field is a day being chosen, so the native control asks for it (#2222):
				     it stores YYYY-MM-DD, which fillProse prints as the product prints a calendar day. -->
				<TextInput
					{id}
					{describedBy}
					{invalid}
					type={DATE_MERGE_FIELDS.has(key) ? 'date' : 'text'}
					value={values[key] ?? ''}
					disabled={readOnly}
					onInput={(value) => onValueChange(key, value)}
				/>
			{/snippet}
		</LabeledField>
	{/each}
</stack-l>
