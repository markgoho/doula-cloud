<script lang="ts">
	import { MERGE_FIELDS } from '#lib/contractTemplate.js';
	import Heading from '#lib/components/atoms/Heading.svelte';
	import Textarea from '#lib/components/atoms/Textarea.svelte';
	import LabeledField from '#lib/components/molecules/LabeledField.svelte';

	/*
	 * #2244: `onProseChange` is how the prose can change, so a reader who
	 * may not change it is handed none, and gets the terms as text rather
	 * than a field whose typing nothing would ever save. The legend stays
	 * for her: the tokens are in the terms she reads.
	 */
	let {
		prose,
		onProseChange
	}: {
		prose: string;
		onProseChange?: (prose: string) => void;
	} = $props();
</script>

{#if onProseChange}
	<LabeledField label="Contract template prose">
		{#snippet children({ id, describedBy, invalid })}
			<Textarea {id} {describedBy} {invalid} value={prose} onInput={onProseChange} rows={12} />
		{/snippet}
	</LabeledField>
{:else}
	<div class="prose">{prose}</div>
{/if}

<section>
	<Heading level={2} text="Merge-field placeholders" />
	{#if onProseChange}
		<p>Insert these tokens into the prose above; they are replaced per Engagement.</p>
	{:else}
		<p>Each Contract fills these in for its Client.</p>
	{/if}
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

<style>
	@layer components {
		/* The Practice's own line breaks are the terms' paragraphs, and a
		   pasted link breaks rather than widening the page, as a
		   Contract's own prose does (`ContractView`). */
		.prose {
			max-inline-size: var(--measure);
			white-space: pre-wrap;
			overflow-wrap: anywhere;
		}
	}
</style>
