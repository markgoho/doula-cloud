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
	 * An Invoice bills part or all of the Contract (#2829): the form asks the
	 * amount, with what the Contract has not yet billed already in it, and
	 * once every dollar is billed the section says so and offers no Create
	 * Invoice.
	 *
	 * Three preconditions gate the Create Invoice form (#270, following
	 * #255's own three-separate-fields rule), each with its own fixer:
	 *
	 * - contractStatus isn't billableContractStatus (#275) -- Staff
	 *   issues a new Contract, or the Client signs the one that exists.
	 *   Applies whichever rail the Practice bills on.
	 * - billingMode is undefined -- #271's "ask once, inline": the very first
	 *   Invoice raise asks which rail this Practice bills on, before either
	 *   check below applies. Since #2106 the question is asked only when the
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
		nothingLeftToBillMessage,
		type ContractBilling,
		paymentRecordedMessage,
		invoiceRowStatusLabel,
		invoiceRowStatusVariant,
		calendarDayIn,
		termsDueDay,
		weekOfPregnancyDay,
		MIN_INVOICE_WEEK,
		MAX_INVOICE_WEEK,
		type InvoiceDueChoice,
		paymentMethodLabels,
		paymentOnItsWayLine,
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
	import { centsToDollars, dollarsToCents } from '#lib/rates.js';
	import RecordPaymentForm from '#lib/components/organisms/RecordPaymentForm.svelte';
	import RefundPaymentForm from '#lib/components/organisms/RefundPaymentForm.svelte';
	import ReversePaymentForm from '#lib/components/organisms/ReversePaymentForm.svelte';
	import { RefusalError, type FormError } from '#lib/formErrors.js';
	import Badge from '#lib/components/atoms/Badge.svelte';
	import Button from '#lib/components/atoms/Button.svelte';
	import Heading from '#lib/components/atoms/Heading.svelte';
	import Link from '#lib/components/atoms/Link.svelte';
	import Notice from '#lib/components/atoms/Notice.svelte';
	import TextInput from '#lib/components/atoms/TextInput.svelte';
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
		canReadInvoiceMoney = true,
		contractorInvoices = [],
		billing,
		hasUnbilled = true,
		isAskingBillingMode = $bindable(false),
		isChangingDefaults = $bindable(false),
		practiceTimezone,
		paymentTermsDays,
		engagementKind,
		engagementDueDate,
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
		/** The Practice's payment terms (#768): the day Create Invoice's
		 * due question already has chosen (#2828). */
		paymentTermsDays: number;
		/** The Engagement's kind and due date, which decide whether a week
		 * of pregnancy can be offered as the Invoice's due day: only a birth
		 * Engagement that has a due date can (#2828). */
		engagementKind: string;
		engagementDueDate?: string;
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
		/** The Contract's price, what its Invoices have billed and what is
		 * left (#2829). Absent for a contractor Doula, who reads no money,
		 * and while it loads; the amount question is asked only with it. */
		billing?: ContractBilling;
		/** Whether any of the Contract's price is left to bill, as a
		 * contractor Doula is told it (#2829): a fact, never a figure. Read
		 * only when canReadInvoiceMoney is false. */
		hasUnbilled?: boolean;
		/** Whether the billing-mode question is open (#2106): it is on the
		 * page only once Create Invoice has asked for it. Bindable, as
		 * OfferSection's isFormOpen, so a demo can draw it open. */
		isAskingBillingMode?: boolean;
		/** Whether Change has opened the amount and due questions under the
		 * line that states them (#2832). Bindable, so a demo can draw it open. */
		isChangingDefaults?: boolean;
		paymentsSettingsHref: string;
		/** The Invoice a link named (#2240): its row takes focus, once,
		 * when it arrives, so the Invoices list lands on it. */
		landOnInvoiceId?: string;
		onCreate: (billingMode?: BillingMode, due?: InvoiceDueChoice, amountCents?: number) => Promise<void>;
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
	const isNothingLeftToBill = $derived(canReadInvoiceMoney ? billing?.unbilledCents === 0 : !hasUnbilled);

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
		resetDefaults();
	}

	// #2829: how much of the Contract this Invoice bills. What the Contract
	// has not yet billed is in the field until a person types over it, so
	// a press that never reads the question bills what it billed before
	// this question was asked (the whole price, on a first Invoice).
	let typedAmount = $state<string | undefined>();
	let amountError = $state('');
	const amountFieldId = 'invoice-amount';
	const unbilledCents = $derived(billing!.unbilledCents);
	const amountDollars = $derived(typedAmount ?? centsToDollars(unbilledCents));
	const isAskingAmount = $derived(canReadInvoiceMoney && billing !== undefined);

	function resetAmountQuestion() {
		typedAmount = undefined;
		amountError = '';
	}

	function resetDefaults() {
		isChangingDefaults = false;
		resetAmountQuestion();
		resetDueQuestion();
	}

	function toggleDefaults() {
		if (isChangingDefaults) {
			resetDefaults();
		} else {
			isChangingDefaults = true;
		}
	}

	/** The cents the form's amount names, or `undefined` for a contractor,
	 * who is not asked and bills what is left. Sets the field's refusal and
	 * returns `false` when the answer cannot be used. */
	function readAmount(): { ok: true; cents?: number } | { ok: false } {
		if (!isAskingAmount) {
			return { ok: true };
		}
		const cents = dollarsToCents(amountDollars);
		if (cents === undefined) {
			amountError = 'Enter an amount to bill greater than $0.00';
			return { ok: false };
		}
		if (cents > unbilledCents) {
			amountError = `Amount to bill must be ${formatAmount(unbilledCents)} or less`;
			return { ok: false };
		}
		return { ok: true, cents };
	}

	// #2828: when the Invoice is due. The Practice's payment terms are
	// chosen when the form opens, a recorded departure from "nothing
	// pre-filled" (ADR-0038's 2026-11-10 amendment, govuk-alignment.md): it
	// is the answer every Invoice has had, so a press that never reads the
	// question raises exactly what it raised before.
	type DueWay = 'terms' | 'date' | 'week';
	let dueWay = $state<DueWay>('terms');
	let dueDate = $state('');
	let dueWeek = $state('');
	let dueDateError = $state('');
	let dueWeekError = $state('');

	function resetDueQuestion() {
		dueWay = 'terms';
		dueDate = '';
		dueWeek = '';
		dueDateError = '';
		dueWeekError = '';
	}

	const dueWayName = 'invoice-due-way';
	const dueDateFieldId = 'invoice-due-date';
	const dueWeekFieldId = 'invoice-due-week';
	const canOfferWeek = $derived(engagementKind === 'birth' && !!engagementDueDate);
	// Read once when the form is drawn: the terms' day is today's date
	// plus the terms, and a form left open across midnight is refused by
	// the BFF, not silently moved.
	const termsDay = $derived(termsDueDay(practiceTimezone, new Date(), paymentTermsDays));
	const todayDay = $derived(calendarDayIn(practiceTimezone, new Date()));
	const chosenWeek = $derived.by(() => {
		const week = Number(dueWeek);
		const isUsable = dueWeek.trim() !== '' && Number.isSafeInteger(week) && week >= MIN_INVOICE_WEEK && week <= MAX_INVOICE_WEEK;
		return isUsable ? week : undefined;
	});
	const weekDay = $derived(
		chosenWeek !== undefined && !!engagementDueDate
			? weekOfPregnancyDay(engagementDueDate, chosenWeek)
			: undefined
	);
	const weekHint = $derived.by(() => {
		if (weekDay !== undefined) {
			return `Week ${chosenWeek} is ${formatCalendarDay(weekDay)}.`;
		}
		return `Week 40 is the due date, ${formatCalendarDay(engagementDueDate!)}.`;
	});
	const defaultsRows = $derived([
		{
			label: isAskingAmount ? 'Amount and due day' : 'Due day',
			value: isAskingAmount
				? `${formatAmount(unbilledCents)}, due ${formatCalendarDay(termsDay)}`
				: `Due ${formatCalendarDay(termsDay)}`,
			changes: isAskingAmount ? 'the amount and the due day' : 'the due day',
			isDetailShown: isChangingDefaults
		}
	]);
	const dueOptions = $derived([
		{
			value: 'terms' as const,
			label: "After this Practice's payment terms.",
			description: `${paymentTermsDays} ${paymentTermsDays === 1 ? 'day' : 'days'} from today: ${formatCalendarDay(termsDay)}.`
		},
		{ value: 'date' as const, label: 'On a date.', conditional: dueDateField },
		...(canOfferWeek
			? [{ value: 'week' as const, label: 'At a week of pregnancy.', conditional: dueWeekField }]
			: [])
	]);

	/** The due day the form's answer names, for onCreate; `undefined` for
	 * the terms, which the BFF counts itself. Sets the field's refusal and
	 * returns `false` when the answer cannot be used. */
	function readDueChoice(): { ok: true; due?: InvoiceDueChoice } | { ok: false } {
		if (dueWay === 'date') {
			if (dueDate.trim() === '') {
				dueDateError = 'Enter the day this Invoice is due';
				return { ok: false };
			}
			if (dueDate < todayDay) {
				dueDateError = 'The due day cannot be before today';
				return { ok: false };
			}
			return { ok: true, due: { dueOn: dueDate } };
		}
		if (dueWay === 'week') {
			if (chosenWeek === undefined || weekDay === undefined) {
				dueWeekError = `Enter a week of pregnancy from ${MIN_INVOICE_WEEK} to ${MAX_INVOICE_WEEK}`;
				return { ok: false };
			}
			if (weekDay < todayDay) {
				dueWeekError = `Week ${chosenWeek} is before today. Enter a later week or choose a date`;
				return { ok: false };
			}
			return { ok: true, due: { gestationalWeek: chosenWeek } };
		}
		return { ok: true };
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
			amountError ? { message: amountError, targetId: amountFieldId } : undefined,
			dueDateError ? { message: dueDateError, targetId: dueDateFieldId } : undefined,
			dueWeekError ? { message: dueWeekError, targetId: dueWeekFieldId } : undefined,
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
		if (error_ instanceof RefusalError && error_.details?.amountCents) {
			isChangingDefaults = true;
			amountError = error_.details.amountCents;
			return;
		}
		if (error_ instanceof RefusalError && (error_.details?.dueOn || error_.details?.gestationalWeek)) {
			const refused = error_.details.dueOn ?? error_.details.gestationalWeek!;
			isChangingDefaults = true;
			if (dueWay === 'week') {
				dueWeekError = refused;
			} else {
				dueDateError = refused;
			}
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

	async function handleCreate(event: SubmitEvent) {
		event.preventDefault();
		createError = undefined;
		billingModeError = '';
		amountError = '';
		dueDateError = '';
		dueWeekError = '';
		hasCreated = false;
		paymentRecorded = '';

		if (billingMode === undefined && chosenBillingMode === undefined) {
			billingModeError = 'Select how this Practice bills Clients';
		}
		const amount = isChangingDefaults ? readAmount() : { ok: true as const };
		const dueChoice = readDueChoice();
		if (billingModeError || !amount.ok || !dueChoice.ok) {
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
			await onCreate(billingMode === undefined ? chosenBillingMode : undefined, dueChoice.due, amount.cents);
			isAskingBillingMode = false;
			resetDefaults();
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
	 * Refunded is where that money is said at all. A blank cell is left
	 * out of a card.
	 */
	const invoiceColumns = [
		{ label: 'Invoice', accessor: (invoice: Invoice) => invoice.reference },
		{ label: 'Sent', accessor: (invoice: Invoice) => formatCalendarDay(invoice.raisedOn) },
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
			label: 'Refunded',
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
		{ label: 'Sent', accessor: (invoice: ContractorInvoice) => formatCalendarDay(invoice.raisedOn) },
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
		{#if !invoice.paymentProcessing}
			<Button
				label="Record payment"
				variant="secondary"
				size="sm"
				{describedBy}
				onClick={() => startRecordingPayment(invoice.id)}
			/>
		{:else}
			<span>{paymentOnItsWayLine}</span>
		{/if}
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
	<!-- What may still be done with the Payment is the BFF's answer
	     (refundableCents, reversible), asked of the same declarations its
	     refund and reverse routes ask, so no button is offered that would
	     only earn a refusal. -->
	{#if isOwnerOrAdmin && invoice.activePaymentId}
		{@const activePaymentId = invoice.activePaymentId}
		{#if invoice.refundableCents > 0}
			<Button
				label="Return money"
				variant="secondary"
				size="sm"
				{describedBy}
				onClick={() => startRefundingPayment(invoice.id, activePaymentId)}
			/>
		{/if}
		{#if invoice.reversible}
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
	{#if billing}
		<SummaryList
			rows={[
				{ label: 'Contract price', value: formatAmount(billing.priceCents) },
				{ label: 'Billed', value: formatAmount(billing.billedCents) },
				{ label: 'Not yet billed', value: formatAmount(billing.unbilledCents) }
			]}
		/>
	{/if}
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

{#snippet dueDateField()}
	<LabeledField id={dueDateFieldId} label="Due on" error={dueDateError || undefined}>
		{#snippet children({ id, describedBy, invalid })}
			<TextInput {id} {describedBy} {invalid} type="date" value={dueDate} onInput={(value) => (dueDate = value)} required />
		{/snippet}
	</LabeledField>
{/snippet}

{#snippet dueWeekField()}
	<LabeledField
		id={dueWeekFieldId}
		label="Week of pregnancy"
		hint={weekHint}
		error={dueWeekError || undefined}
		width={2}
	>
		{#snippet children({ id, describedBy, invalid, width })}
			<TextInput
				{id}
				{describedBy}
				{invalid}
				{width}
				type="number"
				min={MIN_INVOICE_WEEK}
				value={dueWeek}
				onInput={(value) => (dueWeek = value)}
				required
			/>
		{/snippet}
	</LabeledField>
{/snippet}

{#snippet amountQuestion()}
	{#if isAskingAmount}
		<LabeledField
			id={amountFieldId}
			label="Amount to bill (USD)"
			hint="Up to {formatAmount(unbilledCents)} is left to bill on this Contract."
			error={amountError || undefined}
			width={10}
		>
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
					onInput={(value) => (typedAmount = value)}
				/>
			{/snippet}
		</LabeledField>
	{/if}
{/snippet}

{#snippet invoiceDefaults()}
	<SummaryList rows={defaultsRows}>
		{#snippet action(row, describedBy)}
			<Button label="Change" variant="link" expanded={row.isDetailShown} {describedBy} onClick={toggleDefaults} />
		{/snippet}
		{#snippet detail()}
			{@render amountQuestion()}
			{@render dueQuestion()}
		{/snippet}
	</SummaryList>
{/snippet}

{#snippet dueQuestion()}
	<RadioGroup
		legend="When is this Invoice due?"
		name={dueWayName}
		options={dueOptions}
		value={dueWay}
		onChange={(value) => (dueWay = value)}
	/>
{/snippet}

{#if !isBillable}
	<Notice variant="info" message={unbillableContractMessage(contractStatus)} />
{:else if isNothingLeftToBill}
	<Notice variant="info" message={nothingLeftToBillMessage} />
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
		{@render invoiceDefaults()}
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
		{@render invoiceDefaults()}
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
