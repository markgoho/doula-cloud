<script lang="ts">
	/**
	 * Returning money a Client paid (#1009) -- the form an Owner or Admin
	 * fills in, then checks, before a Refund is recorded or issued. Its own
	 * organism rather than a third inline flow in InvoiceSection: the
	 * questions it asks depend on which Payment it returns, which the other
	 * two flows never had to branch on.
	 *
	 * Two Payments, two sets of questions:
	 *
	 * - One recorded by hand ('manual') was returned by hand too, so the
	 *   form asks how -- the same four methods and optional note as
	 *   recording one. That holds on a Stripe-backed Invoice as well: a
	 *   check recorded there with paid_out_of_band never passed through
	 *   Stripe, so Stripe cannot send it back.
	 * - A card Payment Stripe collected ('stripe') is returned by Stripe,
	 *   so there is no method to ask. What she has to be told instead,
	 *   before she confirms, is that Stripe keeps its processing fee and her
	 *   account bears it -- #1009's criterion -- and the amount she typed is
	 *   never adjusted to make up for it.
	 *
	 * The amount is hers to type, up to what is left to return: a Practice
	 * that keeps a canceled Engagement's first two visits returns the rest.
	 * The Invoice stays 'paid' either way, and this form never says
	 * otherwise.
	 *
	 * Same two steps as recording or reversing a Payment (GOV.UK's check
	 * answers, per ADR-0021): fill in, then confirm. onConfirm owns the API
	 * call; this component only reads what was entered and shows whatever
	 * the call throws.
	 */
	import {
		formatAmount,
		paymentMethodLabels,
		paymentMethodOptions,
		stripeRefundFeeWarning,
		type Invoice,
		type PaymentMethod,
		type RefundPaymentInput
	} from '#lib/invoice.js';
	import { RefusalError, type FormError } from '#lib/formErrors.js';
	import Button from '#lib/components/atoms/Button.svelte';
	import TextInput from '#lib/components/atoms/TextInput.svelte';
	import Textarea from '#lib/components/atoms/Textarea.svelte';
	import WarningText from '#lib/components/atoms/WarningText.svelte';
	import ErrorSummary from '#lib/components/molecules/ErrorSummary.svelte';
	import LabeledField from '#lib/components/molecules/LabeledField.svelte';
	import RadioGroup, { radioFieldId } from '#lib/components/molecules/RadioGroup.svelte';
	import StackedForm from '#lib/components/molecules/StackedForm.svelte';
	import SummaryList from '#lib/components/molecules/SummaryList.svelte';

	let {
		invoice,
		onConfirm,
		onCancel
	}: {
		/**
		The paid Invoice whose covering Payment is being returned.
		*/
		invoice: Invoice;
		onConfirm: (input: RefundPaymentInput) => Promise<void>;
		onCancel: () => void;
	} = $props();

	const isCardPayment = $derived(invoice.activePaymentKind === 'stripe');
	/** A Payment always covers its Invoice in full (#271), so what is left
	 * to return is the Invoice's amount less every Refund against it. */
	const remainingCents = $derived(invoice.amountCents - invoice.refundedCents);

	let step = $state<'form' | 'review'>('form');
	let amountDollars = $state('');
	/** The amount in cents as it passed review -- what the review step
	 * shows and what confirm sends, so the two can never disagree. */
	let reviewedCents = $state(0);
	// No method is chosen when the form opens (#1913, ADR-0055): how the
	// money went back is a fact the service does not know.
	let method = $state<PaymentMethod | undefined>();
	let note = $state('');
	// The untargeted summary (#1038) for a refusal naming no field here --
	// a Stripe failure, or more than is left once another Refund landed
	// first; the three below carry one that does.
	let error = $state('');
	let amountError = $state('');
	let methodError = $state('');
	let noteError = $state('');
	let isSubmitting = $state(false);

	const amountFieldId = 'refund-payment-amount';
	const methodName = 'refund-payment-method';
	// The first option's id, as InvoiceSection's Method does: a radio
	// group's refusal belongs to the question, so the summary link lands
	// on the first option whichever one is checked.
	const methodFieldId = radioFieldId(methodName, paymentMethodOptions[0]!.value);
	const noteFieldId = 'refund-payment-note';

	// One GOV.UK error summary for the form (#1913), built from the same
	// strings the fields show, in field order, with the untargeted refusal
	// as its one unlinked entry -- the shape InvoiceSection's
	// paymentFormErrors has.
	const formErrors = $derived<FormError[]>(
		[
			amountError ? { message: amountError, targetId: amountFieldId } : undefined,
			methodError ? { message: methodError, targetId: methodFieldId } : undefined,
			noteError ? { message: noteError, targetId: noteFieldId } : undefined,
			error ? { message: error } : undefined
		].filter((entry) => entry !== undefined)
	);

	function resetErrors() {
		error = '';
		amountError = '';
		methodError = '';
		noteError = '';
	}

	/** The typed amount in cents, or undefined when it is not a positive
	 * number. Rounded, never truncated: 19.99 * 100 is 1998.9999... */
	function amountCents(): number | undefined {
		const dollars = Number(amountDollars);
		if (amountDollars.trim() === '' || !Number.isFinite(dollars) || dollars <= 0) {
			return undefined;
		}
		return Math.round(dollars * 100);
	}

	/** The same checks the BFF makes, each on the field it is about, so a
	 * refusal arrives before she confirms rather than after. */
	function review(event: SubmitEvent) {
		event.preventDefault();
		resetErrors();
		const cents = amountCents();
		if (cents === undefined) {
			amountError = 'Enter an amount to return greater than $0.00';
			return;
		}
		if (cents > remainingCents) {
			amountError = `Amount to return must be ${formatAmount(remainingCents)} or less`;
			return;
		}
		if (!isCardPayment && method === undefined) {
			methodError = 'Select how the money was returned';
			return;
		}
		if (!isCardPayment && method === 'other' && note.trim() === '') {
			noteError = 'Enter a note for "Other"';
			return;
		}
		reviewedCents = cents;
		step = 'review';
	}

	/** Reads a refusal onto the field it names (`amountCents`, `method`,
	 * `note` -- PostRefundPaymentHandler's own details keys), or into the
	 * summary when it names none. */
	function applyRefusal(error_: unknown) {
		resetErrors();
		if (error_ instanceof RefusalError && error_.details) {
			amountError = error_.details.amountCents ?? '';
			methodError = error_.details.method ?? '';
			noteError = error_.details.note ?? '';
			if (amountError || methodError || noteError) {
				step = 'form';
				return;
			}
		}
		error = error_ instanceof Error ? error_.message : 'Failed to return this money';
	}

	async function confirm() {
		isSubmitting = true;
		resetErrors();
		try {
			await onConfirm(
				isCardPayment
					? { amountCents: reviewedCents }
					: // Set: review refuses an empty method before this step.
						{ amountCents: reviewedCents, method: method!, note: note.trim() === '' ? undefined : note.trim() }
			);
		} catch (error_) {
			applyRefusal(error_);
		} finally {
			isSubmitting = false;
		}
	}
</script>

<section aria-label="Return money to the Client">
	<!-- v8 ignore start: Svelte's compiled null-guard on this text node is unreachable -- formatAmount always returns a string -->
	<h3>Return money from a payment of {formatAmount(invoice.amountCents)}</h3>
	<!-- v8 ignore stop -->
	<!-- Above the step switch, as on InvoiceSection's payment form: an
	     untargeted refusal leaves the review step on screen, and it has to
	     show there too. One region for the whole form. -->
	{#if formErrors.length > 0}
		<ErrorSummary errors={formErrors} />
	{/if}
	{#if step === 'form'}
		<StackedForm onSubmit={review}>
			<LabeledField
				id={amountFieldId}
				label="Amount to return (USD)"
				hint="Up to {formatAmount(remainingCents)} is left to return on this payment."
				error={amountError || undefined}
				width={10}
			>
				<!-- Money to the cent, the same width and "$" as the Practice rates (#1216). -->
				{#snippet children({ id, describedBy, invalid, width })}
					<TextInput
						{id}
						{describedBy}
						{invalid}
						{width}
						prefix="$"
						type="number"
						step={0.01}
						inputmode="decimal"
						value={amountDollars}
						onInput={(value) => (amountDollars = value)}
					/>
				{/snippet}
			</LabeledField>
			{#if !isCardPayment}
				<RadioGroup
					legend="How was the money returned?"
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
			{/if}
			<Button label="Continue" type="submit" />
			<Button label="Cancel" variant="secondary" onClick={onCancel} />
		</StackedForm>
	{:else}
		<SummaryList
			rows={isCardPayment
				? [
						{ label: 'Amount to return', value: formatAmount(reviewedCents) },
						{ label: 'Returned by', value: 'Stripe, to the card that paid' }
					]
				: [
						{ label: 'Amount to return', value: formatAmount(reviewedCents) },
						{ label: 'Method', value: paymentMethodLabels[method!] },
						{ label: 'Note', value: note.trim() === '' ? '—' : note }
					]}
		/>
		{#if isCardPayment}
			<WarningText message={stripeRefundFeeWarning} />
		{/if}
		<Button label="Confirm and return money" onClick={confirm} loading={isSubmitting} />
		<Button label="Change" variant="secondary" onClick={() => (step = 'form')} />
	{/if}
</section>
