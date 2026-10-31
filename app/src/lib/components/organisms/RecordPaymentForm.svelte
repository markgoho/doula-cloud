<script lang="ts">
	/**
	 * Recording money that moved outside DoulaCloud, by hand: how it was
	 * paid, an optional note, and the day, then a check step before it is
	 * written (#271, GOV.UK's check answers per ADR-0021). The amount is
	 * never asked: a record always covers the whole sum.
	 *
	 * Two records ask these questions: a Payment a Client made against an
	 * Invoice (InvoiceSection) and a Fee Payment the Practice made to a
	 * contractor Doula (DoulaFeeSection, #2271). onConfirm owns the API
	 * call; a refusal it throws is read back onto the field it names. What
	 * the record said once written is the caller's status line, because
	 * this form is gone by then.
	 */
	import {
		formatAmount,
		paymentMethodLabels,
		paymentMethodOptions,
		type PaymentMethod,
		type RecordPaymentInput
	} from '#lib/invoice.js';
	import { formatCalendarDay } from '#lib/dates.js';
	import { RefusalError, type FormError } from '#lib/formErrors.js';
	import Button from '#lib/components/atoms/Button.svelte';
	import Heading from '#lib/components/atoms/Heading.svelte';
	import TextInput from '#lib/components/atoms/TextInput.svelte';
	import Textarea from '#lib/components/atoms/Textarea.svelte';
	import ErrorSummary from '#lib/components/molecules/ErrorSummary.svelte';
	import LabeledField from '#lib/components/molecules/LabeledField.svelte';
	import RadioGroup, { radioFieldId } from '#lib/components/molecules/RadioGroup.svelte';
	import StackedForm from '#lib/components/molecules/StackedForm.svelte';
	import SummaryList from '#lib/components/molecules/SummaryList.svelte';

	let {
		label,
		heading,
		amountCents,
		dateLabel,
		dateMissingMessage,
		practiceTimezone,
		onConfirm,
		onCancel
	}: {
		/**
		The section's accessible name.
		*/
		label: string;
		heading: string;
		amountCents: number;
		/**
		"Date received" for a Client's Payment, "Date paid" for a fee.
		*/
		dateLabel: string;
		dateMissingMessage: string;
		/** The Practice's own IANA zone (ADR-0036): what "today" is read
		 * in, for the date's default and its future-date ceiling, so both
		 * agree with the BFF's own guard (#1167, #1280). */
		practiceTimezone: string;
		onConfirm: (input: RecordPaymentInput) => Promise<void>;
		onCancel: () => void;
	} = $props();

	/** Today, as TextInput type="date" reads and writes it, in the
	 * Practice's zone: `en-CA` formats as `YYYY-MM-DD`. */
	function todayIsoDate(): string {
		return new Intl.DateTimeFormat('en-CA', { timeZone: practiceTimezone }).format(new Date());
	}

	let step = $state<'form' | 'review'>('form');
	// No method is chosen when the form opens (#1913, ADR-0055): how the
	// money moved is a fact the service does not know.
	let method = $state<PaymentMethod | undefined>();
	let note = $state('');
	// Pre-filled with today on purpose: a recorded departure from GOV.UK's
	// "nothing pre-filled" rule, decided 2026-10-24 on #1923 -- see "A
	// pre-filled answer: Date received" in docs/design/govuk-alignment.md.
	let paidOn = $state(todayIsoDate());
	// The untargeted summary (#1038) for a refusal naming no field here;
	// the three below are read into their field and the summary alike.
	let formError = $state('');
	let methodError = $state('');
	let noteError = $state('');
	let dateError = $state('');
	let isConfirming = $state(false);

	const methodName = 'record-payment-method';
	// A radio group's refusal belongs to the question, so the summary link
	// lands on its first option.
	const methodFieldId = radioFieldId(methodName, paymentMethodOptions[0]!.value);
	const noteFieldId = 'record-payment-note';
	const dateFieldId = 'record-payment-date';

	const errors = $derived<FormError[]>(
		[
			dateError ? { message: dateError, targetId: dateFieldId } : undefined,
			methodError ? { message: methodError, targetId: methodFieldId } : undefined,
			noteError ? { message: noteError, targetId: noteFieldId } : undefined,
			formError ? { message: formError } : undefined
		].filter((entry) => entry !== undefined)
	);

	function resetErrors() {
		formError = '';
		methodError = '';
		noteError = '';
		dateError = '';
	}

	/** A refusal naming `method`, `note` or `paidOn` is read onto that
	 * field, back on the entry step where the field is drawn; anything
	 * else stays the one untargeted summary paragraph. */
	function applyRefusal(error_: unknown) {
		resetErrors();
		if (error_ instanceof RefusalError && error_.details) {
			methodError = error_.details.method ?? '';
			noteError = error_.details.note ?? '';
			dateError = error_.details.paidOn ?? '';
			if (methodError || noteError || dateError) {
				step = 'form';
				return;
			}
		}
		formError = error_ instanceof Error ? error_.message : 'Failed to record payment';
	}

	/**
	The BFF's own checks, before the review step, each on its field.
	*/
	function review(event: SubmitEvent) {
		event.preventDefault();
		resetErrors();
		if (method === undefined) {
			methodError = 'Select how the payment was made';
			return;
		}
		if (paidOn.trim() === '') {
			dateError = dateMissingMessage;
			return;
		}
		if (method === 'other' && note.trim() === '') {
			noteError = 'Enter a note for "Other"';
			return;
		}
		if (paidOn > todayIsoDate()) {
			dateError = 'The date cannot be in the future';
			return;
		}
		step = 'review';
	}

	async function confirm() {
		isConfirming = true;
		resetErrors();
		try {
			await onConfirm({
				// Set: review refuses an empty method before this step.
				method: method!,
				note: note.trim() === '' ? undefined : note.trim(),
				paidOn
			});
		} catch (error_) {
			applyRefusal(error_);
		} finally {
			isConfirming = false;
		}
	}
</script>

<section aria-label={label}>
	<Heading level={3} text={heading} />
	<!-- Above the step switch: an untargeted refusal leaves the review step
	     up, and this is still where it shows (#1228). -->
	{#if errors.length > 0}
		<ErrorSummary {errors} />
	{/if}
	{#if step === 'form'}
		<StackedForm onSubmit={review}>
			<RadioGroup
				legend="Method"
				name={methodName}
				options={paymentMethodOptions}
				value={method}
				onChange={(value) => (method = value)}
				error={methodError || undefined}
			/>
			<LabeledField id={noteFieldId} label="Note (optional)" error={noteError || undefined}>
				{#snippet children({ id, describedBy, invalid })}
					<Textarea {id} {describedBy} {invalid} value={note} onInput={(value) => (note = value)} />
				{/snippet}
			</LabeledField>
			<LabeledField id={dateFieldId} label={dateLabel} error={dateError || undefined}>
				{#snippet children({ id, describedBy, invalid })}
					<TextInput
						{id}
						{describedBy}
						{invalid}
						type="date"
						value={paidOn}
						onInput={(value) => (paidOn = value)}
						required
					/>
				{/snippet}
			</LabeledField>
			<Button label="Continue" type="submit" />
			<Button label="Cancel" variant="secondary" onClick={onCancel} />
		</StackedForm>
	{:else}
		<SummaryList
			rows={[
				{ label: 'Amount', value: formatAmount(amountCents) },
				{ label: 'Method', value: paymentMethodLabels[method!] },
				{ label: 'Note', value: note.trim() === '' ? '—' : note },
				// A calendar day built from its parts, never parsed as UTC
				// midnight, which a recorder behind UTC would read a day early.
				{ label: dateLabel, value: formatCalendarDay(paidOn) }
			]}
		/>
		<Button label="Confirm and record" onClick={confirm} loading={isConfirming} />
		<Button label="Change" variant="secondary" onClick={() => (step = 'form')} />
	{/if}
</section>
