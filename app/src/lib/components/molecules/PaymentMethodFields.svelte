<script lang="ts">
	/*
	 * How money moved and a note about it: the two questions the Practice
	 * answers when it records a payment and when it returns one
	 * (`RecordPaymentForm`, `RefundPaymentForm`). The legend is each form's
	 * own words for the method question.
	 */
	import { paymentMethodOptions, type PaymentMethod } from '#lib/invoice.js';
	import Textarea from '#lib/components/atoms/Textarea.svelte';
	import LabeledField from './LabeledField.svelte';
	import RadioGroup from './RadioGroup.svelte';

	interface Properties {
		legend: string;
		/**
		The stem of the method's radio ids.
		*/
		methodName: string;
		method: PaymentMethod | undefined;
		onMethodChange: (method: PaymentMethod) => void;
		methodError?: string;
		noteFieldId: string;
		note: string;
		onNoteInput: (note: string) => void;
		noteError?: string;
	}

	let {
		legend,
		methodName,
		method,
		onMethodChange,
		methodError,
		noteFieldId,
		note,
		onNoteInput,
		noteError
	}: Properties = $props();
</script>

<RadioGroup
	{legend}
	name={methodName}
	options={paymentMethodOptions}
	value={method}
	onChange={onMethodChange}
	error={methodError}
/>
<LabeledField id={noteFieldId} label="Note (optional)" error={noteError}>
	{#snippet children({ id, describedBy, invalid })}
		<Textarea {id} {describedBy} {invalid} value={note} onInput={onNoteInput} />
	{/snippet}
</LabeledField>
