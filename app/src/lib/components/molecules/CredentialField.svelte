<script lang="ts">
	/*
	 * A labeled text input for what a person types to sign in: the wiring
	 * `EmailAddressField` and `PasswordField` share. They own the label,
	 * the input type and the autocomplete token; this owns the rest.
	 */
	import TextInput from '#lib/components/atoms/TextInput.svelte';
	import LabeledField from './LabeledField.svelte';

	interface Properties {
		id: string;
		label: string;
		type: 'email' | 'password';
		autocomplete: 'username' | 'current-password';
		value: string;
		onInput: (value: string) => void;
		error?: string;
	}

	let { id, label, type, autocomplete, value, onInput, error }: Properties = $props();
</script>

<LabeledField {id} {error} {label}>
	{#snippet children({ id: controlId, describedBy, invalid })}
		<TextInput id={controlId} {describedBy} {invalid} {type} {value} {onInput} required {autocomplete} />
	{/snippet}
</LabeledField>
