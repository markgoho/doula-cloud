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
		 * What the To day is for, under its label: whether it is itself part
		 * of the range ("This day is included."). Absent, no hint is drawn.
		 */
		toHint?: string;
	}

	let { from = $bindable(), to = $bindable(), fromError, toError, toHint }: Properties = $props();
</script>

<LabeledField label="From" error={fromError}>
	{#snippet children({ id, describedBy, invalid })}
		<TextInput {id} {describedBy} {invalid} type="date" name="from" value={from} onInput={(value) => (from = value)} />
	{/snippet}
</LabeledField>
<LabeledField label="To" hint={toHint} error={toError}>
	{#snippet children({ id, describedBy, invalid })}
		<TextInput {id} {describedBy} {invalid} type="date" name="to" value={to} onInput={(value) => (to = value)} />
	{/snippet}
</LabeledField>
