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
	 *   itself carries. Since #2106 the question is asked only when the
	 *   first raise is: the section shows its facts and one Create Invoice
	 *   that opens the question, never the question unasked (the record-page
	 *   pattern, govuk-alignment.md, "Sections of a record page").
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
		cardPaymentsRefusal,
		cardPaymentsWait,
		contractorInvoicesMessage,
		formatAmount,
		invoiceCreatedMessage,
		paymentRecordedMessage,
		invoiceRowStatusLabel,
		invoiceRowStatusVariant,
		paymentMethodLabels,
		unbillableContractMessage,
		type BillingMode,
		type ContractorInvoice,
		type Invoice,
		type InvoiceStanding,
		type PaymentMethod,
		type RecordPaymentInput,
		type RefundPaymentInput
	} from '#lib/invoice.js';
	import { formatCalendarDay } from '#lib/dates.js';
	import RecordPaymentForm from '#lib/components/organisms/RecordPaymentForm.svelte';
	import RefundPaymentForm from '#lib/components/organisms/RefundPaymentForm.svelte';
	import ReversePaymentForm from '#lib/components/organisms/ReversePaymentForm.svelte';
	import { RefusalError, type FormError } from '#lib/formErrors.js';
	import Badge from '#lib/components/atoms/Badge.svelte';
	import Button from '#lib/components/atoms/Button.svelte';
	import Heading from '#lib/components/atoms/Heading.svelte';
	import Link from '#lib/components/atoms/Link.svelte';
	import Notice from '#lib/components/atoms/Notice.svelte';
	import ErrorSummary from '#lib/components/molecules/ErrorSummary.svelte';
	import RadioGroup, { radioFieldId } from '#lib/components/molecules/RadioGroup.svelte';
	import StackedForm from '#lib/components/molecules/StackedForm.svelte';
	import DataTable, { type DataTableView } from '#lib/components/organisms/DataTable.svelte';

	let {
		invoices,
		contractStatus,
		billingMode,
		clientsCanPay,
		hasClientEmail,
		isOwner,
		isOwnerOrAdmin,
		canReadInvoiceMoney = true,
		contractorInvoices = [],
		isAskingBillingMode = $bindable(false),
		practiceTimezone,
		paymentsSettingsHref,
		landOnInvoiceId,
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
		 * what RecordPaymentForm reads "today" in, so the payment-date field's
		 * default and its future-date ceiling agree with
		 * `PostManualPaymentHandler`'s own guard (#1167, #1280) instead of
		 * UTC's calendar day. The caller fetches it the same way the
		 * timezone settings screen does (`loadPracticeTimezone`). */
		practiceTimezone: string;
		/** Gates Record payment/Void/Write off/Reverse payment/Return money -- ADR-0008's
		 * Contract-money write row, Owner and Admin only. Also who is linked
		 * to Getting paid while Stripe cannot take a card (#2200): its
		 * Connect status read is the same Owner-and-Admin pair. */
		isOwnerOrAdmin: boolean;
		/** False for a contractor Doula (#2002, #2094): ADR-0008 gives her
		 * each Invoice's date and status and never its money, so the
		 * money-bearing `invoices` list is replaced by `contractorInvoices`
		 * and one information line. Raising an Invoice stays hers (ADR-0008's
		 * write table), so the Create Invoice branches below are unchanged. */
		canReadInvoiceMoney?: boolean;
		/** A contractor's rows (#2094), as the BFF sends her them: days and
		 * status, no money. Read only when canReadInvoiceMoney is false. */
		contractorInvoices?: ContractorInvoice[];
		/** Whether the billing-mode question is open (#2106): it is on the
		 * page only once Create Invoice has asked for it. Bindable, as
		 * OfferSection's isFormOpen, so a demo can draw it open. */
		isAskingBillingMode?: boolean;
		paymentsSettingsHref: string;
		/** The Invoice a link named (#2240): its row takes focus, once,
		 * when it arrives, so the Invoices list lands on it. */
		landOnInvoiceId?: string;
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

	function invoiceRowId(invoice: { id: string }, view: DataTableView): string {
		return `invoice-row-${view}-${invoice.id}`;
	}

	// Once only: invoices is read again after every record, void and
	// reverse, and a later read must not pull focus back to the row.
	let hasLanded = false;
	$effect(() => {
		if (hasLanded || !landOnInvoiceId) {
			return;
		}
		const rows: { id: string }[] = canReadInvoiceMoney ? invoices : contractorInvoices;
		const invoice = rows.find((row) => row.id === landOnInvoiceId);
		if (!invoice) {
			return;
		}
		hasLanded = true;
		const views: DataTableView[] = ['table', 'record'];
		const shownRow = views
			.map((view) => document.querySelector<HTMLElement>(`#${CSS.escape(invoiceRowId(invoice, view))}`))
			.find((row) => row !== null && row.getClientRects().length > 0);
		shownRow?.scrollIntoView({ block: 'start' });
		shownRow?.focus({ preventScroll: true });
	});

	// No option is selected when the question opens (#1590): a default
	// carries authority, and a Staff member who presses the button without
	// reading would otherwise set the Practice's billing mode unchosen.
	let chosenBillingMode = $state<BillingMode | undefined>();
	// billingModeError is the billing-mode question's own error, from the
	// empty-submit check or a BFF refusal naming `billingMode` (#1775).
	// createError is the untargeted summary fallback: a refusal that names
	// no field on screen, such as Stripe not connected. It is a whole
	// FormError so that the one this form composes itself can carry the
	// link to its fix (#2200).
	let billingModeError = $state('');
	let isCreating = $state(false);
	let createError = $state<FormError | undefined>();
	// Set once a create works (#2002); cleared when the next one starts.
	let hasCreated = $state(false);
	// What the last Confirm and record wrote (#2241); cleared when another
	// act on the section starts, so one status line stands at a time.
	let paymentRecorded = $state('');
	function openBillingModeQuestion() {
		hasCreated = false;
		paymentRecorded = '';
		isAskingBillingMode = true;
	}

	function closeBillingModeQuestion() {
		isAskingBillingMode = false;
		chosenBillingMode = undefined;
		billingModeError = '';
		createError = undefined;
	}

	const billingModeName = 'invoice-billing-mode';
	// The first option's id, as the payment form's Method does: the summary
	// link lands on the first option, whichever one is checked.
	const billingModeFieldId = radioFieldId(billingModeName, 'stripe');
	// One list for both Create Invoice forms (#1775), the same shape as
	// RecordPaymentForm's own list, so a refusal is one ErrorSummary and never
	// a second, bare alert line beside it.
	const createFormErrors = $derived<FormError[]>(
		[
			billingModeError ? { message: billingModeError, targetId: billingModeFieldId } : undefined,
			createError
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
		createError = { message: error_ instanceof Error ? error_.message : 'Failed to create invoice' };
	}

	// #2200: what an Owner, an Admin and anyone else can do about it.
	const cardPaymentsWaitWords = $derived(cardPaymentsWait({ isOwner, isOwnerOrAdmin }));

	const billingModeOptions = $derived([
		{
			value: 'stripe' as const,
			label: 'By card, through Stripe.',
			description: clientsCanPay
				? 'DoulaCloud sends the bill and the Client pays by card.'
				: `DoulaCloud sends the bill and the Client pays by card. ${isOwner ? 'Connect Stripe first, on Getting paid.' : 'A Practice Owner must connect Stripe first.'}`
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
		createError = undefined;
		billingModeError = '';
		hasCreated = false;
		paymentRecorded = '';

		if (billingMode === undefined && chosenBillingMode === undefined) {
			billingModeError = 'Select how this Practice bills Clients';
			return;
		}

		// #2200: the BFF refuses a card Invoice until Stripe can take one,
		// and this form already knows that from the standing fact, so it
		// says so itself, links to the fix where the reader can open it,
		// and sends nothing.
		if (billingMode === undefined && chosenBillingMode === 'stripe' && !clientsCanPay) {
			createError = {
				message: cardPaymentsRefusal({ isOwner, isOwnerOrAdmin }),
				...(isOwnerOrAdmin && { href: paymentsSettingsHref })
			};
			return;
		}

		isCreating = true;
		try {
			if (billingMode === undefined) {
				await onCreate(chosenBillingMode);
			} else {
				await onCreate();
			}
			isAskingBillingMode = false;
			hasCreated = true;
		} catch (error_) {
			applyCreateRefusal(error_);
		} finally {
			isCreating = false;
		}
	}

	// #271's recording flow: which Invoice (if any) is being settled. Only
	// one Invoice's form is ever open at a time; the form itself is
	// RecordPaymentForm, shared with a Doula's fee (#2271).
	let payingInvoiceId = $state('');

	function startRecordingPayment(invoiceId: string) {
		paymentRecorded = '';
		payingInvoiceId = invoiceId;
		cancelReversingPayment();
		cancelRefundingPayment();
	}

	function cancelRecordingPayment() {
		payingInvoiceId = '';
	}

	async function confirmPayment(input: RecordPaymentInput) {
		// Set: the form is drawn only for an Invoice in the list.
		const { amountCents } = payingInvoice()!;
		await onRecordPayment(payingInvoiceId, input);
		payingInvoiceId = '';
		hasCreated = false;
		paymentRecorded = paymentRecordedMessage(amountCents, input.method, input.paidOn);
	}

	function payingInvoice(): Invoice | undefined {
		return invoices.find((invoice) => invoice.id === payingInvoiceId);
	}

	// #945's reversal flow: which Invoice/Payment (if any) is being
	// reversed. The form itself is ReversePaymentForm, shared with a
	// Doula's fee (#2271). Only one Invoice's form of any kind is open.
	let reversingInvoiceId = $state('');
	let reversingPaymentId = $state('');

	function startReversingPayment(invoiceId: string, paymentId: string) {
		paymentRecorded = '';
		reversingInvoiceId = invoiceId;
		reversingPaymentId = paymentId;
		cancelRecordingPayment();
		cancelRefundingPayment();
	}

	function cancelReversingPayment() {
		reversingInvoiceId = '';
		reversingPaymentId = '';
	}

	async function confirmReversal(reason: string) {
		await onReversePayment(reversingInvoiceId, reversingPaymentId, reason);
		cancelReversingPayment();
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
		paymentRecorded = '';
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
	 * The Invoices as a table (#2059, #2146), in the drawing's order: the
	 * Invoice's own number first, since it titles each Invoice's card at a
	 * narrow width and is what a check "for invoice ___" is matched
	 * against, then the day it was raised, the amount, the status as a
	 * Badge, the day it was paid, and the money returned on it. A Paid
	 * Invoice stays Paid after a Refund, as Stripe's own does (#1009), so
	 * Returned is where that money is said at all. A blank cell is left
	 * out of a card.
	 */
	const invoiceColumns = [
		{ label: 'Invoice', accessor: (invoice: Invoice) => invoice.reference },
		{ label: 'Raised', accessor: (invoice: Invoice) => formatCalendarDay(invoice.raisedOn) },
		{ label: 'Amount', accessor: (invoice: Invoice) => formatAmount(invoice.amountCents), numeric: true },
		{ label: 'Status', accessor: (invoice: Invoice) => invoiceRowStatusLabel(invoice), content: invoiceStatus },
		{ label: 'Paid', accessor: (invoice: Invoice) => (invoice.paidOn ? formatCalendarDay(invoice.paidOn) : '') },
		// #2241: how a by-hand Payment was made, and its recorder's note.
		{
			label: 'Paid by',
			accessor: (invoice: Invoice) =>
				invoice.activePaymentMethod ? paymentMethodLabels[invoice.activePaymentMethod] : ''
		},
		{
			label: 'Returned',
			accessor: (invoice: Invoice) => (invoice.refundedCents > 0 ? formatAmount(invoice.refundedCents) : ''),
			numeric: true
		},
		{
			label: 'Note',
			accessor: (invoice: Invoice) => invoice.activePaymentNote ?? '',
			variant: 'muted' as const,
			wraps: true
		}
	];

	/*
	 * A contractor's Invoices (#2094): the day each was raised first, since
	 * it titles each card at a narrow width and is what she checks before
	 * she raises another, then where it stands, when it is due and when it
	 * was paid. No column holds money, because her rows hold none.
	 */
	const contractorInvoiceColumns = [
		{ label: 'Raised', accessor: (invoice: ContractorInvoice) => formatCalendarDay(invoice.raisedOn) },
		{
			label: 'Status',
			accessor: (invoice: ContractorInvoice) => invoiceRowStatusLabel(invoice),
			content: invoiceStatus
		},
		{ label: 'Payment due', accessor: (invoice: ContractorInvoice) => formatCalendarDay(invoice.dueOn) },
		{
			label: 'Paid',
			accessor: (invoice: ContractorInvoice) => (invoice.paidOn ? formatCalendarDay(invoice.paidOn) : '')
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

<!--
	One Badge for both row shapes (#2122): an Owner's, an Admin's or an
	employee Doula's Invoice and a contractor's read the same status,
	Overdue included, from the BFF's one count.
-->
{#snippet invoiceStatus(invoice: InvoiceStanding)}
	<Badge label={invoiceRowStatusLabel(invoice)} variant={invoiceRowStatusVariant(invoice)} />
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

{#if hasCreated}
	<Notice variant="status" message={invoiceCreatedMessage} isFocusedOnAppear />
{/if}

{#if paymentRecorded}
	<Notice variant="status" message={paymentRecorded} isFocusedOnAppear />
{/if}

{#if !canReadInvoiceMoney}
	<!--
		#2094: her rows, dates and status only, so she can see what has
		already been billed before she raises another. No row actions: every
		act on an Invoice is an Owner's or an Admin's.
	-->
	<DataTable
		columns={contractorInvoiceColumns}
		rows={contractorInvoices}
		rowId={invoiceRowId}
		emptyMessage="No Invoices yet."
	/>
	<Notice variant="info" message={contractorInvoicesMessage} />
{:else}
	<!--
		#2059: one row per Invoice, its status a Badge and its actions the
		row's own, in place of a list item holding a sentence and buttons
		(#2017: no list item holds a control). At a narrow width DataTable
		draws each Invoice as a card with its actions at the foot.
	-->
	<div class="invoice-table">
		<DataTable
			columns={invoiceColumns}
			rows={invoices}
			rowId={invoiceRowId}
			rowActions={isOwnerOrAdmin ? { label: 'Actions', content: invoiceActions } : undefined}
			emptyMessage="No Invoices yet."
		/>
	</div>
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
		{#key payingInvoiceId}
			<RecordPaymentForm
				label="Record a payment"
				heading="Record a payment of {formatAmount(invoice.amountCents)}"
				amountCents={invoice.amountCents}
				dateLabel="Date received"
				dateMissingMessage="Enter the date the payment was received"
				{practiceTimezone}
				onConfirm={confirmPayment}
				onCancel={cancelRecordingPayment}
			/>
		{/key}
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
		{#key reversingInvoiceId}
			<ReversePaymentForm
				label="Reverse this payment"
				heading="Reverse a payment of {formatAmount(invoice.amountCents)}"
				amountCents={invoice.amountCents}
				reasonMissingMessage="Enter a reason for reversing this payment"
				confirmLabel="Confirm and reverse"
				onConfirm={confirmReversal}
				onCancel={cancelReversingPayment}
			/>
		{/key}
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
{:else if billingMode === undefined && !isAskingBillingMode}
	<!-- #2106: the first raise is where the billing mode is asked (#271), so
	     the question waits for this press rather than standing on the page. -->
	<div><Button label="Create Invoice" variant="secondary" onClick={openBillingModeQuestion} /></div>
{:else if billingMode === undefined}
	<StackedForm onSubmit={handleCreate}>
		<Heading level={3} text="Create an Invoice" focusOnAppearing />
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
		<Button label="Cancel" variant="secondary" onClick={closeBillingModeQuestion} />
	</StackedForm>
{:else if billingMode === 'stripe' && !clientsCanPay}
	<Notice variant="info" message={cardPaymentsWaitWords.message} />
	{#if cardPaymentsWaitWords.linkLabel}
		<Link href={paymentsSettingsHref} label={cardPaymentsWaitWords.linkLabel} />
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

<style>
	@layer components {
		/* The table's own floor (#2241, #2376), as the Practice landing's
		   tables set theirs. Measured 2026-10-31 by floor.svelte.spec.ts
		   in Chromium: the style guide's Invoices, the Note column at its
		   longest word, need about 1160px, so the table fits from 74rem.
		   Below it, each Invoice is a card, where DataTable's default
		   55rem drew a table that broke words in its headers. */
		.invoice-table {
			container: invoice-table / inline-size;
		}

		.invoice-table > :global(*) {
			font-size: var(--text-body-size);
			--data-table-table-display: none;
			--data-table-records-display: block;
		}

		@container invoice-table (min-width: 74rem) {
			.invoice-table > :global(*) {
				--data-table-table-display: table;
				--data-table-records-display: none;
			}
		}
	}
</style>
