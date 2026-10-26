<script lang="ts">
	import { mergeFieldLabel } from '#lib/contract.js';
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
				<TextInput
					{id}
					{describedBy}
					{invalid}
					value={values[key] ?? ''}
					disabled={readOnly}
					onInput={(value) => onValueChange(key, value)}
				/>
			{/snippet}
		</LabeledField>
	{/each}
</stack-l>
