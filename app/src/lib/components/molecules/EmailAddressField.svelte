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

<LabeledField {id} {error} label="Email address">
	{#snippet children({ id: controlId, describedBy, invalid })}
		<TextInput
			id={controlId}
			{describedBy}
			{invalid}
			type="email"
			{value}
			{onInput}
			required
			autocomplete="username"
		/>
	{/snippet}
</LabeledField>
