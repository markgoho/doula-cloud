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
		contractorInvoicesMessage,
		formatAmount,
		invoiceCreatedMessage,
		invoiceStatusLabel,
		invoiceStatusVariant,
		paymentMethodLabels,
		paymentMethodOptions,
		unbillableContractMessage,
		type BillingMode,
		type Invoice,
		type PaymentMethod,
		type RefundPaymentInput
	} from '#lib/invoice.js';
	import { formatCalendarDay } from '#lib/dates.js';
	import RefundPaymentForm from '#lib/components/organisms/RefundPaymentForm.svelte';
	import { RefusalError, type FormError } from '#lib/formErrors.js';
	import Badge from '#lib/components/atoms/Badge.svelte';
	import Button from '#lib/components/atoms/Button.svelte';
	import Heading from '#lib/components/atoms/Heading.svelte';
	import Link from '#lib/components/atoms/Link.svelte';
	import Notice from '#lib/components/atoms/Notice.svelte';
	import TextInput from '#lib/components/atoms/TextInput.svelte';
	import Textarea from '#lib/components/atoms/Textarea.svelte';
	import ErrorSummary from '#lib/components/molecules/ErrorSummary.svelte';
	import LabeledField from '#lib/components/molecules/LabeledField.svelte';
	import RadioGroup, { radioFieldId } from '#lib/components/molecules/RadioGroup.svelte';
	import StackedForm from '#lib/components/molecules/StackedForm.svelte';
	import SummaryList from '#lib/components/molecules/SummaryList.svelte';
	import DataTable, { type DataTableView } from '#lib/components/organisms/DataTable.svelte';

	let {
		invoices,
		contractStatus,
		billingMode,
		clientsCanPay,
		hasClientEmail,
		isOwner,
		isOwnerOrAdmin,
		canReadInvoices = true,
		practiceTimezone,
		paymentsSettingsHref,
		onCreate,
		onRecordPayment,
		onVoidInvoice,
		onWriteOffInvoice,
		onReversePayment,
		onRefundPayment
	}: {
		invoices: Invoice[];
		contractStatus: string;
		/** Undefined until the Practice has chosen a rail (#271) -- the
		 * component itself asks then, rather than being told what to ask. */
		billingMode: BillingMode | undefined;
		clientsCanPay: boolean;
		hasClientEmail: boolean;
		isOwner: boolean;
		/** The Practice's own IANA zone (`practices.timezone`, ADR-0036) --
		 * what `todayIsoDate` reads "today" in, so the payment-date field's
		 * default and its future-date ceiling agree with
		 * `PostManualPaymentHandler`'s own guard (#1167, #1280) instead of
		 * UTC's calendar day. The caller fetches it the same way the
		 * timezone settings screen does (`loadPracticeTimezone`). */
		practiceTimezone: string;
		/** Gates Record payment/Void/Write off/Reverse payment/Return money -- ADR-0008's
		 * Contract-money write row, Owner and Admin only. */
		isOwnerOrAdmin: boolean;
		/** False for a contractor Doula (#2002): ADR-0008 gives her no
		 * Invoice history and no Practice price, so the list is replaced by
		 * one information line. Raising an Invoice stays hers (ADR-0008's
		 * write table), so the Create Invoice branches below are unchanged. */
		canReadInvoices?: boolean;
		paymentsSettingsHref: string;
		onCreate: (billingMode?: BillingMode) => Promise<void>;
		onRecordPayment: (
			invoiceId: string,
			input: { method: PaymentMethod; note?: string; paidOn: string }
		) => Promise<void>;
		onVoidInvoice: (invoiceId: string) => Promise<void>;
		onWriteOffInvoice: (invoiceId: string) => Promise<void>;
		onReversePayment: (invoiceId: string, paymentId: string, reason: string) => Promise<void>;
		/**
		Returns money against the Invoice's covering Payment (#1009).
		*/
		onRefundPayment: (invoiceId: string, paymentId: string, input: RefundPaymentInput) => Promise<void>;
	} = $props();

	const isBillable = $derived(contractStatus === billableContractStatus);

	// No option is selected when the question opens (#1590): a default
	// carries authority, and a Staff member who presses the button without
	// reading would otherwise set the Practice's billing mode unchosen.
	let chosenBillingMode = $state<BillingMode | undefined>();
	// billingModeError is the billing-mode question's own error, from the
	// empty-submit check or a BFF refusal naming `billingMode` (#1775).
	// createError is the untargeted summary fallback: a refusal that names
	// no field on screen, such as Stripe not connected.
	let billingModeError = $state('');
	let isCreating = $state(false);
	let createError = $state('');
	// #2002: set once a create works for a reader with no list to show it in.
	let hasCreatedUnseen = $state(false);

	const billingModeName = 'invoice-billing-mode';
	// The first option's id, as the payment form's Method does: the summary
	// link lands on the first option, whichever one is checked.
	const billingModeFieldId = radioFieldId(billingModeName, 'stripe');
	// One list for both Create Invoice forms (#1775), the same shape as
	// paymentFormErrors below, so a refusal is one ErrorSummary and never
	// a second, bare alert line beside it.
	const createFormErrors = $derived<FormError[]>(
		[
			billingModeError ? { message: billingModeError, targetId: billingModeFieldId } : undefined,
			createError ? { message: createError } : undefined
		].filter((entry) => entry !== undefined)
	);

	/** Reads a caught Create Invoice refusal into the form's error state,
	 * as applyPaymentRefusal does below. A `details.billingMode` is read
	 * onto the question only while the question is on screen (a
	 * field-level error is only wired into a field that is there); any
	 * other refusal is the untargeted summary entry. */
	function applyCreateRefusal(error_: unknown) {
		if (billingMode === undefined && error_ instanceof RefusalError && error_.details?.billingMode) {
			billingModeError = error_.details.billingMode;
			return;
		}
		createError = error_ instanceof Error ? error_.message : 'Failed to create invoice';
	}

	const billingModeOptions = $derived([
		{
			value: 'stripe' as const,
			label: 'By card, through Stripe.',
			description: clientsCanPay
				? 'DoulaCloud sends the bill and the Client pays by card.'
				: 'DoulaCloud sends the bill and the Client pays by card. A Practice Owner must connect Stripe first.'
		},
		{
			value: 'by_hand' as const,
			label: 'By hand.',
			description: 'This Practice sends the bill and collects the payment itself. You record each payment here.'
		}
	]);

	// #947: the amount an Invoice is raised for is the Contract's own
	// price, never a figure typed here -- there is nothing left to
	// validate before calling onCreate.
	async function handleCreate(event: SubmitEvent) {
		event.preventDefault();
		createError = '';
		billingModeError = '';

		if (billingMode === undefined && chosenBillingMode === undefined) {
			billingModeError = 'Select how this Practice bills Clients';
			return;
		}

		isCreating = true;
		try {
			if (billingMode === undefined) {
				await onCreate(chosenBillingMode);
			} else {
				await onCreate();
			}
			hasCreatedUnseen = !canReadInvoices;
		} catch (error_) {
			applyCreateRefusal(error_);
		} finally {
			isCreating = false;
		}
	}

	/** Today's date in the form TextInput type="date" reads and writes --
	 * the default paymentDate, and the future-date guard's own ceiling.
	 *
	 * Read in the Practice's own zone (#1280), not UTC's: `en-CA` formats
	 * `Intl.DateTimeFormat` as `YYYY-MM-DD` directly, so the offset zone
	 * gives back exactly the calendar day this field wants, with no
	 * further reassembly. Before this, `toISOString().slice(0, 10)` read
	 * UTC's day, which for several evening hours in a zone behind UTC
	 * (including America/New_York) disagreed with
	 * `PostManualPaymentHandler`'s own guard (#1167) and refused a Staff
	 * member's genuinely-today date. */
	function todayIsoDate(): string {
		return new Intl.DateTimeFormat('en-CA', { timeZone: practiceTimezone }).format(new Date());
	}

	// #271's recording flow: which Invoice (if any) is being settled, and
	// which of its two steps -- fill in the details, then confirm them
	// (the check-your-answers step #271 asks for) -- it is on. Only one
	// Invoice's form is ever open at a time.
	let payingInvoiceId = $state('');
	let paymentStep = $state<'form' | 'review'>('form');
	// No method is chosen when the form opens (#1913, ADR-0055): how a
	// Client paid is a fact the service does not know, and a Staff member
	// who went straight to Continue used to record a check unasked.
	let paymentMethod = $state<PaymentMethod | undefined>();
	let paymentNote = $state('');
	// Pre-filled with today on purpose: a recorded departure from GOV.UK's
	// "nothing pre-filled" rule, decided 2026-10-24 on #1923 -- see "A
	// pre-filled answer: Date received" under Where we depart on purpose in
	// docs/design/govuk-alignment.md.
	let paymentDate = $state(todayIsoDate());
	// paymentError is the untargeted GOV.UK summary fallback (#1038): a
	// refusal naming no field of this form -- a Stripe-side failure, an
	// Invoice no longer open. The three below carry a refusal that does
	// name a field, read into the LabeledField/RadioGroup it concerns and
	// into the summary (#1228) alike -- one string, both places.
	let paymentError = $state('');
	let paymentMethodError = $state('');
	let paymentNoteError = $state('');
	let paymentDateError = $state('');
	let isRecordingPayment = $state(false);

	const paymentMethodName = 'record-payment-method';
	// The first option's own id, the same trick `endingReasonFieldId`
	// (above `completeSubmission`, on the Engagement detail page) uses: a
	// radio group's refusal belongs to the question, not to whichever
	// option happens to be checked, so the summary link always lands on
	// the first one. Built through `radioFieldId` rather than a second
	// hand-rolled copy of RadioGroup's own `${name}-${value}` template.
	const paymentMethodFieldId = radioFieldId(paymentMethodName, paymentMethodOptions[0]!.value);
	const paymentNoteFieldId = 'record-payment-note';
	const paymentDateFieldId = 'record-payment-date';

	// #1228: assembled from the same four strings the fields already show,
	// so the summary and the field-level message cannot drift. Built only
	// while the form step is on screen -- the review step shows a
	// SummaryList, not these controls, so nothing here could be linked
	// to during it.
	const paymentFormErrors = $derived<FormError[]>(
		[
			paymentDateError ? { message: paymentDateError, targetId: paymentDateFieldId } : undefined,
			paymentMethodError ? { message: paymentMethodError, targetId: paymentMethodFieldId } : undefined,
			paymentNoteError ? { message: paymentNoteError, targetId: paymentNoteFieldId } : undefined,
			paymentError ? { message: paymentError } : undefined
		].filter((entry) => entry !== undefined)
	);

	function resetPaymentErrors() {
		paymentError = '';
		paymentMethodError = '';
		paymentNoteError = '';
		paymentDateError = '';
	}

	/** Reads a caught refusal into this form's field-level state, per
	 * `formErrors.ts`'s GOV.UK rules: a `RefusalError` naming one of this
	 * form's own fields (`method`/`note`/`paidOn`, PostManualPaymentHandler's
	 * own `details` keys, #1037) is read onto that field; anything else --
	 * no `details` at all, or a key none of these fields own -- stays the
	 * single untargeted summary paragraph this form already showed. */
	function applyPaymentRefusal(error_: unknown) {
		resetPaymentErrors();
		if (error_ instanceof RefusalError && error_.details) {
			paymentMethodError = error_.details.method ?? '';
			paymentNoteError = error_.details.note ?? '';
			paymentDateError = error_.details.paidOn ?? '';
			if (paymentMethodError || paymentNoteError || paymentDateError) {
				// Back to the entry step: a field-level error is only
				// wired into the field it concerns while that field is on
				// screen, and the review step shows a SummaryList, not
				// the form.
				paymentStep = 'form';
				return;
			}
		}
		paymentError = error_ instanceof Error ? error_.message : 'Failed to record payment';
	}

	function startRecordingPayment(invoiceId: string) {
		payingInvoiceId = invoiceId;
		paymentStep = 'form';
		paymentMethod = undefined;
		paymentNote = '';
		paymentDate = todayIsoDate();
		resetPaymentErrors();
		cancelReversingPayment();
		cancelRefundingPayment();
	}

	function cancelRecordingPayment() {
		payingInvoiceId = '';
		resetPaymentErrors();
	}

	/** Moves from the entry form to the review step, after the same
	 * validation the BFF itself enforces -- so a Staff member sees the
	 * refusal before, not after, typing the whole thing twice. Each check
	 * targets the field it is about (#1038), the same as a refusal read
	 * back off the BFF.
	 *
	 * The empty-date check used to be `required`'s job -- #1228 adopted
	 * `StackedForm`, whose `novalidate` (ADR-0021) took that browser
	 * refusal away, so this function covers emptiness now as well as the
	 * semantic checks it already made. */
	function reviewPayment(event: SubmitEvent) {
		event.preventDefault();
		resetPaymentErrors();
		// Method first: it is the form's first question, and GOV.UK lists
		// refusals in the order the fields are on the page.
		if (paymentMethod === undefined) {
			paymentMethodError = 'Select how the payment was made';
			return;
		}
		if (paymentDate.trim() === '') {
			paymentDateError = 'Enter the date the payment was received';
			return;
		}
		if (paymentMethod === 'other' && paymentNote.trim() === '') {
			paymentNoteError = 'Enter a note for "Other"';
			return;
		}
		if (paymentDate > todayIsoDate()) {
			paymentDateError = 'The date cannot be in the future';
			return;
		}
		paymentStep = 'review';
	}

	async function confirmPayment() {
		isRecordingPayment = true;
		resetPaymentErrors();
		try {
			await onRecordPayment(payingInvoiceId, {
				// Set: reviewPayment refuses an empty Method before the
				// review step, the only place this button is drawn.
				method: paymentMethod!,
				note: paymentNote.trim() === '' ? undefined : paymentNote.trim(),
				paidOn: paymentDate
			});
			payingInvoiceId = '';
		} catch (error_) {
			applyPaymentRefusal(error_);
		} finally {
			isRecordingPayment = false;
		}
	}

	function payingInvoice(): Invoice | undefined {
		return invoices.find((invoice) => invoice.id === payingInvoiceId);
	}

	// #945's reversal flow: the same shape as payment recording above --
	// which Invoice/Payment (if any) is being reversed, and which of its
	// two steps -- fill in a reason, then confirm it -- it is on. Only one
	// Invoice's form (recording or reversal) is ever open at a time.
	let reversingInvoiceId = $state('');
	let reversingPaymentId = $state('');
	let reversalStep = $state<'form' | 'review'>('form');
	let reversalReason = $state('');
	// reversalError is the untargeted summary fallback (#1038) -- a
	// Stripe-backed refusal, or a Payment already reversed; neither names
	// the Reason field. reversalReasonError is that field's own error.
	let reversalError = $state('');
	let reversalReasonError = $state('');
	let isReversingPayment = $state(false);

	function resetReversalErrors() {
		reversalError = '';
		reversalReasonError = '';
	}

	/** As applyPaymentRefusal above: PostReversePaymentHandler's own
	 * `reason` details key (#945) is read onto the Reason field; anything
	 * else stays the untargeted summary paragraph. */
	function applyReversalRefusal(error_: unknown) {
		resetReversalErrors();
		if (error_ instanceof RefusalError && error_.details?.reason) {
			reversalReasonError = error_.details.reason;
			// Back to the entry step, matching applyPaymentRefusal above.
			reversalStep = 'form';
			return;
		}
		reversalError = error_ instanceof Error ? error_.message : 'Failed to reverse this payment';
	}

	function startReversingPayment(invoiceId: string, paymentId: string) {
		reversingInvoiceId = invoiceId;
		reversingPaymentId = paymentId;
		reversalStep = 'form';
		reversalReason = '';
		resetReversalErrors();
		cancelRecordingPayment();
		cancelRefundingPayment();
	}

	function cancelReversingPayment() {
		reversingInvoiceId = '';
		reversingPaymentId = '';
		resetReversalErrors();
	}

	/** Moves from the entry form to the review step, after the same
	 * non-blank check the BFF itself enforces on the reason -- targeted at
	 * the Reason field (#1038), the same as a refusal read back off the
	 * BFF. */
	function reviewReversal(event: SubmitEvent) {
		event.preventDefault();
		resetReversalErrors();
		if (reversalReason.trim() === '') {
			reversalReasonError = 'Enter a reason for reversing this payment';
			return;
		}
		reversalStep = 'review';
	}

	async function confirmReversal() {
		isReversingPayment = true;
		resetReversalErrors();
		try {
			await onReversePayment(reversingInvoiceId, reversingPaymentId, reversalReason.trim());
			reversingInvoiceId = '';
			reversingPaymentId = '';
		} catch (error_) {
			applyReversalRefusal(error_);
		} finally {
			isReversingPayment = false;
		}
	}

	function reversingInvoice(): Invoice | undefined {
		return invoices.find((invoice) => invoice.id === reversingInvoiceId);
	}

	// #1009's Refund flow: which Invoice/Payment (if any) money is being
	// returned against. The form itself is RefundPaymentForm, which owns
	// its own steps and errors; only one Invoice's form of any kind is ever
	// open at a time, so opening this one closes the other two.
	let refundingInvoiceId = $state('');
	let refundingPaymentId = $state('');

	function startRefundingPayment(invoiceId: string, paymentId: string) {
		refundingInvoiceId = invoiceId;
		refundingPaymentId = paymentId;
		cancelRecordingPayment();
		cancelReversingPayment();
	}

	function cancelRefundingPayment() {
		refundingInvoiceId = '';
		refundingPaymentId = '';
	}

	async function confirmRefund(input: RefundPaymentInput) {
		await onRefundPayment(refundingInvoiceId, refundingPaymentId, input);
		cancelRefundingPayment();
	}

	function refundingInvoice(): Invoice | undefined {
		return invoices.find((invoice) => invoice.id === refundingInvoiceId);
	}

	/**
	Whether any money is still left to return on a paid Invoice.
	*/
	/*
	 * The Invoices as a table (#2059): the amount first, since it titles
	 * each Invoice's card at a narrow width, then the status as a Badge, the
	 * day it was paid, and the money returned on it. A Paid Invoice stays
	 * Paid after a Refund, as Stripe's own does (#1009), so Returned is
	 * where that money is said at all. A blank cell is left out of a card.
	 */
	const invoiceColumns = [
		{ label: 'Amount', accessor: (invoice: Invoice) => formatAmount(invoice.amountCents), numeric: true },
		{ label: 'Status', accessor: (invoice: Invoice) => invoiceStatusLabel(invoice.status), content: invoiceStatus },
		{ label: 'Paid', accessor: (invoice: Invoice) => (invoice.paidOn ? formatCalendarDay(invoice.paidOn) : '') },
		{
			label: 'Returned',
			accessor: (invoice: Invoice) => (invoice.refundedCents > 0 ? formatAmount(invoice.refundedCents) : ''),
			numeric: true
		}
	];

	function hasMoneyLeftToReturn(invoice: Invoice): boolean {
		return invoice.amountCents > invoice.refundedCents;
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

{#snippet invoiceStatus(invoice: Invoice)}
	<Badge label={invoiceStatusLabel(invoice.status)} variant={invoiceStatusVariant(invoice.status)} />
{/snippet}

<!--
	An Invoice's actions (#2059): the row's own, never a field inside a list
	item. DataTable draws this into both of its trees, so the name span's id
	carries the view (#666). The buttons read the same words on every row,
	so each is described by the Invoice it acts on.
-->
{#snippet invoiceActionButtons(invoice: Invoice, describedBy: string)}
	{#if invoice.status === 'open' && isOwnerOrAdmin}
		<Button
			label="Record payment"
			variant="secondary"
			size="sm"
			{describedBy}
			onClick={() => startRecordingPayment(invoice.id)}
		/>
		{#if invoice.billingMode === 'by_hand'}
			<Button
				label="Void"
				variant="secondary"
				size="sm"
				{describedBy}
				loading={transitioningInvoiceId === invoice.id}
				onClick={() => handleVoid(invoice.id)}
			/>
			<Button
				label="Write off"
				variant="secondary"
				size="sm"
				{describedBy}
				loading={transitioningInvoiceId === invoice.id}
				onClick={() => handleWriteOff(invoice.id)}
			/>
		{/if}
	{/if}
	{#if invoice.status === 'paid' && isOwnerOrAdmin && invoice.activePaymentId}
		{@const activePaymentId = invoice.activePaymentId}
		{#if hasMoneyLeftToReturn(invoice)}
			<Button
				label="Return money"
				variant="secondary"
				size="sm"
				{describedBy}
				onClick={() => startRefundingPayment(invoice.id, activePaymentId)}
			/>
		{/if}
		<!-- Reverse is by-hand only (#945), and never once money has
		     gone back: the BFF refuses a refunded Payment
		     (MsgRefundedPaymentCannotBeReversed), so the button that
		     would only earn that refusal is not offered. -->
		{#if invoice.billingMode === 'by_hand' && invoice.refundedCents === 0}
			<Button
				label="Reverse payment"
				variant="secondary"
				size="sm"
				{describedBy}
				onClick={() => startReversingPayment(invoice.id, activePaymentId)}
			/>
		{/if}
	{/if}
{/snippet}

{#snippet invoiceActions(invoice: Invoice, view: DataTableView)}
	{@const nameId = `invoice-${view}-${invoice.id}`}
	<!-- v8 ignore start: Svelte's compiled null-guard on this text node is unreachable -- reference is required and formatAmount always returns a string -->
	<span class="visually-hidden" id={nameId}>Invoice {invoice.reference}, {formatAmount(invoice.amountCents)}</span>
	<!-- v8 ignore stop -->
	<!-- The record view already sets a card's actions in a cluster; the
	     table's cell is the one that needs its own. -->
	{#if view === 'table'}
		<cluster-l space="var(--space-related)">
			{@render invoiceActionButtons(invoice, nameId)}
		</cluster-l>
	{:else}
		{@render invoiceActionButtons(invoice, nameId)}
	{/if}
{/snippet}

{#if !canReadInvoices}
	<Notice variant="info" message={contractorInvoicesMessage} />
	{#if hasCreatedUnseen}
		<Notice variant="status" message={invoiceCreatedMessage} isFocusedOnAppear />
	{/if}
{:else}
	<!--
		#2059: one row per Invoice, its status a Badge and its actions the
		row's own, in place of a list item holding a sentence and buttons
		(#2017: no list item holds a control). At a narrow width DataTable
		draws each Invoice as a card with its actions at the foot.
	-->
	<DataTable
		columns={invoiceColumns}
		rows={invoices}
		rowActions={isOwnerOrAdmin ? { label: 'Actions', content: invoiceActions } : undefined}
		emptyMessage="No Invoices yet."
	/>
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
			<Heading level={3} text="Record a payment of {formatAmount(invoice.amountCents)}" />
			<!--
				Above the step switch, not inside the form branch: a refusal
				that names no field (applyPaymentRefusal's untargeted
				paymentError) leaves paymentStep at 'review', where the form
				below is not even rendered, and this is still where it has to
				show (#1228). One region for the whole section, matching
				ErrorSummary's own "never two alert regions on one form" rule.
			-->
			{#if paymentFormErrors.length > 0}
				<ErrorSummary errors={paymentFormErrors} />
			{/if}
			{#if paymentStep === 'form'}
				<StackedForm onSubmit={reviewPayment}>
					<RadioGroup
						legend="Method"
						name={paymentMethodName}
						options={paymentMethodOptions}
						value={paymentMethod}
						onChange={(value) => (paymentMethod = value)}
						error={paymentMethodError || undefined}
					/>
					<LabeledField id={paymentNoteFieldId} label="Note (optional)" error={paymentNoteError || undefined}>
						{#snippet children({ id, describedBy, invalid })}
							<Textarea {id} {describedBy} {invalid} value={paymentNote} onInput={(value) => (paymentNote = value)} />
						{/snippet}
					</LabeledField>
					<LabeledField id={paymentDateFieldId} label="Date received" error={paymentDateError || undefined}>
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
				</StackedForm>
			{:else}
				<SummaryList
					rows={[
						{ label: 'Amount', value: formatAmount(invoice.amountCents) },
						{ label: 'Method', value: paymentMethodLabels[paymentMethod!] },
						{ label: 'Note', value: paymentNote.trim() === '' ? '—' : paymentNote },
						{
							label: 'Date received',
							// The date input's own "YYYY-MM-DD" is a calendar day, so
							// it is built from its parts rather than parsed as UTC
							// midnight, which a Rochester, NY recorder would read
							// back one day earlier.
							value: formatCalendarDay(paymentDate)
						}
					]}
				/>
				<Button label="Confirm and record" onClick={confirmPayment} loading={isRecordingPayment} />
				<Button label="Change" variant="secondary" onClick={() => (paymentStep = 'form')} />
			{/if}
		</section>
	{/if}
	<!-- v8 ignore stop -->
{/if}

{#if reversingInvoiceId}
	{@const invoice = reversingInvoice()}
	<!-- v8 ignore start: reversingInvoiceId is only ever set to an id already
	     present in invoices (startReversingPayment), so invoice is always
	     found in practice -- this guard exists only against a reload
	     racing the form open, not a path a test can drive without faking
	     that race. -->
	{#if invoice}
		<section aria-label="Reverse this payment">
			<Heading level={3} text="Reverse a payment of {formatAmount(invoice.amountCents)}" />
			{#if reversalStep === 'form'}
				<StackedForm onSubmit={reviewReversal}>
					<LabeledField label="Reason" error={reversalReasonError || undefined}>
						{#snippet children({ id, describedBy, invalid })}
							<Textarea {id} {describedBy} {invalid} value={reversalReason} onInput={(value) => (reversalReason = value)} />
						{/snippet}
					</LabeledField>
					<Button label="Continue" type="submit" />
					<Button label="Cancel" variant="secondary" onClick={cancelReversingPayment} />
				</StackedForm>
			{:else}
				<SummaryList
					rows={[
						{ label: 'Amount', value: formatAmount(invoice.amountCents) },
						{ label: 'Reason', value: reversalReason }
					]}
				/>
				<Button label="Confirm and reverse" onClick={confirmReversal} loading={isReversingPayment} />
				<Button label="Change" variant="secondary" onClick={() => (reversalStep = 'form')} />
			{/if}
			{#if reversalError}
				<p role="alert">{reversalError}</p>
			{/if}
		</section>
	{/if}
	<!-- v8 ignore stop -->
{/if}

{#if refundingInvoiceId}
	{@const invoice = refundingInvoice()}
	<!-- v8 ignore start: refundingInvoiceId is only ever set to an id already
	     present in invoices (startRefundingPayment) -- the same reload-race
	     guard the two forms above carry. -->
	{#if invoice}
		{#key refundingInvoiceId}
			<RefundPaymentForm {invoice} onConfirm={confirmRefund} onCancel={cancelRefundingPayment} />
		{/key}
	{/if}
	<!-- v8 ignore stop -->
{/if}

{#if !isBillable}
	<Notice variant="info" message={unbillableContractMessage(contractStatus)} />
{:else if billingMode === undefined}
	<StackedForm onSubmit={handleCreate}>
		{#if createFormErrors.length > 0}
			<ErrorSummary errors={createFormErrors} />
		{/if}
		<RadioGroup
			legend="How does this Practice bill Clients?"
			hint="You answer this one time. A Practice Owner can change it later in Settings."
			name={billingModeName}
			options={billingModeOptions}
			value={chosenBillingMode}
			onChange={(value) => (chosenBillingMode = value)}
			error={billingModeError || undefined}
		/>
		<Button label="Create Invoice" type="submit" loading={isCreating} />
	</StackedForm>
{:else if billingMode === 'stripe' && !clientsCanPay}
	<Notice variant="info" message={clientsCannotPayMessage} />
	{#if isOwner}
		<Link href={paymentsSettingsHref} label="Go to Payments settings" />
	{/if}
{:else if billingMode === 'stripe' && !hasClientEmail}
	<Notice variant="info" message={clientHasNoEmailMessage} />
{:else}
	<!-- A StackedForm since #1775: a refusal puts an ErrorSummary above the button, so the form has a run to space, which #1108's bare `<form>` did not. -->
	<StackedForm onSubmit={handleCreate}>
		{#if createFormErrors.length > 0}
			<ErrorSummary errors={createFormErrors} />
		{/if}
		<Button label="Create Invoice" type="submit" loading={isCreating} />
	</StackedForm>
{/if}
