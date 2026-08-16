<script lang="ts">
	import { answerChecked, answerOptions, answerText, type Answers, type Field } from './planInstance.js';

	let { fields, answers }: { fields: Field[]; answers: Answers } = $props();

	function textValue(field: Field): string {
		const value = answerText(answers, field.id);
		return value !== '' ? value : '—';
	}

	function checkboxValue(field: Field): string {
		return answerChecked(answers, field.id) ? 'Yes' : 'No';
	}

	function selectedOptions(field: Field): string {
		const options = answerOptions(answers, field.id);
		return options.length > 0 ? options.join(', ') : '—';
	}
</script>

<dl>
	{#each fields as field (field.id)}
		{#if field.type === 'section_header'}
			<h3>{field.label}</h3>
		{:else}
			<div>
				<dt>{field.label}</dt>
				{#if field.type === 'checkbox'}
					<dd>{checkboxValue(field)}</dd>
				{:else if field.type === 'multi_select'}
					<dd>{selectedOptions(field)}</dd>
				{:else}
					<dd>{textValue(field)}</dd>
				{/if}
			</div>
		{/if}
	{/each}
</dl>
