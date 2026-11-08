<script lang="ts">
	import Button from '../atoms/Button.svelte';
	import Heading from '../atoms/Heading.svelte';
	import WarningText from '../atoms/WarningText.svelte';

	interface Properties {
		warning: string;
		isSubmitting: boolean;
		onConfirm: () => void;
		onCancel: () => void;
	}

	let { warning, isSubmitting, onConfirm, onCancel }: Properties = $props();
</script>

<Heading level={2} text="Before you continue" focusOnAppearing />
<WarningText message={warning} />
<Button type="button" label="Continue and sign out" loading={isSubmitting} onClick={onConfirm} />
<Button type="button" label="Cancel" variant="secondary" onClick={onCancel} />
