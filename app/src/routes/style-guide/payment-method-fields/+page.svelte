<script module lang="ts">
	import type { DemoVariant } from '../drag-surface/dragSurface.js';

	interface Properties {
		isRefused?: boolean;
	}

	export const variants: readonly DemoVariant<Properties>[] = [
		{ name: 'Payment method fields, refused', props: { isRefused: true } }
	];
</script>

<script lang="ts">
	import type { PaymentMethod } from '#lib/invoice.js';
	import PaymentMethodFields from '#lib/components/molecules/PaymentMethodFields.svelte';
	import StackedForm from '#lib/components/molecules/StackedForm.svelte';

	let { isRefused = false }: Properties = $props();

	let method = $state<PaymentMethod | undefined>();
	let note = $state('');
</script>

<StackedForm onSubmit={(event) => event.preventDefault()}>
	<PaymentMethodFields
		legend="How was the money refunded?"
		methodName="demo-method"
		{method}
		onMethodChange={(value) => (method = value)}
		methodError={isRefused ? 'Select how the money was refunded' : undefined}
		noteFieldId="demo-note"
		{note}
		onNoteInput={(value) => (note = value)}
		noteError={isRefused ? 'Enter a note for "Other"' : undefined}
	/>
</StackedForm>
