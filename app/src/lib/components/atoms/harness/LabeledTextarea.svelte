<!--
	Test support, not a component anybody ships.

	`LabeledField` hands its control a Snippet, and a Snippet cannot be
	built from a spec without `createRawSnippet`, which renders a string of
	HTML rather than a component. This is the only way to assert that the
	real `Textarea` and the real `LabeledField` agree about `id`,
	`describedBy` and `invalid`. It lives in a subdirectory so the style
	guide's retrofit gate, which reads the tier directories themselves,
	does not ask for a page for it.
-->
<script lang="ts">
	import Textarea from '../Textarea.svelte';
	import LabeledField from '../../molecules/LabeledField.svelte';

	let {
		label,
		hint,
		error,
		maxLength
	}: {
		label: string;
		hint?: string;
		error?: string;
		maxLength?: number;
	} = $props();

	let value = $state('');
</script>

<LabeledField {label} {hint} {error}>
	{#snippet children({ id, describedBy, invalid })}
		<Textarea {id} {describedBy} {invalid} {maxLength} {value} onInput={(next) => (value = next)} />
	{/snippet}
</LabeledField>
