<script lang="ts">
	import { MERGE_FIELDS } from '#lib/contractTemplate.js';
	import Textarea from '#lib/components/atoms/Textarea.svelte';

	let {
		prose,
		onProseChange
	}: {
		prose: string;
		onProseChange: (prose: string) => void;
	} = $props();
</script>

<label>
	Contract template prose
	<Textarea value={prose} onInput={onProseChange} rows={12} />
</label>

<section>
	<h2>Merge-field placeholders</h2>
	<p>Insert these tokens into the prose above; they are replaced per Engagement.</p>
	<ul>
		<!-- v8 ignore start: Svelte-compiled keyed-each-diffing branch, same
		     class of unreachable-from-interaction-tests branch as
		     DynamicFieldEditor.svelte's bound <select>/<option> pair -->
		{#each MERGE_FIELDS as field (field.token)}
			<li><code>{field.token}</code> &mdash; {field.label}</li>
		{/each}
		<!-- v8 ignore stop -->
	</ul>
</section>
