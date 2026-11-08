<script lang="ts">
	import TextInput from '../atoms/TextInput.svelte';
	import LabeledField from './LabeledField.svelte';

	interface Properties {
		from: string;
		to: string;
	}

	let { from = $bindable(), to = $bindable() }: Properties = $props();
</script>

<LabeledField label="From">
	{#snippet children({ id, describedBy, invalid })}
		<TextInput {id} {describedBy} {invalid} type="date" name="from" value={from} onInput={(value) => (from = value)} />
	{/snippet}
</LabeledField>
<LabeledField label="To">
	{#snippet children({ id, describedBy, invalid })}
		<TextInput {id} {describedBy} {invalid} type="date" name="to" value={to} onInput={(value) => (to = value)} />
	{/snippet}
</LabeledField>
