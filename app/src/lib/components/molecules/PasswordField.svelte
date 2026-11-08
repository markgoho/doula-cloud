<script lang="ts">
	import TextInput from '#lib/components/atoms/TextInput.svelte';
	import LabeledField from './LabeledField.svelte';

	interface Properties {
		id: string;
		value: string;
		onInput: (value: string) => void;
		error?: string;
	}

	let { id, value, onInput, error }: Properties = $props();
</script>

<LabeledField {id} {error} label="Password">
	{#snippet children({ id: controlId, describedBy, invalid })}
		<TextInput
			id={controlId}
			{describedBy}
			{invalid}
			type="password"
			{value}
			{onInput}
			required
			autocomplete="current-password"
		/>
	{/snippet}
</LabeledField>
