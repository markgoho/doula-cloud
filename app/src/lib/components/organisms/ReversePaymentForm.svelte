<script lang="ts">
	/**
	 * Undoing a payment recorded by mistake (#945): a reason, then a check
	 * step, then an additive reversal the BFF writes beside the original,
	 * never an edit of it. Asked for a Client's Payment (InvoiceSection) and
	 * for a Doula's Fee Payment (DoulaFeeSection, #2271). onConfirm owns the
	 * API call; a refusal naming `reason` is read onto the Reason field.
	 */
	import { formatAmount } from '#lib/invoice.js';
	import { RefusalError } from '#lib/formErrors.js';
	import Button from '#lib/components/atoms/Button.svelte';
	import Heading from '#lib/components/atoms/Heading.svelte';
	import Textarea from '#lib/components/atoms/Textarea.svelte';
	import LabeledField from '#lib/components/molecules/LabeledField.svelte';
	import StackedForm from '#lib/components/molecules/StackedForm.svelte';
	import SummaryList from '#lib/components/molecules/SummaryList.svelte';

	let {
		label,
		heading,
		amountCents,
		reasonMissingMessage,
		confirmLabel,
		onConfirm,
		onCancel
	}: {
		/**
		The section's accessible name.
		*/
		label: string;
		heading: string;
		amountCents: number;
		reasonMissingMessage: string;
		confirmLabel: string;
		onConfirm: (reason: string) => Promise<void>;
		onCancel: () => void;
	} = $props();

	let step = $state<'form' | 'review'>('form');
	let reason = $state('');
	// formError is the untargeted fallback (#1038): a refusal naming no
	// field, such as one already reversed. reasonError is the field's own.
	let formError = $state('');
	let reasonError = $state('');
	let isConfirming = $state(false);

	function resetErrors() {
		formError = '';
		reasonError = '';
	}

	function applyRefusal(error_: unknown) {
		resetErrors();
		if (error_ instanceof RefusalError && error_.details?.reason) {
			reasonError = error_.details.reason;
			step = 'form';
			return;
		}
		formError = error_ instanceof Error ? error_.message : 'Failed to reverse this payment';
	}

	function review(event: SubmitEvent) {
		event.preventDefault();
		resetErrors();
		if (reason.trim() === '') {
			reasonError = reasonMissingMessage;
			return;
		}
		step = 'review';
	}

	async function confirm() {
		isConfirming = true;
		resetErrors();
		try {
			await onConfirm(reason.trim());
		} catch (error_) {
			applyRefusal(error_);
		} finally {
			isConfirming = false;
		}
	}
</script>

<section aria-label={label}>
	<Heading level={3} text={heading} />
	{#if step === 'form'}
		<StackedForm onSubmit={review}>
			<LabeledField label="Reason" error={reasonError || undefined}>
				{#snippet children({ id, describedBy, invalid })}
					<Textarea {id} {describedBy} {invalid} value={reason} onInput={(value) => (reason = value)} />
				{/snippet}
			</LabeledField>
			<Button label="Continue" type="submit" />
			<Button label="Cancel" variant="secondary" onClick={onCancel} />
		</StackedForm>
	{:else}
		<SummaryList
			rows={[
				{ label: 'Amount', value: formatAmount(amountCents) },
				{ label: 'Reason', value: reason }
			]}
		/>
		<Button label={confirmLabel} onClick={confirm} loading={isConfirming} />
		<Button label="Change" variant="secondary" onClick={() => (step = 'form')} />
	{/if}
	{#if formError}
		<p role="alert">{formError}</p>
	{/if}
</section>
