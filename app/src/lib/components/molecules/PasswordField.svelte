<script lang="ts">
	import CredentialField from './CredentialField.svelte';

	interface Properties {
		id: string;
		value: string;
		onInput: (value: string) => void;
		error?: string;
	}

	let { id, value, onInput, error }: Properties = $props();
</script>

<CredentialField {id} {value} {onInput} {error} label="Password" type="password" autocomplete="current-password" />
