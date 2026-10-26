<script lang="ts">
	import { MERGE_FIELDS } from '#lib/contractTemplate.js';
	import Heading from '#lib/components/atoms/Heading.svelte';
	import Textarea from '#lib/components/atoms/Textarea.svelte';
	import LabeledField from '#lib/components/molecules/LabeledField.svelte';

	let {
		prose,
		onProseChange
	}: {
		prose: string;
		onProseChange: (prose: string) => void;
	} = $props();
</script>

<LabeledField label="Contract template prose">
	{#snippet children({ id, describedBy, invalid })}
		<Textarea {id} {describedBy} {invalid} value={prose} onInput={onProseChange} rows={12} />
	{/snippet}
</LabeledField>

<section>
	<Heading level={2} text="Merge-field placeholders" />
	<p>Insert these tokens into the prose above; they are replaced per Engagement.</p>
	<ul>
		<!-- v8 ignore start: Svelte-compiled keyed-each-diffing branch, same
		     class of branch no interaction test reaches as any keyed each
		     whose list never changes after mount -->
		{#each MERGE_FIELDS as field (field.token)}
			<li><code>{field.token}</code> &mdash; {field.label}</li>
		{/each}
		<!-- v8 ignore stop -->
	</ul>
</section>
