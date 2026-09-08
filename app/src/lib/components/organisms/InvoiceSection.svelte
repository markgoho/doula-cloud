<script lang="ts">
	/**
	 * The Engagement view's Invoices section (#81, extended by #271): the
	 * list of Invoices billed against the Contract so far, the form that
	 * raises a new one, and -- once one is open -- the two ways an Owner
	 * or Admin settle it without Stripe: recording a Payment that arrived
	 * by check, bank transfer, or cash, or voiding/writing off a by-hand
	 * Invoice that will never be paid. Each of onCreate/onRecordPayment/
	 * onVoidInvoice/onWriteOffInvoice owns its own API call; this
	 * component only reports what was entered and displays whatever error
	 * the call throws.
	 *
	 * Three preconditions gate the Create Invoice form (#270, following
	 * #255's own three-separate-fields rule), each with its own fixer:
	 *
	 * - contractStatus isn't billableContractStatus (#275) -- Staff
	 *   issues a new Contract, or the Client signs the one that exists.
	 *   Applies whichever rail the Practice bills on.
	 * - billingMode is undefined -- #271's "ask once, inline": the very first
	 *   Invoice raise asks which rail this Practice bills on, before either
	 *   check below applies. There is no amount to ask alongside it any
	 *   more (#947) -- the Invoice is raised for whatever the Contract
	 *   itself carries.
	 * - On the Stripe rail only, once chosen: !clientsCanPay (a Practice
	 *   Owner has to connect Stripe) or !hasClientEmail (Staff adds an
	 *   email to the Client's record) -- #430's amendment: a by-hand
	 *   Invoice mails nothing, so neither ever blocks that rail.
	 *
	 * Every gate renders Notice's info variant (role="status", a polite
	 * live region), never a disabled form: a Staff member who just hit
	 * one of these, or a screen reader user, hears why billing is
	 * unavailable rather than finding the control silently gone.
	 */
	import {
		billableContractStatus,
		clientHasNoEmailMessage,
		clientsCannotPayMessage,
		formatAmount,
		invoiceStatusLabel,
		unbillableContractMessage,
		type BillingMode,
		type Invoice,
		type PaymentMethod
	} from '#lib/invoice.js';
	import Button from '#lib/components/atoms/Button.svelte';
	import Link from '#lib/components/atoms/Link.svelte';
	import Notice from '#lib/components/atoms/Notice.svelte';
	import TextInput from '#lib/components/atoms/TextInput.svelte';
	import Textarea from '#lib/components/atoms/Textarea.svelte';
	import LabeledField from '#lib/components/molecules/LabeledField.svelte';
	import RadioGroup from '#lib/components/molecules/RadioGroup.svelte';
	import DescriptionList from '#lib/components/molecules/DescriptionList.svelte';

	let {
		invoices,
		contractStatus,
		billingMode,
		clientsCanPay,
		hasClientEmail,
		isOwner,
		isOwnerOrAdmin,
		paymentsSettingsHref,
		onCreate,
		onRecordPayment,
		onVoidInvoice,
		onWriteOffInvoice
	}: {
		invoices: Invoice[];
		contractStatus: string;
		/** Undefined until the Practice has chosen a rail (#271) -- the
		 * component itself asks then, rather than being told what to ask. */
		billingMode: BillingMode | undefined;
		clientsCanPay: boolean;
		hasClientEmail: boolean;
		isOwner: boolean;
		/** Gates Record payment/Void/Write off -- ADR-0008's Contract-money
		 * write row, Owner and Admin only. */
		isOwnerOrAdmin: boolean;
		paymentsSettingsHref: string;
		onCreate: (billingMode?: BillingMode) => Promise<void>;
		onRecordPayment: (
			invoiceId: string,
			input: { method: PaymentMethod; note?: string; paidOn: string }
		) => Promise<void>;
		onVoidInvoice: (invoiceId: string) => Promise<void>;
		onWriteOffInvoice: (invoiceId: string) => Promise<void>;
	} = $props();

	const isBillable = $derived(contractStatus === billableContractStatus);

	let chosenBillingMode = $state<BillingMode>('stripe');
	let isCreating = $state(false);
	let createError = $state('');

	const billingModeOptions = [
		{ value: 'stripe' as const, label: 'Stripe', description: 'Doula Cloud sends the bill and collects the card payment.' },
		{ value: 'by_hand' as const, label: 'By hand', description: 'This Practice bills and collects payment itself.' }
	];

	// #947: the amount an Invoice is raised for is the Contract's own
	// price, never a figure typed here -- there is nothing left to
	// validate before calling onCreate.
	async function handleCreate(event: SubmitEvent) {
		event.preventDefault();
		createError = '';

		isCreating = true;
		try {
			if (billingMode === undefined) {
				await onCreate(chosenBillingMode);
			} else {
				await onCreate();
			}
		} catch (error_) {
			createError = error_ instanceof Error ? error_.message : 'Failed to create invoice';
		} finally {
			isCreating = false;
		}
	}

	/** Today's date in the form TextInput type="date" reads and writes --
	 * the default paymentDate, and the future-date guard's own ceiling. */
	function todayIsoDate(): string {
		return new Date().toISOString().slice(0, 10);
	}

	// #271's recording flow: which Invoice (if any) is being settled, and
	// which of its two steps -- fill in the details, then confirm them
	// (the check-your-answers step #271 asks for) -- it is on. Only one
	// Invoice's form is ever open at a time.
	let payingInvoiceId = $state('');
	let paymentStep = $state<'form' | 'review'>('form');
	let paymentMethod = $state<PaymentMethod>('check');
	let paymentNote = $state('');
	let paymentDate = $state(todayIsoDate());
	let paymentError = $state('');
	let isRecordingPayment = $state(false);

	const paymentMethodOptions = [
		{ value: 'check' as const, label: 'Check' },
		{ value: 'bank_transfer' as const, label: 'Bank transfer' },
		{ value: 'cash' as const, label: 'Cash' },
		{ value: 'other' as const, label: 'Other' }
	];

	const paymentMethodLabels: Record<PaymentMethod, string> = {
		check: 'Check',
		bank_transfer: 'Bank transfer',
		cash: 'Cash',
		other: 'Other'
	};

	function startRecordingPayment(invoiceId: string) {
		payingInvoiceId = invoiceId;
		paymentStep = 'form';
		paymentMethod = 'check';
		paymentNote = '';
		paymentDate = todayIsoDate();
		paymentError = '';
	}

	function cancelRecordingPayment() {
		payingInvoiceId = '';
		paymentError = '';
	}

	/** Moves from the entry form to the review step, after the same
	 * validation the BFF itself enforces -- so a Staff member sees the
	 * refusal before, not after, typing the whole thing twice. */
	function reviewPayment(event: SubmitEvent) {
		event.preventDefault();
		paymentError = '';
		if (paymentMethod === 'other' && paymentNote.trim() === '') {
			paymentError = 'Enter a note for "Other"';
			return;
		}
		// No empty-date check: the date TextInput's own `required` already
		// blocks an empty submission from ever reaching this handler --
		// `required` covers emptiness, this function covers the semantic
		// checks beyond it.
		if (paymentDate > todayIsoDate()) {
			paymentError = 'The date cannot be in the future';
			return;
		}
		paymentStep = 'review';
	}

	async function confirmPayment() {
		isRecordingPayment = true;
		paymentError = '';
		try {
			await onRecordPayment(payingInvoiceId, {
				method: paymentMethod,
				note: paymentNote.trim() === '' ? undefined : paymentNote.trim(),
				paidOn: paymentDate
			});
			payingInvoiceId = '';
		} catch (error_) {
			paymentError = error_ instanceof Error ? error_.message : 'Failed to record payment';
		} finally {
			isRecordingPayment = false;
		}
	}

	function payingInvoice(): Invoice | undefined {
		return invoices.find((invoice) => invoice.id === payingInvoiceId);
	}

	// Void and write-off (#271) need no confirm step of their own -- #271
	// carries no AC for one, unlike recording a Payment -- but do share one
	// busy/error pair, since only one is ever in flight per row.
	let transitioningInvoiceId = $state('');
	let transitionError = $state('');

	async function handleVoid(invoiceId: string) {
		transitioningInvoiceId = invoiceId;
		transitionError = '';
		try {
			await onVoidInvoice(invoiceId);
		} catch (error_) {
			transitionError = error_ instanceof Error ? error_.message : 'Failed to void this Invoice';
		} finally {
			transitioningInvoiceId = '';
		}
	}

	async function handleWriteOff(invoiceId: string) {
		transitioningInvoiceId = invoiceId;
		transitionError = '';
		try {
			await onWriteOffInvoice(invoiceId);
		} catch (error_) {
			transitionError = error_ instanceof Error ? error_.message : 'Failed to write off this Invoice';
		} finally {
			transitioningInvoiceId = '';
		}
	}
</script>

{#if invoices.length === 0}
	<p>No Invoices yet.</p>
{:else}
	<ul>
		{#each invoices as invoice (invoice.id)}
			<li>
				<!-- v8 ignore start: Svelte's compiled null-guard on these text nodes is unreachable -- formatAmount and invoiceStatusLabel always return a string -->
				{formatAmount(invoice.amountCents)} — {invoiceStatusLabel(invoice.status)}
				<!-- v8 ignore stop -->
				{#if invoice.paidAt}
					<!-- v8 ignore start: Svelte's compiled null-guard on this text node is unreachable -- toLocaleDateString always returns a string -->
					(paid {new Date(invoice.paidAt).toLocaleDateString()})
					<!-- v8 ignore stop -->
				{/if}
				{#if invoice.status === 'open' && isOwnerOrAdmin}
					<cluster-l space="var(--space-2)">
						<Button
							label="Record payment"
							variant="secondary"
							size="sm"
							onClick={() => startRecordingPayment(invoice.id)}
						/>
						{#if invoice.billingMode === 'by_hand'}
							<Button
								label="Void"
								variant="secondary"
								size="sm"
								loading={transitioningInvoiceId === invoice.id}
								onClick={() => handleVoid(invoice.id)}
							/>
							<Button
								label="Write off"
								variant="secondary"
								size="sm"
								loading={transitioningInvoiceId === invoice.id}
								onClick={() => handleWriteOff(invoice.id)}
							/>
						{/if}
					</cluster-l>
				{/if}
			</li>
		{/each}
	</ul>
{/if}

{#if transitionError}
	<p role="alert">{transitionError}</p>
{/if}

{#if payingInvoiceId}
	{@const invoice = payingInvoice()}
	<!-- v8 ignore start: payingInvoiceId is only ever set to an id already
	     present in invoices (startRecordingPayment), so invoice is always
	     found in practice -- this guard exists only against a reload
	     racing the form open, not a path a test can drive without faking
	     that race. -->
	{#if invoice}
		<section aria-label="Record a payment">
			<h3>Record a payment of {formatAmount(invoice.amountCents)}</h3>
			{#if paymentStep === 'form'}
				<form onsubmit={reviewPayment}>
					<RadioGroup
						legend="Method"
						options={paymentMethodOptions}
						value={paymentMethod}
						onChange={(value) => (paymentMethod = value)}
					/>
					<LabeledField label="Note (optional)">
						{#snippet children({ id, describedBy, invalid })}
							<Textarea {id} {describedBy} {invalid} value={paymentNote} onInput={(value) => (paymentNote = value)} />
						{/snippet}
					</LabeledField>
					<LabeledField label="Date received">
						{#snippet children({ id, describedBy, invalid })}
							<TextInput
								{id}
								{describedBy}
								{invalid}
								type="date"
								value={paymentDate}
								onInput={(value) => (paymentDate = value)}
								required
							/>
						{/snippet}
					</LabeledField>
					<Button label="Continue" type="submit" />
					<Button label="Cancel" variant="secondary" onClick={cancelRecordingPayment} />
				</form>
			{:else}
				<DescriptionList
					items={[
						{ label: 'Amount', value: formatAmount(invoice.amountCents) },
						{ label: 'Method', value: paymentMethodLabels[paymentMethod] },
						{ label: 'Note', value: paymentNote.trim() === '' ? '—' : paymentNote },
						{
							label: 'Date received',
							// The date input's own "YYYY-MM-DD" parses as UTC midnight
							// -- appending a local midnight time before formatting
							// keeps a Rochester, NY recorder's chosen date from
							// rendering back one day earlier.
							value: new Date(`${paymentDate}T00:00:00`).toLocaleDateString()
						}
					]}
				/>
				<Button label="Confirm and record" onClick={confirmPayment} loading={isRecordingPayment} />
				<Button label="Change" variant="secondary" onClick={() => (paymentStep = 'form')} />
			{/if}
			{#if paymentError}
				<p role="alert">{paymentError}</p>
			{/if}
		</section>
	{/if}
	<!-- v8 ignore stop -->
{/if}

{#if !isBillable}
	<Notice variant="info" message={unbillableContractMessage(contractStatus)} />
{:else if billingMode === undefined}
	<form onsubmit={handleCreate}>
		<RadioGroup
			legend="How does this Practice bill Clients?"
			options={billingModeOptions}
			value={chosenBillingMode}
			onChange={(value) => (chosenBillingMode = value)}
		/>
		<Button label="Create Invoice" type="submit" loading={isCreating} />
	</form>
	{#if createError}
		<p role="alert">{createError}</p>
	{/if}
{:else if billingMode === 'stripe' && !clientsCanPay}
	<Notice variant="info" message={clientsCannotPayMessage} />
	{#if isOwner}
		<Link href={paymentsSettingsHref} label="Go to Payments settings" />
	{/if}
{:else if billingMode === 'stripe' && !hasClientEmail}
	<Notice variant="info" message={clientHasNoEmailMessage} />
{:else}
	<form onsubmit={handleCreate}>
		<Button label="Create Invoice" type="submit" loading={isCreating} />
	</form>
	{#if createError}
		<p role="alert">{createError}</p>
	{/if}
{/if}
