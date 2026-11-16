<script lang="ts">
	import { onDestroy, onMount, tick, untrack, type Snippet } from 'svelte';
	import type { ListBack } from '#lib/listOrigin.svelte.js';
	import { page } from '#lib/appState.svelte.js';
	import { apiFetchWithSession } from '#lib/api.js';
	import { loadClientDetail, type ClientDetail } from '#lib/clientDetail.js';
	import { canAddDetails, detailsLinkLabel } from '#lib/clientDetailsJourney.js';
	import { detailsHref } from '../../clients/[clientId]/details/details.js';
	import { PaginatedList } from '#lib/paginatedList.svelte.js';
	import { SectionState } from '#lib/sectionState.svelte.js';
	import { FormSubmission, orThrownErrors, orThrownMessage } from '#lib/formSubmission.svelte.js';
	import { triggerBlobDownload } from '#lib/blobDownload.js';
	import {
		attachDoula,
		attachDoulaWords,
		attachSelf,
		takeOffDoula,
		takeOffWords,
		takenOffMessage,
		visitDoulaText,
		changeEngagementDueDate,
		changeEngagementKind,
		changeEngagementStatus,
		createVisit,
		doulaSummaryItem,
		downloadAttachment,
		endingReasons,
		OUTCOME_BEFORE_COMPLETION,
		loadAttachmentPreviews,
		loadEngagement,
		loadEngagementOffersOrNone as loadOffers,
		loadMessagesPage,
		loadVisitsPage,
		reassignVisit,
		recordBirthOutcome,
		saveVisitNotes,
		scheduleVisit,
		sendMessage,
		sendPortalInvite,
		sortVisits,
		visitGroup,
		visitTypeLabel,
		engagementStatusText,
		engagementURL,
		yourFeeRows,
		type BirthOutcomeFacts,
		type BirthOutcomeRequest,
		type EngagementSummary,
		type Visit,
		type WhoIsOn
	} from '#lib/engagementDetail.js';
	import { isDoulaSelect, kindLabel } from '#lib/engagementRequest.js';
	import {
		assigneeBlock,
		attachableDoulaOptions,
		loadVisitAssigneesOrNone,
		unnameableHint,
		visitAssigneeOptions,
		type VisitAssignee
	} from '#lib/staff.js';
	import {
		canReachPortal,
		hasAcceptedPortalInvite,
		hasNeverBeenInvited,
		isRefusedInvite,
		portalInviteStatusText,
		refusedInviteWayOn,
		type PortalInviteSubject
	} from '#lib/portalInvite.js';
	import { isErased } from '#lib/clientStanding.js';
	import type { PageData } from './$types';
	import { stampText } from '#lib/stamp.js';
	import { landOnRow, landOnSection } from '#lib/landOnRow.js';
	import { unansweredSince } from '#lib/threadSummary.js';
	import { formatGapTimes } from '#lib/coverageGap.js';
	import CoverageGapWhen from '#lib/components/molecules/CoverageGapWhen.svelte';
	import { formatActivityTimestamp, formatCalendarDay, formatCalendarDayThisYear, formatCalendarWeekdayDay, formatInstant, weeksAlong, formatInstantWithClock, formatScheduledVisit, toDatetimeLocalValue } from '#lib/dates.js';
	import { dayIn, instantFromWall, readerTimeZone, zoneNote } from '#lib/zonedTime.js';
	import { formatAddress, formatPhoneNumber } from '#lib/contactDetails.js';
	import { sectionOrder, type SectionHeading } from './sectionOrder.js';
	import {
		ACTIVITY_KINDS,
		activityLedgerColumns,
		loadEngagementActivityPage,
		type ActivityEntry,
		type ActivityKind
	} from '#lib/activityLedger.js';
	import { subscribeToThreadPushMessages } from '#lib/pushRefresh.js';
	import PlanInstanceForm from '#lib/components/organisms/PlanInstanceForm.svelte';
	import {
		loadInstance,
		createInstance,
		saveAnswers,
		setAnswer,
		toggleMultiSelectOption,
		planProgress,
		type Answers,
		type Instance
	} from '#lib/planInstance.js';
	import ContractForm from '#lib/components/molecules/ContractForm.svelte';
	import EarlierContracts from '#lib/components/molecules/EarlierContracts.svelte';
	import ContractStatus from '#lib/components/molecules/ContractStatus.svelte';
	import ContractView from '#lib/components/molecules/ContractView.svelte';
	import {
		loadContract,
		createContract,
		saveContractValues,
		sendContract,
		remindContract,
		voidContract,
		requestContractVoid,
		declineContractVoidRequest,
		downloadSignedContractPdf,
		setMergeFieldValue,
		mergeFieldLabel,
		contractStatusText,
		missingMergeFieldKeys,
		editableMergeFields,
		editableValues,
		type Contract
	} from '#lib/contract.js';
	import { isAmbientContractor, isDoula, isOwner, isOwnerOrAdmin, listEntryHref } from '#lib/roles.js';
	import BirthOutcomeSection from '#lib/components/organisms/BirthOutcomeSection.svelte';
	import InvoiceSection from '#lib/components/organisms/InvoiceSection.svelte';
	import {
		loadInvoices,
		loadContractorInvoices,
		createInvoice,
		loadBillingMode,
		recordPayment,
		reversePayment,
		refundPayment,
		voidInvoice,
		writeOffInvoice,
		loadPaymentTerms,
		type BillingMode,
		billingAfterRaising,
		type ContractBilling,
		type ContractorInvoice,
		type ContractorOpenSummary,
		formatAmount,
		type OwedSummary,
		type Invoice,
		type InvoiceDueChoice,
		type PaymentMethod,
		type RecordPaymentInput,
		type RefundPaymentInput
	} from '#lib/invoice.js';
	import { loadRates, type Rate } from '#lib/rates.js';
	import OfferSection from '#lib/components/organisms/OfferSection.svelte';
	import DoulaFeeSection from '#lib/components/organisms/DoulaFeeSection.svelte';
	import { loadDoulaFees, recordDoulaFeePaid, removeDoulaFeePaid, type DoulaFee } from '#lib/doulaFee.js';
	import { createOffer, loadEngagementOffers, withdrawOffer, type NewOffer, type Offer } from '#lib/offer.js';
	import { goto } from '$app/navigation';
	import { resolve } from '$app/paths';
	import MessageThread, { type Message } from '#lib/components/organisms/MessageThread.svelte';
	import Text from '#lib/components/atoms/Text.svelte';
	import Link from '#lib/components/atoms/Link.svelte';
	import Button from '#lib/components/atoms/Button.svelte';
	import Notice from '#lib/components/atoms/Notice.svelte';
	import SummaryList, { type SummaryListRow } from '#lib/components/molecules/SummaryList.svelte';
	import PlanInstanceView from '#lib/components/molecules/PlanInstanceView.svelte';
	import VisitDoula from '#lib/components/molecules/VisitDoula.svelte';
	import VisitWhen from '#lib/components/molecules/VisitWhen.svelte';
	import RadioGroup, { radioFieldId } from '#lib/components/molecules/RadioGroup.svelte';
	import ConfirmDialog from '#lib/components/molecules/ConfirmDialog.svelte';
	import DateFields from '#lib/components/molecules/DateFields.svelte';
	import {
		EMPTY_DATE_PARTS,
		dateFieldId,
		dateGroupRefusal,
		joinCalendarDay,
		splitDate,
		type DateParts
	} from '#lib/intakeDate.js';
	import ErrorSummary from '#lib/components/molecules/ErrorSummary.svelte';
	import type { FormError } from '#lib/formErrors.js';
	import DataTable, { type DataTableView } from '#lib/components/organisms/DataTable.svelte';
	import RecordDetail from '#lib/components/templates/RecordDetail.svelte';
	import TextInput, { INPUT_WIDTHS, type InputWidth } from '#lib/components/atoms/TextInput.svelte';
	import Textarea from '#lib/components/atoms/Textarea.svelte';
	import LabeledField from '#lib/components/molecules/LabeledField.svelte';
	import StackedForm from '#lib/components/molecules/StackedForm.svelte';
	import Select from '#lib/components/atoms/Select.svelte';
	import Badge from '#lib/components/atoms/Badge.svelte';
	import Icon from '#lib/components/atoms/Icon.svelte';
	import EngagementStatusLine from './EngagementStatusLine.svelte';
	import Heading from '#lib/components/atoms/Heading.svelte';
	import Skeleton from '#lib/components/atoms/Skeleton.svelte';
	import EmptyState from '#lib/components/molecules/EmptyState.svelte';
	import Details from '#lib/components/atoms/Details.svelte';
	import {
		clearGap,
		describeNoWindow,
		loadEngagementOnCall,
		saveGap,
		saveNarrowing,
		type CoverageGap,
		type EngagementOnCall
	} from '#lib/onCall.js';

	// The Engagement comes from +page.ts's read, once it lands (#695,
	// #2249); +page.svelte draws the loading state and a refusal. The
	// other six sections still load after mount, each rendering as it
	// lands.
	//
	// `outcome` is what the save that led here did, if one did (#1710): an
	// approved start from the Start work form, or her details journey's
	// save. +page.svelte takes it, because it is handed over only while
	// the navigation runs, and that ends before this screen mounts.
	let {
		data,
		outcome,
		back
	}: { data: Omit<PageData, 'read'> & EngagementSummary; outcome?: string; back?: ListBack } = $props();
	const detail = $derived(data);

	// The Contract's PDF download opens to an Owner, an Admin, and an
	// employed Doula (ADR-0008's money row as amended by #282) and refuses
	// only a contractor -- this drawing decision is not the gate:
	// contract.ts's downloadSignedContractPdf hits the real endpoint,
	// which refuses a contractor on its own.
	const isPracticeOwnerOrAdmin = $derived(isOwnerOrAdmin(data.session));
	// A Visit's day and time are the Practice's own (ADR-0036), here as on the Schedule.
	const timeZone = $derived(data.session.timezone);
	const canReadContractMoney = $derived(!isAmbientContractor(data.session));

	// InvoiceSection's Owner branch (#270): only an Owner gets the link to
	// the Payments settings screen, matching PostConnectHandler's own
	// Owner-only gate.
	const isPracticeOwner = $derived(isOwner(data.session));
	const paymentsSettingsHref = $derived(
		resolve('/practices/[practiceId]/settings/payments', { practiceId: page.params.practiceId! })
	);
	// #2240: an Invoices list row links here with `#invoice-<id>`.
	const landOnInvoiceId = $derived(/^#invoice-(.+)$/.exec(page.url.hash)?.[1]);
	const isLandingOnContract = $derived(/^#(contract|void-request)-/.test(page.url.hash));

	// #1998: the drawing's "← Clients" back link, to the Practice's
	// Clients list, where every Engagement is reached from.
	const clientsHref = $derived(
		listEntryHref(resolve('/practices/[practiceId]/clients', { practiceId: page.params.practiceId! }), data.session)
	);

	// #280: the Birth Plan's own address -- a reading and taking-away
	// surface, distinct from this page's own editing form below.
	const birthPlanHref = $derived(
		resolve('/practices/[practiceId]/engagements/[engagementId]/birth-plan', {
			practiceId: page.params.practiceId!,
			engagementId: page.params.engagementId!
		})
	);

	// The reference every read on this page is about. Derived rather than
	// captured, so a client-side navigation to a sibling Engagement is
	// picked up by the reads below.
	const reference = $derived({
		practiceId: page.params.practiceId!,
		engagementId: page.params.engagementId!
	});

	// #253: the Engagement's status and its next legal moves, overlaid on
	// the load-time read once a move succeeds -- so the page reflects it
	// without a reload. Written by both paths below (the bare-click moves
	// and the completing form), which is why it is one plain $state
	// rather than living inside either path's own submission object.
	// undefined means "no move made yet", in which case
	// `displayStatus`/`displayStatusMoves` fall back to `detail` itself.
	let statusOverride = $state<{ status: string; statusMoves: string[] } | undefined>();
	const displayStatus = $derived(statusOverride?.status ?? detail?.status ?? '');
	const displayStatusMoves = $derived(statusOverride?.statusMoves ?? detail?.statusMoves ?? []);

	// intake -> active and the reopen correction are bare commands, not a
	// form -- nothing is asked of the caller, so this stays SectionState +
	// Notice, the same shape every other row control on this page uses
	// (reassign, schedule, notes).
	const directMove = new SectionState<void>(undefined);

	// #1998: the summary card's Change (GOV.UK's Summary list row action).
	// Each opens an in-place form under its own row, holding the controls
	// that used to sit in a loose stack under the facts: the care-phase
	// moves, the kind change, and "Put me on this work". One row is
	// open at a time. A save closes it and says what changed in a status
	// that takes focus, because the control she pressed goes with the
	// form; a refusal leaves it open, beside the refusal.
	type SummaryChange = 'care' | 'due' | 'carePhase' | 'doula';
	// `isInvite` is the Portal invite row, whose action sends the invite; `contact`
	// makes the Client's phone or email the link that calls or writes her.
	type EngagementSummaryRow = SummaryListRow & {
		change?: SummaryChange;
		isInvite?: boolean;
		contact?: 'tel' | 'mailto';
		/**
		What follows `tel:` or `mailto:`, where it is not the words shown.
		*/
		href?: string;
	};
	const changeWords: Record<SummaryChange, string> = {
		care: 'care',
		due: 'due date',
		carePhase: 'care phase',
		doula: 'Doula'
	};
	let openChange = $state<SummaryChange | undefined>();
	let statusChangedMessage = $state('');

	// Cancel hands focus back to the link that opened the form (#2103).
	let refocusedChange = $state<SummaryChange | undefined>();

	function cancelChange(change: SummaryChange) {
		openChange = undefined;
		refocusedChange = change;
	}

	function toggleChange(change: SummaryChange) {
		refocusedChange = undefined;
		openChange = openChange === change ? undefined : change;
		statusChangedMessage = '';
		kindChangedMessage = '';
		selfAttachedMessage = '';
		dueChangedMessage = '';
		attachSubmission.errors = [];
		if (openChange === 'due') openDueForm();
	}

	function closeChange(message: string) {
		openChange = undefined;
		statusChangedMessage = message;
	}

	async function handleStatusMoveClick(move: string) {
		if (move === 'completed') {
			isCompleteFormShown = true;
			return;
		}
		isCompleteFormShown = false;
		const hasMoved = await directMove.mutate(async () => {
			statusOverride = await changeEngagementStatus(apiFetchWithSession, reference, move);
		}, 'We could not change the status.');
		if (!hasMoved) return;
		closeChange(`The care phase is now ${engagementStatusText(displayStatus)}.`);
		await refreshWhoIsOn();
		await loadOnCall();
		await loadDoulaFeesSection();
		await loadActivity();
	}

	// Completing asks for a reason -- GOV.UK's question-page pattern
	// (ADR-0021), the same FormSubmission/ErrorSummary shape
	// engagement-requests/new's own Refuse-shaped radio group uses, not a
	// hand-rolled error string: `errorFor` is what lets RadioGroup show
	// the same wording ErrorSummary lists at the top.
	const completeSubmission = new FormSubmission();
	let isCompleteFormShown = $state(false);
	let completeReasonValue = $state('');
	let completeNoteValue = $state('');
	const endingReasonFieldId = `ending-reason-${endingReasons[0]!.value}`;
	// Keyed by the BFF's own json tags (engagement.TransitionRequest), so
	// a refusal it names lands on the control it is about (#488). `status`
	// has no control -- the two paths are named buttons -- so a refusal
	// naming it stays an untargeted summary entry.
	const completeFieldIds = { endingReason: endingReasonFieldId };

	async function handleCompleteSubmit(event: SubmitEvent) {
		event.preventDefault();
		await completeSubmission.run(async () => {
			if (!completeReasonValue) {
				return [{ message: 'Select why care is ending', targetId: endingReasonFieldId }];
			}
			statusOverride = await changeEngagementStatus(
				apiFetchWithSession,
				reference,
				'completed',
				completeReasonValue,
				completeNoteValue || undefined
			);
			isCompleteFormShown = false;
			completeReasonValue = '';
			completeNoteValue = '';
			closeChange(`The care phase is now ${engagementStatusText(displayStatus)}.`);
			// #2270: completion ends every Attachment, so On call is read
			// again too, and no longer lists a Doula who is off the birth.
			// So are the Doula fees (#2605), whose rows read each ended
			// Attachment's span.
			await refreshWhoIsOn();
			await loadOnCall();
			await loadDoulaFeesSection();
			await loadActivity();
		}, orThrownErrors(completeFieldIds));
	}

	// The app-side mirror of api/internal/engagement/transition.go's own
	// refuseFactWrite: a contractor Doula may not write any of ADR-0015's
	// mutable Engagement facts, and neither may a member holding none of
	// the three roles. Shared by the birth-outcome and kind sections
	// below, which both gate on this exact role table. Drawing only,
	// never the gate -- the endpoint refuses her whether or not the
	// control was drawn (ADR-0006).
	const canWriteFact = $derived(
		!isAmbientContractor(data.session) && (isPracticeOwnerOrAdmin || isDoula(data.session))
	);

	// #943: what happened to the pregnancy, overlaid on the load-time read
	// once a record succeeds, exactly as statusOverride above does -- so
	// the section reads back the new pair without a reload. `undefined`
	// means "no write on this page view", which falls back to `detail`.
	// The overlay holds the whole pair rather than the outcome alone,
	// because a *cleared* pair is a write whose result is two absences,
	// and two separate overrides could not tell that apart from "nothing
	// written yet" (recordBirthOutcome normalizes the endpoint's own
	// `null`s to absent before this ever sees them).
	let birthOutcomeOverride = $state<BirthOutcomeFacts | undefined>();
	const birthOutcome = $derived<BirthOutcomeFacts>(
		birthOutcomeOverride ?? {
			birthOutcome: detail?.birthOutcome,
			pregnancyEndedOn: detail?.pregnancyEndedOn
		}
	);

	async function handleRecordBirthOutcome(
		request: BirthOutcomeRequest,
		fieldIds: Record<string, string>
	) {
		const result = await recordBirthOutcome(apiFetchWithSession, reference, request, fieldIds);
		if (result.kind === 'recorded') {
			birthOutcomeOverride = result.facts;
			void refreshBirthPlanOffer();
			void loadActivity();
		}
		return result;
	}

	// #2277, #2317: whether a Birth Plan can be started, and whether the
	// Client is shown one, as the BFF answers it. A kind change or a
	// recorded outcome on this page view reads the answer again rather
	// than re-spelling ADR-0015's rule here.
	let birthPlanOfferOverride = $state<Pick<EngagementSummary, 'offersBirthPlanToStart' | 'offersBirthPlan'>>();
	const birthPlanOffer = $derived(
		birthPlanOfferOverride ?? {
			offersBirthPlanToStart: detail.offersBirthPlanToStart,
			offersBirthPlan: detail.offersBirthPlan
		}
	);

	const NO_BIRTH_PLAN_ON_POSTPARTUM = 'No Birth Plan. This is postpartum care.';
	const NO_BIRTH_PLAN_AFTER_OUTCOME = 'No Birth Plan. The birth outcome is recorded.';

	async function refreshBirthPlanOffer() {
		try {
			const response = await apiFetchWithSession(engagementURL(reference));
			if (!response.ok) return;
			const fresh: EngagementSummary = await response.json();
			birthPlanOfferOverride = {
				offersBirthPlanToStart: fresh.offersBirthPlanToStart,
				offersBirthPlan: fresh.offersBirthPlan
			};
		} catch {
			// The write refuses a stale Start on its own.
		}
	}

	// #874: what the Practice sold, overlaid on the load-time read once a
	// change succeeds -- the same statusOverride shape above. ADR-0015:
	// mutable in both directions while the pregnancy is still expected, so
	// this is a bare command (no form asks anything), the same shape the
	// status section's own reopen/activate moves use.
	const kindChange = new SectionState<void>(undefined);
	let kindOverride = $state<string | undefined>();
	let kindChangedMessage = $state('');
	const displayKind = $derived(kindOverride ?? detail?.kind ?? '');
	const otherKind = $derived(displayKind === 'birth' ? 'postpartum' : 'birth');
	// ADR-0015's other kind rule, beside the role gate every mutable fact
	// shares: "the product stops offering a postpartum -> birth change
	// once the birth outcome is recorded, because attending a birth that
	// has already happened means nothing." Reads the same overlaid
	// `birthOutcome` the section above renders, so an outcome recorded
	// earlier on this same page view hides the button without a reload --
	// the endpoint refuses the write either way (refuseUpgradeAfterBirth);
	// this only decides whether the control is drawn. The downgrade,
	// 'birth' -> 'postpartum', carries no such caveat.
	const canChangeKind = $derived(
		canWriteFact && (displayKind === 'birth' || birthOutcome.birthOutcome === undefined)
	);

	async function handleChangeKind() {
		kindChangedMessage = '';
		const target = otherKind;
		const isChanged = await kindChange.mutate(async () => {
			const result = await changeEngagementKind(apiFetchWithSession, reference, target);
			kindOverride = result.kind;
			void refreshBirthPlanOffer();
			openChange = undefined;
			kindChangedMessage = `Now set to ${kindLabel(result.kind)}.`;
		}, 'We could not change what the Practice sold.');
		if (isChanged) await loadActivity();
	}

	// #2013: the due date, overlaid on the load-time read once a change
	// succeeds -- the kindOverride shape above. A dating scan or a
	// midwife's revised estimate moves it in real care. It is a question,
	// not a command, so it is a form: GOV.UK's date input (DateFields),
	// with the boxes starting on the date held now. The role gate is
	// canWriteFact, the one every mutable fact shares; the endpoint
	// refuses a contractor Doula whatever is drawn. Unlike kind, a
	// recorded birth outcome does not take the Change away: ADR-0015
	// freezes the outcome and its date, not the due date.
	let dueDateOverride = $state<string | undefined>();
	const displayDueDate = $derived(dueDateOverride ?? detail?.dueDate);
	let dueChangedMessage = $state('');
	let dueParts = $state<DateParts>({ ...EMPTY_DATE_PARTS });
	const dueSubmission = new FormSubmission();
	const DUE_DATE_NAME = 'due-date';
	// Keyed by engagement.DueDateChangeRequest's json tag (#488), aimed at
	// the first box: a <fieldset> is not focusable.
	const dueFieldIds = { dueDate: dateFieldId(DUE_DATE_NAME, 'month') };
	const dueRefusal = $derived(dateGroupRefusal(dueSubmission.errors, DUE_DATE_NAME));

	function openDueForm() {
		dueParts = displayDueDate === undefined ? { ...EMPTY_DATE_PARTS } : splitDate(displayDueDate);
		dueSubmission.errors = [];
	}

	async function handleDueSubmit(event: SubmitEvent) {
		event.preventDefault();
		let isSaved = false;
		await dueSubmission.run(async () => {
			const date = joinCalendarDay(dueParts, 'The due date');
			if (!date.ok) return [{ message: date.message, targetId: dateFieldId(DUE_DATE_NAME, date.field) }];
			if (date.value === '') {
				return [{ message: 'Enter the due date', targetId: dateFieldId(DUE_DATE_NAME, 'month') }];
			}
			const result = await changeEngagementDueDate(apiFetchWithSession, reference, date.value);
			dueDateOverride = result.dueDate;
			openChange = undefined;
			dueChangedMessage = `The due date is now ${formatCalendarDay(result.dueDate)}.`;
			isSaved = true;
		}, orThrownErrors(dueFieldIds));
		if (!isSaved) return;
		// The On-call window is drawn from the due date, and the ledger has
		// a new entry: both sections are read again, each with its own
		// error state, the same pair handleAttachSelf re-reads.
		await loadOnCall();
		await loadActivity();
	}

	// #1598: who is on this Engagement, overlaid on the load-time read the
	// way statusOverride is. The BFF decides both halves -- the names, and
	// whether "Put me on this work" is hers to press
	// (engagement.canAttachSelf, the function its own write asks) -- and
	// this page only draws them. `undefined` means "nothing on this page
	// view has changed who is on it".
	let whoIsOnOverride = $state<WhoIsOn | undefined>();
	const attachedDoulas = $derived(whoIsOnOverride?.doulas ?? detail.doulas);
	const canAttachSelf = $derived(whoIsOnOverride?.canAttachSelf ?? detail.canAttachSelf);

	// A bare command, the shape the kind change above has: nothing is
	// asked of her, so it is one button, SectionState and a Notice, and
	// not a form. One press is the whole act (Mark's decision on #1515,
	// Q7), so there is no confirmation step either: she puts herself on
	// work at her own Practice, and the ledger below records it.
	const selfAttach = new SectionState<void>(undefined);
	let selfAttachedMessage = $state('');

	async function handleAttachSelf() {
		selfAttachedMessage = '';
		const isAttached = await selfAttach.mutate(async () => {
			whoIsOnOverride = await attachSelf(apiFetchWithSession, reference);
			openChange = undefined;
			selfAttachedMessage = 'You are on this work.';
		}, 'We could not put you on this work. Try again.');
		if (!isAttached) return;
		// A birth with a Doula on it can have an On-call window, and the
		// ledger has a new entry: both sections are re-read, each with its
		// own error state.
		await loadOnCall();
		await loadActivity();
	}

	// Three other writes on this page change who is on the Engagement, and
	// none of them answers with it: a Visit that names an employee grants
	// her an Attachment (create and reassign), completion ends every
	// Attachment, and a reopen leaves them ended. So the Engagement is read
	// again after each. A failed re-read is swallowed for the reason
	// refreshStatusAfterScheduling gives: the write succeeded and is
	// reported, and a stale control corrects itself -- the press of a
	// person already on it is a 200 that answers with the true list.
	async function refreshWhoIsOn() {
		try {
			const { doulas: refreshedDoulas, canAttachSelf: refreshedCanAttach } = await loadEngagement(
				apiFetchWithSession,
				reference
			);
			whoIsOnOverride = { doulas: refreshedDoulas, canAttachSelf: refreshedCanAttach };
			// "You are on this work." was true when it was said. A
			// later write can end her Attachment (completion), so the
			// message goes when who is on it is read again.
			selfAttachedMessage = '';
		} catch {
			// Left as it was; a reload reads who is on it.
		}
	}

	/** The label a status-move button carries -- "Reopen" reads as a
	 * correction (ADR-0015: "Reopen is undo, not resumption"), never as
	 * the same word a first activation uses, even though both moves land
	 * on 'active'. */
	function statusMoveLabel(move: string): string {
		if (move === 'completed') return 'Mark care complete';
		return displayStatus === 'completed' ? 'Reopen (correction)' : 'Mark care as active';
	}

	// Visits are newest-logged-first from the BFF (#446); a further page is
	// appended to what is already loaded, and the table shows them sorted
	// by date (`sortedVisits`, #1999). Loading the first page and adding a
	// Visit are separate SectionState instances (#841) so neither action's
	// busy flag lights up the other's control; a failed load says so above
	// the table, a refused add in the add form.
	const visits = new PaginatedList<Visit>({
		first: { items: [], hasMore: false },
		loadPage: (cursor) => loadVisitsPage(apiFetchWithSession, reference, cursor),
		failureMessage: 'We could not load more Visits.'
	});
	const visitsLoad = new SectionState<void>(undefined);
	// The BFF's own "next" (#2755), over every Visit and not the rows loaded.
	let nextVisit = $state<Visit | undefined>();
	// The BFF's own "last" (#2991): the newest Visit that has happened, named
	// in the state line so the notes a Doula wrote last time are one press away.
	let lastVisit = $state<Visit | undefined>();
	const visitsCreate = new SectionState<void>(undefined);
	const isCreatingVisit = $derived(visitsCreate.isBusy);
	// #250: optional at creation, the same as it is afterward -- a Doula
	// logging a meeting that already happened leaves this blank.
	let newVisitScheduledAt = $state('');
	// #268: who the new Visit is for. Empty until she picks somebody, and
	// only ever rendered for a reader who has a roster to pick from -- a
	// plain Doula sends no assignee at all and the Visit is logged for
	// her, exactly as it was before this field existed.
	// `undefined` is "she has not touched the picker" (#909), which is what
	// falls back to the standing answer below; anything she chooses herself
	// is kept as-is, including after the roster finishes loading. The same
	// Owner also books her colleagues' Visits, so a default that overwrote
	// her own choice would be worse than no default at all.
	let newVisitStaffId = $state<string | undefined>();

	// #486 AC4: the same record-scoped ledger the practice-wide feed reuses,
	// through engagement.ListActivityHandler (unchanged by #486) --
	// ADR-0008's money tier already applies there, per that handler's own
	// doc comment.
	// #2755: the kind the BFF narrows the ledger to; never a filter over the
	// rows already loaded.
	// The kind is in the address (`?activity=`, #2755), so Back and a shared link
	// keep it; the Engagement's own read never takes the query, so a change of
	// kind re-reads only the Activity.
	const kindOfAddress = (): ActivityKind =>
		ACTIVITY_KINDS.find((option) => option.kind !== '' && option.kind === page.url.searchParams.get('activity'))
			?.kind ?? '';
	let activityKind = $state<ActivityKind>(kindOfAddress());
	const activity = new PaginatedList<ActivityEntry>({
		first: { items: [], hasMore: false },
		loadPage: (cursor) => loadEngagementActivityPage(apiFetchWithSession, reference, cursor, activityKind),
		failureMessage: 'We could not load more Activity.'
	});
	const activityLoad = new SectionState<void>(undefined);
	const activityError = $derived(activityLoad.error);

	// #2105: the newest eight entries, as the Overview's Recent activity
	// shows (#2006), because every entry stacked as a record card made
	// this section about 3,000px of an 8,815px page at 390px. "Load more"
	// shows the rest in place: no screen holds one Engagement's whole
	// ledger, and the Practice's Activity screen cannot narrow to one. The
	// first press shows the entries already read, and the next press reads
	// the next page, as it did before.
	// The Overview keeps its eight; the record, which has nine sections to
	// get through, draws five.
	const FIRST_ACTIVITY_ROWS = 5;
	let isWholeLedgerShown = $state(false);
	const shownActivity = $derived(
		isWholeLedgerShown ? activity.items : activity.items.slice(0, FIRST_ACTIVITY_ROWS)
	);
	const hasMoreActivity = $derived(activity.hasMore || shownActivity.length < activity.items.length);

	function showMoreActivity() {
		if (isWholeLedgerShown) {
			activity.loadMore();
			return;
		}
		isWholeLedgerShown = true;
	}

	const portalInvite = new SectionState('');
	const portalInviteLink = $derived(portalInvite.value);
	const portalInviteError = $derived(portalInvite.error);
	const isSendingPortalInvite = $derived(portalInvite.isBusy);

	// #255: once a portal invite send succeeds, this overlays the
	// load-time read the same way statusOverride (#253, below) does --
	// so the Contract section's block lifts and the "Send portal invite"
	// action's own visibility updates without a reload. An invited row
	// whose send is queued ('pending', the outbox row's own default) is
	// exactly what portalinvite.invite() leaves behind.
	let clientPortalOverride = $state<PortalInviteSubject | undefined>();
	const clientPortalState = $derived<PortalInviteSubject>(
		clientPortalOverride ?? {
			portalInviteStatus: detail?.clientPortalInviteStatus,
			portalInviteDelivery: detail?.clientPortalInviteDelivery,
			emailSuppressed: detail?.clientEmailSuppressed
		}
	);
	const hasNeverInvitedClient = $derived(hasNeverBeenInvited(clientPortalState));
	// #2522: an accept refused because her login reaches another record
	// here. Nothing from this record reaches her, so it holds the Contract
	// back as never-invited does. Send portal invite stays: a new invite
	// asks again (InviteHandler clears the refusal).
	const isInviteRefused = $derived(isRefusedInvite(clientPortalState));
	const hasAcceptedPortalAccess = $derived(hasAcceptedPortalInvite(clientPortalState));
	// The question PostSendContractHandler asks (portalinvite's
	// CanReachPortal): an invite that expired, was withdrawn or never
	// arrived holds Send back as never-invited does.
	const canClientReachPortal = $derived(canReachPortal(clientPortalState));
	const isClientErased = $derived(isErased(detail?.clientStanding ? { standing: detail.clientStanding } : undefined));
	const hasClientEmailOnFile = $derived(detail?.clientHasEmail ?? true);
	// InvoiceSection's own standing check (#270) -- defaults true so an
	// in-flight load never flashes the "cannot pay" Notice before
	// `detail` resolves, the same reasoning hasClientEmailOnFile above
	// already uses. Named canClientsPay, word order swapped from the wire
	// field's clientsCanPay, only because unicorn/consistent-boolean-name
	// requires a local boolean binding to start with "can"/"is"/etc; the
	// prop it's passed into below is still clientsCanPay.
	const canClientsPay = $derived(detail?.clientsCanPay ?? true);

	// #2004: a Client reads a Message only through her portal account, so
	// until she has accepted an invite the thread offers no form and says
	// why, and what opens it. Drawing only: the same words the drawing's
	// empty Messages section has.
	const erasedBecause = (consequence: string) => (isClientErased ? `This Client was erased, so ${consequence}.` : undefined);
	const erasedVisitsNote = $derived(erasedBecause('no Visit can be added'));
	const messageBlock = $derived.by(() => {
		if (hasAcceptedPortalAccess) return;
		if (isClientErased) return 'This Client was erased, so no message can be sent.';
		const name = detail.clientName;
		if (!hasClientEmailOnFile) {
			return `${name} has no email address on file, so cannot be invited to the portal to read a message. Add an email address first.`;
		}
		if (isInviteRefused) return `${name} signs in through another record, so cannot read a message sent from this one. ${refusedInviteWayOn}`;
		if (hasNeverInvitedClient) {
			return `${name} reads messages in the portal, so send the portal invite first. The thread opens once ${name} has accepted it.`;
		}
		return `The thread opens once ${name} has accepted the portal invite.`;
	});

	// #2004, #2146: one line for each place the page says a Contract waits
	// on the portal -- the Draft's Send gate and the empty Contract section
	// -- and one for the reason an invite cannot be sent, shared by the
	// header and the empty Contract section, so each says the same thing.
	const inviteBeforeContractMessage =
		'Send a portal invite to this Client before sending the Contract — the Contract can only be viewed and signed once portal access exists.';
	const noEmailForInviteMessage = $derived(
		isClientErased
			? 'This Client was erased, so nobody can be invited to the portal.'
			: 'This Client has no email address on file. Add one before sending a portal invite.'
	);
	const refusedBeforeContractMessage = `This Client signs in through another record, so cannot sign a Contract sent from this one. ${refusedInviteWayOn}`;
	const newInviteBeforeContractMessage =
		'Send this Client a new portal invite before sending the Contract — the last one did not arrive or can no longer be accepted.';
	// #2146: what the empty Contract section says while the Client has no
	// portal account: the same states as messageBlock above. An invite
	// that can still be accepted does not hold Send back, so that state
	// names the acceptance signing waits on rather than asking for an
	// invite again; one that cannot asks for a new one.
	const contractInviteBlock = $derived.by(() => {
		if (hasAcceptedPortalAccess) return;
		if (isClientErased) return noEmailForInviteMessage;
		if (isInviteRefused) return refusedBeforeContractMessage;
		if (hasNeverInvitedClient || !hasClientEmailOnFile) return inviteBeforeContractMessage;
		if (!canClientReachPortal) return newInviteBeforeContractMessage;
		return 'The Contract can only be viewed and signed once this Client has accepted the portal invite.';
	});

	// #1999: the Visits are rows of facts, and a form appears only when
	// asked -- the add form, or one Visit's change, never both, so the
	// section holds at most the one primary its open form submits with
	// (#2019). The same shape as the On call section (#2042).
	// #2231: reassigning is its own act, apart from the change of when and
	// notes, so it is a form of its own.
	type VisitChange = 'add' | { visit: Visit; act: 'change' | 'reassign' };
	let openVisitChange = $state<VisitChange | undefined>();
	const changingVisit = $derived(
		typeof openVisitChange === 'object' && openVisitChange.act === 'change' ? openVisitChange.visit : undefined
	);
	const reassigningVisit = $derived(
		typeof openVisitChange === 'object' && openVisitChange.act === 'reassign' ? openVisitChange.visit : undefined
	);
	// What the last add or change did; it takes focus, because the control
	// she pressed went with the form it was in.
	let visitsSavedMessage = $state('');
	// The open change's drafts, each starting on the Visit's current value
	// when the change opens, so a field she leaves alone sends nothing.
	let changeScheduledAt = $state('');
	let changeNotes = $state('');
	const visitChange = new SectionState<void>(undefined);
	let reassignStaffId = $state('');
	let hasAttemptedReassign = $state(false);
	const visitReassign = new SectionState<void>(undefined);
	// #2103: which opener takes focus back after a Cancel -- 'add' for
	// "Add a Visit", or the id of the Visit whose row Change opened the
	// form. Only a Cancel sets it, so a save's status keeps focus.
	let refocusedVisitOpener = $state('');

	function openVisitForm(change?: VisitChange) {
		openVisitChange = change;
		refocusedVisitOpener = '';
		visitsSavedMessage = '';
		visitChange.error = '';
		visitReassign.error = '';
		reassignStaffId = '';
		hasAttemptedReassign = false;
		if (typeof change === 'object') {
			changeScheduledAt = toDatetimeLocalValue(change.visit.scheduledAt, timeZone);
			changeNotes = change.visit.notes ?? '';
		}
	}

	function toggleVisitChange(visit: Visit) {
		if (changingVisit?.visitId === visit.visitId) openVisitForm();
		else openVisitForm({ visit, act: 'change' });
	}

	// #2231: no Cancel; the row's Reassign closes it, as the Doula row's
	// Change closes its question (#2230).
	function toggleVisitReassign(visit: Visit) {
		if (reassigningVisit?.visitId === visit.visitId) openVisitForm();
		else openVisitForm({ visit, act: 'reassign' });
	}

	function cancelVisitChange(visit: Visit) {
		openVisitForm();
		refocusedVisitOpener = visit.visitId;
	}

	// Coming up, soonest first; not scheduled yet; earlier, newest first (#2755).
	// Read once as the Visits arrive: a Visit that passes while the page is
	// open moves down on the next read.
	const visitsView = $derived.by(() => {
		const now = new Date();
		return { now, rows: sortVisits(visits.items, now) };
	});
	const sortedVisits = $derived(visitsView.rows);
	const visitsNow = $derived(visitsView.now);

	// #2755: the newest earlier Visit is drawn; the rest open in place on
	// request, or when the page lands on one of them.
	let isEarlierVisitsShown = $state(false);
	const shownVisits = $derived.by(() => {
		if (isEarlierVisitsShown) return sortedVisits;
		let earlier = 0;
		return sortedVisits.filter((visit) => {
			if (visitGroup(visit, visitsNow).key !== 'earlier') return true;
			earlier += 1;
			return earlier <= 1;
		});
	});
	const hasEarlierVisits = $derived(
		!isEarlierVisitsShown && (shownVisits.length < sortedVisits.length || visits.hasMore)
	);

	// Brings the row of the last Visit to the top and focuses it, whether or
	// not the Earlier cap had drawn it or its page had loaded; the same
	// landing a `#visit-<id>` link makes on arrival, but repeatable.
	async function showLastVisit(event: MouseEvent) {
		event.preventDefault();
		const target = lastVisit;
		if (!target) return;
		if (shownVisits.every((visit) => visit.visitId !== target.visitId)) isEarlierVisitsShown = true;
		while (visits.hasMore && visits.items.every((visit) => visit.visitId !== target.visitId)) {
			await visits.loadMore();
			if (visits.loadMoreError) break;
		}
		await tick();
		const row = (['table', 'record'] as const)
			.map((view) => document.querySelector<HTMLElement>(`#${CSS.escape(visitRowId(target, view))}`))
			.find((candidate) => candidate !== null && candidate.getClientRects().length > 0);
		row?.scrollIntoView({ block: 'start' });
		row?.focus({ preventScroll: true });
	}

	// Into Notes with the caret at the end, so a Doula writes on from what
	// she wrote; a copy of the form that is not drawn is left alone.
	function focusNotesAtEnd(wrapper: HTMLElement) {
		const field = wrapper.querySelector('textarea');
		if (!field || field.getClientRects().length === 0) return;
		field.focus();
		field.setSelectionRange(field.value.length, field.value.length);
	}

	// #2747: a Schedule row links here with `#visit-<id>`; its row takes
	// focus, once, when the Visits arrive.
	const landOnVisitId = $derived(/^#visit-(.+)$/.exec(page.url.hash)?.[1]);
	const isVisitFormOpen = (visit: Visit) =>
		changingVisit?.visitId === visit.visitId || (canAssignVisits && reassigningVisit?.visitId === visit.visitId);
	const visitRowId = (visit: Visit, view: DataTableView) => `visit-row-${view}-${visit.visitId}`;
	const MAX_ARRIVAL_PAGES = 20;
	let arrivalPagesRead = 0;
	let hasLandedOnVisit = false;
	$effect(() => {
		if (hasLandedOnVisit || !landOnVisitId || !visitsLoad.hasSettled) {
			return;
		}
		const visit = sortedVisits.find((row) => row.visitId === landOnVisitId);
		if (!visit) {
			// A Visit past the first page: read on, a page at a time, until it
			// is there or the Visits run out. The cap stops a link that names
			// a Visit this Engagement does not hold from reading them all.
			if (visits.hasMore && !visits.isLoadingMore && !visits.loadMoreError && arrivalPagesRead < MAX_ARRIVAL_PAGES) {
				arrivalPagesRead += 1;
				void visits.loadMore();
			}
			return;
		}
		hasLandedOnVisit = true;
		const views: DataTableView[] = ['table', 'record'];
		if (!shownVisits.includes(visit)) isEarlierVisitsShown = true;
		void tick().then(() => landOnRow(views.map((view) => visitRowId(visit, view))));
	});

	let messages = $state<Message[]>([]);
	let messagesCursor = $state('');
	let isMessagesHasMore = $state(false);
	// #2755: the newest two are drawn, the rest in place on request.
	const SHOWN_MESSAGES = 2;
	let isEarlierMessagesShown = $state(false);
	const shownMessages = $derived(isEarlierMessagesShown ? messages : messages.slice(-SHOWN_MESSAGES));
	const hasEarlierMessages = $derived(!isEarlierMessagesShown && (messages.length > SHOWN_MESSAGES || isMessagesHasMore));
	// Load (including "load older") and download are separate SectionState
	// instances so downloading an attachment never lights up the "Load
	// older" spinner. Both write the one Notice the thread renders. A
	// send is the compose form's own: MessageThread holds its refusals and
	// its busy state, against the field each names (#1952).
	const messagesLoad = new SectionState<void>(undefined);
	const messagesDownload = new SectionState<void>(undefined);
	// "Show earlier" reads older Messages: its failure stays beside its
	// button and never takes the thread's rows away.
	const messagesOlder = new SectionState<void>(undefined);
	const messagesError = $derived(messagesOlder.error || messagesDownload.error);
	const isLoadingOlderMessages = $derived(messagesOlder.isBusy);
	// Object URLs for image attachments, keyed by messageId, so images
	// render inline in the thread (not just downloadable) -- fetched via
	// apiFetchWithSession since the attachment endpoint requires the
	// caller's session cookie, which a plain <img src> can't send.
	let attachmentPreviewURLs = $state<Record<string, string>>({});
	let unsubscribePushMessages: () => void = () => {};

	type PlanType = 'care_plan' | 'birth_plan';
	// The Engagement view's Care Plan and Birth Plan sections both render
	// off this one list -- see planInstance.ts's doc comment: they're driven
	// by the same generic Plan Instance API, parameterized by plan type.
	const planSections: { type: PlanType; heading: string }[] = [
		{ type: 'care_plan', heading: 'Care Plan' },
		{ type: 'birth_plan', heading: 'Birth Plan' }
	];
	// One SectionState per plan type -- Care Plan and Birth Plan are two
	// sections, not one, so a failure or a busy save in either never
	// touches the other's.
	const planState: Record<PlanType, SectionState<Instance | undefined>> = {
		care_plan: new SectionState<Instance | undefined>(undefined),
		birth_plan: new SectionState<Instance | undefined>(undefined)
	};
	// Distinct from planState[type].isBusy: this gates whether the section
	// renders a Form or a Create button at all, and stays true once the
	// first load has settled, success or failure -- same as isContractLoaded
	// below.
	let planLoaded = $state<Record<PlanType, boolean>>({ care_plan: false, birth_plan: false });
	// A plan that exists and could not be read says so, never "not started".
	let planLoadFailed = $state<Record<PlanType, boolean>>({ care_plan: false, birth_plan: false });

	// Named contractState, not contractSection: that name is already the
	// RecordDetail section snippet below (content: contractSection), and a
	// script binding can't share it.
	const contractState = new SectionState<Contract | undefined>(undefined);
	const contract = $derived(contractState.value);
	const contractError = $derived(contractState.error);
	const isContractBusy = $derived(contractState.isBusy);
	let isContractLoaded = $state(false);

	// #258: block over warn -- the same precondition PostSendContractHandler
	// enforces server-side, checked here so the Send control never offers a
	// click that the server would refuse anyway. `contract.values` carries
	// every merge field value in one map (#969 retired the money/scope
	// split), so a filled money field is never flagged as missing.
	const missingMergeFields = $derived(
		contract ? missingMergeFieldKeys(contract.mergeFields, contract.values) : []
	);

	// #1876: the merge fields ContractForm offers for editing. Save sends
	// only these (editableValues removes price), so a Draft with none of
	// them has nothing to save, and the page does not offer Save.
	const contractEditableFields = $derived(contract ? editableMergeFields(contract.mergeFields) : []);

	// #1885: block over warn -- PostContractHandler refuses a Contract when
	// the Practice has no rate for the Engagement's kind (#967), so "Create
	// Draft Contract" is live only once the rates are known and this kind
	// has one. Read only while there is no Contract (loadContractSection),
	// and from displayKind, so a kind change on this page moves the block
	// with no second read: the response always holds both kinds.
	const ratesState = new SectionState<Rate[] | undefined>(undefined);
	const hasRateForKind = $derived(
		typeof ratesState.value?.find((rate) => rate.kind === displayKind)?.amountCents === 'number'
	);
	const canCreateContract = $derived(hasRateForKind && !isClientErased);
	const isRateMissing = $derived(ratesState.value !== undefined && !hasRateForKind);
	// The settings hub lists Rates only for an Owner or Admin, so only they
	// get the link; anyone else is told who can set the rate.
	const noRateMessage = $derived(
		isPracticeOwnerOrAdmin
			? `Your Practice has no ${displayKind} rate yet, so you cannot create a Contract.`
			: `Your Practice has no ${displayKind} rate yet, so you cannot create a Contract. Ask an owner or admin to set one.`
	);

	// Named invoicesState for the same reason as contractState above.
	const invoicesState = new SectionState<Invoice[]>([]);
	const invoices = $derived(invoicesState.value);
	// #2094: a contractor's rows, dates and status with no money, read
	// from the same route in a shape of their own.
	const contractorInvoicesState = new SectionState<ContractorInvoice[]>([]);
	// #2829: what the Contract has billed, read with the list. A money
	// reader gets the figures; a contractor only whether any is left.
	let contractBilling = $state<ContractBilling | undefined>();
	let hasUnbilledForContractor = $state(true);
	let contractorOpen = $state<ContractorOpenSummary>({ openCount: 0, overdueCount: 0 });
	// What every open Invoice adds up to (#2755), the BFF's sum for a money reader.
	let owed = $state<OwedSummary | undefined>();
	const invoicesError = $derived(invoicesState.error || contractorInvoicesState.error);

	// #271: undefined until this Practice has chosen a billing rail --
	// InvoiceSection itself decides what to render for that, from the
	// same standing fact clientsCanPay/hasClientEmail already are.
	const billingModeState = new SectionState<BillingMode | undefined>(undefined);
	const billingMode = $derived(billingModeState.value);
	// An empty list that has not been read is not "No Invoices yet.", and a
	// billing mode that has not been read is not "Create Invoice".
	const isInvoicesSettled = $derived(
		(canReadContractMoney ? invoicesState.hasSettled : contractorInvoicesState.hasSettled) &&
			billingModeState.hasSettled
	);

	const practiceTimezone = $derived(timeZone);
	const visitsZoneNote = $derived(
		visits.items.some((visit) => visit.scheduledAt) ? zoneNote(timeZone, readerTimeZone(), new Date()) : undefined
	);
	// #2828: the Practice's payment terms, the answer Create Invoice's due
	// question has chosen. 0 until read; real terms are at least a day.
	const paymentTermsState = new SectionState(0);
	const paymentTermsDays = $derived(paymentTermsState.value);

	// Offers on this Engagement (#317). Owner/Admin only at the BFF, so a
	// Doula's load simply fails and the section stays hidden -- the read
	// table keeps who-was-asked away from her, and an error banner about
	// it would only be noise on her own screen. loadEngagementOffersOrNone
	// (engagementDetail.ts) already turns that refusal into `undefined`
	// rather than a throw, so offersState.error is never rendered here on
	// purpose -- the section's rule is silence, not a Notice. Named
	// offersState for the same reason as contractState above.
	const offersState = new SectionState<unknown[] | undefined>(undefined);
	const offers = $derived((offersState.value as Offer[] | undefined) ?? []);

	// #2271: each contractor's fee and whether it was paid. Owner and Admin
	// only: loadDoulaFees answers undefined for anyone the BFF refuses, and
	// the section is then left out, as Offers is. Drawn only when some
	// Doula on the Engagement has a fee, since there is nothing else to say.
	// #2492: the Engagement read says so up front, so the section stands
	// from the first frame and its heading never joins the page late.
	const doulaFeesState = new SectionState<DoulaFee[] | undefined>(undefined);
	const isDoulaFeesVisible = $derived(
		data.hasDoulaFees || (doulaFeesState.value?.length ?? 0) > 0 || doulaFeesState.error !== ''
	);

	// The Practice's Doulas, read once for the whole page (#268): the
	// Offers section offers them, and so do the two Visit pickers. One
	// read, so the two sections can never disagree about who is on the
	// roster, and one place that decides what a refusal means.
	//
	// Read through the Engagement, not through the Practice-wide roster
	// (#911). Whether a Doula may be *named on a Visit here* is an
	// Engagement-scoped fact -- a contractor needs the attachment her own
	// acceptance of an Offer opened -- and the roster read could not know
	// it, so both pickers offered names the BFF then refused with a 400.
	// The BFF answers it now, from the same code its own write enforces
	// with (api/internal/visit/nameable.go). The Offers section takes the
	// same rows: it wants the identity half, which has not changed.
	//
	// `undefined` is "this reader may not be offered a colleague at all"
	// -- the read is Owner/Admin (ADR-0008), and loadVisitAssigneesOrNone
	// answers a refusal with an absence rather than a throw. Every
	// assign-shaped control on this page is drawn off that absence, which
	// is #274: the screen must not offer a Visit write the reader's own
	// role will refuse. It is drawing only, never the gate -- the BFF
	// refuses the same call whether or not the control was rendered
	// (api/internal/visit/roles.go).
	const rosterState = new SectionState<VisitAssignee[] | undefined>(undefined);
	const doulas = $derived(rosterState.value);
	const canAssignVisits = $derived(doulas !== undefined);
	// What the "(cannot be named yet)" marker in those labels means, said
	// once above the field rather than left for a reader to discover by
	// choosing somebody. The empty string when nobody is marked.
	const assigneeHint = $derived(unnameableHint(doulas ?? []));
	// A Doula logging her own Visit needs no roster and no picker: an
	// absent assignee means "me". An Owner or Admin who is not a Doula has
	// no self to log, so for her the picker is the only way in.
	const canLogOwnVisit = $derived(isDoula(data.session));
	const canAddVisit = $derived((canAssignVisits || canLogOwnVisit) && !isClientErased);

	// #2755: an address ending in `#add-visit` opens the form the way
	// `#visit-<id>` opens a Visit. The hash is what is tracked, so a Cancel
	// does not open the form again.
	const isAddVisitAsked = $derived(page.url.hash === '#add-visit');
	$effect(() => {
		if (isAddVisitAsked && canAddVisit) {
			untrack(() => {
				if (openVisitChange !== 'add') openVisitForm('add');
			});
		}
	});

	// Raised by the header's "Message the Client": each rise opens the compose
	// in Messages and puts focus in the Message field.
	let composeRequest = $state(0);

	function handleMessageTheClient(event: MouseEvent) {
		if (messageBlock || !messagesLoad.hasSettled || messagesLoad.error) return;
		event.preventDefault();
		composeRequest += 1;
	}

	// #909: which roster entry is the reader herself. Off the practice
	// session `practices/[practiceId]/+layout.ts` already resolved before
	// first paint -- not a second fetch of `/api/staff/session`, which
	// lands after it.
	const callerStaffId = $derived(data.session.staffId);
	// The standing answer to "Who is this Visit for?" for a reader who is
	// herself on the roster: a Doula-Owner is being asked a question the
	// service can already answer -- `assignee` in api/internal/visit reads
	// an absent assignee as the caller, which is exactly why a plain Doula
	// is never asked at all. Derived from the loaded roster rather than
	// from her roles, so the value is always an option the picker really
	// offers: an Owner or Admin who holds no Doula role is not on this
	// list, gets `''`, and must answer for herself.
	const defaultVisitStaffId = $derived(
		doulas?.some((doula) => doula.staffId === callerStaffId) ? callerStaffId : ''
	);
	const visitStaffId = $derived(newVisitStaffId ?? defaultVisitStaffId);
	// One derivation, both pickers (#911), taking #909's per-caller axis on
	// the way through: two independently built lists on one page is the
	// failure this closes, so neither picker can come to disagree with the
	// other about who is on offer or about how she is drawn. The reassign
	// picker adds its own per-Visit exclusion at the call site.
	const createAssigneeOptions = $derived(visitAssigneeOptions(doulas ?? [], { callerStaffId }));
	// The hard block, derived rather than raised on submit: the refusal
	// and its remedy are on screen the moment she picks the name, and
	// handleCreateVisit below returns without sending anything. Prevented
	// here, still enforced at the BFF (api/internal/visit/roles.go).
	//
	// #1228: an empty pick used to fall to the Select's own `required` --
	// StackedForm's `novalidate` (ADR-0021) takes that away, so an empty
	// visitStaffId is refused here too, but only once a submit has been
	// attempted (hasAttemptedCreateVisit): there is no live indicator for
	// "nothing chosen yet" the way #911's ineligible-pick block already is
	// one. Guarded on canAssignVisits, because visitStaffId also reads ''
	// for a plain Doula logging her own Visit, who is never shown this
	// picker at all and so has nothing to refuse.
	let hasAttemptedCreateVisit = $state(false);
	const newVisitBlock = $derived(
		canAssignVisits && !visitStaffId
			? (hasAttemptedCreateVisit ? 'Select who this Visit is for' : '')
			: assigneeBlock(doulas ?? [], visitStaffId)
	);
	// The Offers section turns on the Offers read alone, not on the roster.
	// The two happen to be the same pair of roles today (both Owner and
	// Admin), so tying Offers to `canAssignVisits` looked free -- but it
	// makes an unrelated roster outage take the Offers section down with
	// it, and it re-decides who may see Offers in a place that is not
	// about Offers. Each section answers for its own read.
	const isOffersVisible = $derived(isPracticeOwnerOrAdmin || offersState.value !== undefined);
	// #1598: nobody offers herself work, so the Offers form does not list
	// the person at the screen. The filter is on what the form is handed
	// and not on the roster: the Visit pickers read the same roster and
	// must keep her ("(you)", #909). `?.` keeps `undefined` as it is --
	// a roster that could not be read is not a roster with nobody on it
	// (#1432). Drawing only: the BFF refuses an Offer whose target is its
	// sender (offer.resolveStaffTarget).
	// #2230: nor a Doula already on it -- the work is hers already.
	const offerDoulas = $derived(
		doulas?.filter(
			(doula) =>
				doula.staffId !== callerStaffId && attachedDoulas.every((attached) => attached.staffId !== doula.staffId)
		)
	);

	// #2230: an Owner or an Admin puts a Doula on from the same Change, one
	// question and a save. GLOSSARY.md's Attachment entry: "An Admin may
	// attach an employee directly". Who is offered is the BFF's
	// visit-assignees answer (the rule its own write reads), less who is on
	// it already, so a contractor who holds neither role is not offered:
	// she goes on by an Offer. Only where somebody besides the reader could
	// be chosen is it a question; otherwise the reader keeps the one press
	// (#1598), since a question with one answer is not a question.
	const attachOptions = $derived(
		doulas === undefined || displayStatus === 'completed'
			? []
			: attachableDoulaOptions(doulas, {
					attachedStaffIds: attachedDoulas.map((doula) => doula.staffId),
					callerStaffId
				})
	);
	const isAttachQuestion = $derived(attachOptions.some((option) => option.value !== callerStaffId));
	const canChangeDoula = $derived(canAttachSelf || isAttachQuestion);
	// Said under the question only where the roster holds somebody it
	// leaves out for this reason, so the line is never about nobody.
	const hasUnlistedContractor = $derived(doulas?.some((doula) => !doula.nameable) ?? false);
	const ATTACH_GROUP = 'attach-doula';
	const attachWords = $derived(attachDoulaWords(attachedDoulas));
	// #2569: past DOULA_RADIOS_MAX the question is a select, as on Start work.
	const isAttachSelect = $derived(isDoulaSelect(attachOptions));
	function attachFieldId(): string {
		return isAttachSelect ? ATTACH_GROUP : radioFieldId(ATTACH_GROUP, attachOptions[0]!.value);
	}
	let attachChoice = $state<string | undefined>();
	const attachSubmission = new FormSubmission();

	async function handleAttachSubmit(event: SubmitEvent) {
		event.preventDefault();
		selfAttachedMessage = '';
		let isAttached = false;
		await attachSubmission.run(async () => {
			const chosen = attachOptions.find((option) => option.value === attachChoice);
			if (!chosen) {
				return [{ message: 'Select who the Doula is', targetId: attachFieldId() }];
			}
			whoIsOnOverride = await attachDoula(apiFetchWithSession, reference, chosen.value);
			openChange = undefined;
			attachChoice = undefined;
			selfAttachedMessage =
				chosen.value === callerStaffId
					? 'You are on this work.'
					: `${doulas!.find((doula) => doula.staffId === chosen.value)!.name} is on this work.`;
			isAttached = true;
		}, orThrownMessage);
		if (!isAttached) return;
		await loadOnCall();
		await loadActivity();
	}

	// #2596: taking a Doula off is its own control under the summary, apart
	// from the Doula row's add, and ends in a confirm dialog that names the
	// act. Offered only to an Owner or an Admin where somebody is on it; the
	// endpoint refuses anyone else whatever is drawn.
	const canTakeOff = $derived(isPracticeOwnerOrAdmin && attachedDoulas.length > 0 && displayStatus !== 'completed');
	const TAKE_OFF_GROUP = 'take-off-doula';
	let isTakeOffShown = $state(false);
	let isTakeOffConfirmShown = $state(false);
	let takeOffChoice = $state<string | undefined>();
	let takeOffError = $state('');
	const takeOffSubmission = new FormSubmission();
	const takeOffOptions = $derived(attachedDoulas.map((doula) => ({ value: doula.staffId, label: doula.name })));
	const takeOffDoulaChosen = $derived(
		attachedDoulas.find((doula) => doula.staffId === (takeOffChoice ?? (attachedDoulas.length === 1 ? attachedDoulas[0]!.staffId : undefined)))
	);
	const takeOffDialog = $derived(
		takeOffDoulaChosen ? takeOffWords(takeOffDoulaChosen, takeOffDoulaChosen.staffId === callerStaffId) : undefined
	);

	function toggleTakeOff() {
		isTakeOffShown = !isTakeOffShown;
		takeOffChoice = undefined;
		takeOffError = '';
		takeOffSubmission.errors = [];
	}

	async function handleTakeOffSubmit(event: SubmitEvent) {
		event.preventDefault();
		await takeOffSubmission.run(async () => {
			if (!takeOffDoulaChosen) {
				return [{ message: 'Select who comes off', targetId: radioFieldId(TAKE_OFF_GROUP, takeOffOptions[0]!.value) }];
			}
			takeOffError = '';
			isTakeOffConfirmShown = true;
		}, orThrownMessage);
	}

	async function confirmTakeOff() {
		const doula = takeOffDoulaChosen!;
		try {
			whoIsOnOverride = await takeOffDoula(apiFetchWithSession, reference, doula.staffId);
		} catch (error) {
			takeOffError = error instanceof Error ? error.message : 'Could not take off the Doula';
			throw error;
		}
		isTakeOffShown = false;
		takeOffChoice = undefined;
		openChange = undefined;
		selfAttachedMessage = takenOffMessage(doula, doula.staffId === callerStaffId);
		await Promise.all([loadVisits(), loadOnCall(), loadActivity()]);
	}

	onDestroy(() => {
		for (const url of Object.values(attachmentPreviewURLs)) {
			URL.revokeObjectURL(url);
		}
		unsubscribePushMessages();
	});

	// The module fetches and returns the new object URLs; this owns
	// merging and revoking them, because only the component knows when its
	// own teardown has come.
	async function refreshAttachmentPreviews(items: Message[]) {
		Object.assign(
			attachmentPreviewURLs,
			await loadAttachmentPreviews(apiFetchWithSession, reference, items, attachmentPreviewURLs)
		);
	}

	// The Client detail hub (#494). `detail.clientId` comes straight off
	// the Engagement's own read (engagement.Detail), so no extra fetch is
	// needed to build the link.
	function clientDetailHref(): string {
		return `/practices/${page.params.practiceId}/clients/${detail!.clientId}`;
	}

	/*
	 * The summary card (#1998), in the drawing's order: Care, Due, Care
	 * phase, Doula. The Client's name is the page's h1, so it is not a row.
	 * Two rows the drawing does not have stay after the drawn four, with no
	 * Change: "Portal invite" (#255), the Client's invite state as standing
	 * information in the same word the Clients list column carries, since
	 * the header's Send control is withheld once she has accepted; and
	 * "Created", the "how did this come to be" every feature answers.
	 *
	 * Due shows the date when there is one. Its Change (#2013) opens the
	 * date form, and the row reads the overlaid date once it is saved.
	 * With no date, a birth Engagement with no birth outcome yet shows the
	 * row as "Not given", the Request pages' word for the same gap, so the
	 * same Change can give it a first date and On-call cover (#2118).
	 * Every other Engagement with no date leaves the row out (#538): a
	 * postpartum one has none (ADR-0017); after a loss or an unknown
	 * outcome nothing may presume an expected baby (ADR-0015 standing rule
	 * 1); and after a live birth the pregnancy has ended, the On-call
	 * window ends on the day it did, and a first date would drive nothing
	 * (canChangeKind's own reason).
	 *
	 * A row carries `changes` -- and so a Change -- only where this reader
	 * may change it, from the same flags that drew the old buttons:
	 * canChangeKind, canWriteFact for the due date, the BFF's statusMoves,
	 * and the BFF's canAttachSelf. A contractor Doula holds none of the
	 * four, so her summary has no Change at all, and the endpoints refuse
	 * her whatever is drawn.
	 */
	const summaryRows = $derived.by((): EngagementSummaryRow[] => {
		const rows: EngagementSummaryRow[] = [
			// #874: what the Practice sold, in the Staff-only word ADR-0015
			// gives it.
			{ label: 'Care', value: kindLabel(displayKind), ...changeable('care', canChangeKind) }
		];
		// Her email, from the read of her record this page already makes (her
		// phone is the facts' sixth); left out where that read was refused or
		// her record is erased.
		if (clientRecord && !clientRecord.erasedAt && clientRecord.email) {
			rows.push({ label: 'Email', value: clientRecord.email, contact: 'mailto' });
		}
		const homeAddress = clientRecord && !clientRecord.erasedAt ? formatAddress(clientRecord) : '';
		if (homeAddress) rows.push({ label: 'Home address', value: homeAddress });
		rows.push(
			{
				label: 'Portal invite',
				value: portalInviteSummaryText(clientPortalState, hasClientEmailOnFile),
				...(!hasAcceptedPortalAccess && hasClientEmailOnFile && { isInvite: true, changes: 'portal invite' })
			},
			// createdAt is an instant (engagement/detail.go).
			{ label: 'Created', value: formatInstant(detail.createdAt, timeZone) }
		);
		return rows;
	});

	/*
	 * #2755: the due date, the care phase and the Doulas are the header's own
	 * words, so Details holds no row for them, only the act that changes each,
	 * for the reader the BFF lets act (the same flags that drew the old rows'
	 * Change). With no date the act gives the first one (#2118); with no Doula
	 * it puts one on.
	 */
	const detailActs = $derived.by((): { change: SummaryChange; label: string }[] => {
		const acts: { change: SummaryChange; label: string }[] = [];
		if (canWriteFact && (displayDueDate || (displayKind === 'birth' && birthOutcome.birthOutcome === undefined))) {
			acts.push({ change: 'due', label: displayDueDate ? 'Change the due date' : 'Give the due date' });
		}
		if (displayStatusMoves.length > 0) acts.push({ change: 'carePhase', label: 'Change the care phase' });
		if (canChangeDoula) {
			acts.push({
				change: 'doula',
				label: attachedDoulas.length > 0 ? 'Put on or take off a Doula' : (isAttachQuestion ? 'Put a Doula on' : 'Put yourself on')
			});
		}
		return acts;
	});

	// The form that is open, if its act is still the reader's to take: a Doula
	// who is put on while it is open takes the "Put me on" form's reason with it.
	const openAct = $derived(detailActs.find((act) => act.change === openChange)?.change);

	interface FactRow extends SummaryListRow {
		/**
		A muted second line under the value: who the Next Visit is with.
		*/
		detail?: string;
		href?: string;
		hiddenWords?: string;
		linkName?: string;
		onClick?: (event: MouseEvent) => void;
		/**
		A state someone has to act on (warning) or that is late (error): an icon beside the words.
		*/
		tone?: 'warning' | 'error';
	}

	/*
	 * #2755: the facts a reader wants on arriving, each from the read that
	 * owns it. A read that failed says so, with no link, never a state it did
	 * not read; one still on its way says nothing it does not know. Each
	 * derives from the state its section writes after a save, so a payment, a
	 * Visit or a Message re-reads its fact in place.
	 */
	const isOnCallEnded = $derived(
		displayStatus === 'completed' || birthOutcome.birthOutcome !== undefined || detail.kind === 'postpartum'
	);
	const todayInZone = $derived(dayIn(new Date(), timeZone));

	function factLink(label: string, section: string, value: string, tone?: FactRow['tone']): FactRow {
		return { label, value, href: `#${section.toLowerCase().replaceAll(' ', '-')}`, hiddenWords: `${label}, in ${section}`, tone };
	}

	const onCallFact = $derived.by((): FactRow | undefined => {
		if (isOnCallEnded) return undefined;
		const tonight = onCall ? onCall.tonight : detail.tonight;
		if (!tonight && onCallError) return { label: 'On call tonight', value: 'Could not be read' };
		if (tonight?.isNobody) return factLink('On call tonight', 'On call', 'Nobody');
		if (tonight) {
			const who = tonight.holders.map((holder) =>
				holder.coveringFor ? `${holder.name}, covering for ${holder.coveringFor}` : holder.name
			);
			return factLink('On call tonight', 'On call', who.join('; '));
		}
		if (detail.doulas.length === 0) return factLink('On call tonight', 'On call', 'No Doula yet');
		const from = detail.onCallWindow?.from;
		return factLink(
			'On call tonight',
			'On call',
			from && todayInZone < from ? `Starts ${formatCalendarDayThisYear(from, timeZone)}` : 'No on-call days'
		);
	});

	// The On call section's first line (#2755): who is on call tonight, from
	// the on-call read; nothing before the window or after an outcome.
	const tonightLine = $derived.by((): { text: string; isNobody: boolean } | undefined => {
		if (isOnCallEnded || !onCall?.tonight) return undefined;
		if (onCall.tonight.isNobody) {
			// Whose gap it is, and until when, so the line answers "why" too.
			const out = onCall.gaps.filter(
				(gap) =>
					!gap.coveringStaffName &&
					gapNow < new Date(gap.endsAt) &&
					dayIn(new Date(gap.startsAt), timeZone) <= todayInZone
			);
			const names = [...new Set(out.map((gap) => gap.staffName))];
			if (names.length === 0) return { text: 'Nobody on call tonight.', isNobody: true };
			const who = names.length === 1 ? names[0] : `${names.slice(0, -1).join(', ')} and ${names.at(-1)}`;
			const until =
				names.length === 1 && out.length === 1 ? ` until ${formatScheduledVisit(out[0]!.endsAt, timeZone)}` : '';
			return { text: `Nobody on call tonight: ${who} cannot be reached${until}.`, isNobody: true };
		}
		const names = onCall.tonight.holders.map((holder) => holder.name);
		return { text: `${names.join(' and ')} ${names.length === 1 ? 'is' : 'are'} on call tonight.`, isNobody: false };
	});

	const nextVisitFact = $derived.by((): FactRow => {
		if (visitsLoad.error) return { label: 'Next Visit', value: 'Could not be read' };
		if (!visitsLoad.hasSettled) {
			return detail.hasVisits
				? { label: 'Next Visit', value: 'Reading the Visits' }
				: factLink('Next Visit', 'Visits', 'None scheduled');
		}
		if (!nextVisit?.scheduledAt) return factLink('Next Visit', 'Visits', 'None scheduled');
		return {
			...factLink('Next Visit', 'Visits', formatScheduledVisit(nextVisit.scheduledAt, timeZone)),
			detail: nextVisit.staffName
		};
	});

	// Absent for a reader the BFF serves no money (ADR-0008).
	const owedFact = $derived.by((): FactRow | undefined => {
		if (!canReadContractMoney) return undefined;
		if (invoicesState.error) return { label: 'Owed', value: 'Could not be read' };
		if (!invoicesState.hasSettled) {
			return detail.hasInvoices ? { label: 'Owed', value: 'Reading the Invoices' } : factLink('Owed', 'Invoices', 'Nothing owed');
		}
		if (!owed) return { label: 'Owed', value: 'Could not be read' };
		if (owed.overdueCents > 0) {
			const since = owed.oldestOverdueOn ? ` since ${formatCalendarDayThisYear(owed.oldestOverdueOn, timeZone)}` : '';
			return factLink('Owed', 'Invoices', `${formatAmount(owed.overdueCents)} overdue${since}`, 'error');
		}
		if (owed.owedCents > 0) {
			const due = owed.nextDueOn ? `, due ${formatCalendarDayThisYear(owed.nextDueOn, timeZone)}` : '';
			return factLink('Owed', 'Invoices', `${formatAmount(owed.owedCents)}${due}`);
		}
		return factLink('Owed', 'Invoices', 'Nothing owed');
	});

	// The Invoices section's first line (#2755): what is owed, in the BFF's
	// sum; a reader served no money is told how many Invoices are open and
	// when the next is due, never an amount (ADR-0008).
	const invoiceStateLine = $derived.by((): { text: string; isOverdue: boolean } | undefined => {
		if (invoicesError || !isInvoicesSettled) return undefined;
		if (canReadContractMoney) {
			if (!owed) return undefined;
			if (owed.overdueCents > 0) {
				const since = owed.oldestOverdueOn ? ` since ${formatCalendarDayThisYear(owed.oldestOverdueOn, timeZone)}` : '';
				return { text: `${formatAmount(owed.overdueCents)} overdue${since}.`, isOverdue: true };
			}
			if (owed.owedCents > 0) {
				const due = owed.nextDueOn ? `, due ${formatCalendarDayThisYear(owed.nextDueOn, timeZone)}` : '';
				return { text: `${formatAmount(owed.owedCents)} owed${due}.`, isOverdue: false };
			}
			return { text: 'Nothing owed.', isOverdue: false };
		}
		const { openCount, overdueCount, nextDueOn } = contractorOpen;
		if (openCount === 0) return { text: 'No Invoice is open.', isOverdue: false };
		const noun = openCount === 1 ? 'Invoice' : 'Invoices';
		const late = overdueCount > 0 ? `, ${overdueCount} overdue` : '';
		const next = nextDueOn ? `, next due ${formatCalendarDayThisYear(nextDueOn, timeZone)}` : '';
		return { text: `${openCount} ${noun} open${late}${next}.`, isOverdue: overdueCount > 0 };
	});

	const contractFact = $derived.by((): FactRow => {
		if (contractState.error) return { label: 'Contract', value: 'Could not be read' };
		if (contract) return factLink('Contract', 'Contract', contractStatusText(contract.status));
		if (detail.hasContract && !contractState.hasSettled) return { label: 'Contract', value: 'Reading the Contract' };
		return factLink('Contract', 'Contract', 'No Contract yet');
	});

	const messagesFact = $derived.by((): FactRow => {
		if (messagesLoad.error) return { label: 'Messages', value: 'Could not be read' };
		if (!messagesLoad.hasSettled) return { label: 'Messages', value: 'Reading the Messages' };
		if (messages.length === 0) return factLink('Messages', 'Messages', 'No Messages yet');
		const since = unansweredSince(messages);
		if (since) {
			return factLink('Messages', 'Messages', `Unanswered since ${formatActivityTimestamp(since, timeZone)}`, 'warning');
		}
		return factLink('Messages', 'Messages', 'Answered');
	});

	// The sixth fact, after Messages (the two ways to reach her together):
	// the number calls; none on file is "Not given", a link to Details' add;
	// a read that was refused says so (T10's words) with no link. The row is
	// always drawn, so nothing moves when her record lands.
	const phoneFact = $derived.by((): FactRow => {
		if (!isClientRecordSettled) return { label: 'Phone', value: 'Reading the Client' };
		if (!clientRecord) return { label: 'Phone', value: 'Could not be read' };
		if (clientRecord.erasedAt) return { label: 'Phone', value: 'Not given' };
		if (clientRecord.phone) { // contact:ignore -- tested for presence, drawn by formatPhoneNumber below
			return {
				label: 'Phone',
				value: formatPhoneNumber(clientRecord.phone),
				href: `tel:${clientRecord.phone.replaceAll(/[^\d+]/g, '')}`,
				linkName: `Call ${detail.clientDisplayName}, ${formatPhoneNumber(clientRecord.phone)}`
			};
		}
		return {
			label: 'Phone',
			value: 'Not given',
			href: '#details',
			hiddenWords: 'Phone, in Details',
			onClick: focusAddDetails
		};
	});

	async function focusAddDetails(event: MouseEvent) {
		event.preventDefault();
		const region = document.querySelector<HTMLElement>('#details');
		region?.scrollIntoView();
		await tick();
		region?.querySelector<HTMLElement>(':scope [data-add-details] a')?.focus();
	}

	const factRows = $derived(
		[onCallFact, nextVisitFact, contractFact, owedFact, messagesFact, phoneFact].filter(
			(row): row is FactRow => row !== undefined
		)
	);

	/** The fields that give a row its Change, and open its in-place form
	 * while it is the open one. The visually-hidden words are the row's own
	 * label, so a screen reader hears "Change, care phase". */
	function changeable(change: SummaryChange, canChange: boolean): Partial<EngagementSummaryRow> {
		if (!canChange) return {};
		return { change, changes: changeWords[change], isDetailShown: openChange === change };
	}

	/** portalInviteStatusText's own word, plus the one qualifier that word
	 * alone can't carry: a Client with no email address on file cannot be
	 * invited at all (#255), which "Never invited" alone would read as
	 * merely "not invited yet". */
	function portalInviteSummaryText(portalState: PortalInviteSubject, hasClientEmail: boolean): string {
		if (isClientErased) return 'Client erased, so nobody can be invited';
		const text = portalInviteStatusText(portalState);
		if (isRefusedInvite(portalState)) return `${text}. ${refusedInviteWayOn}`;
		if (!hasClientEmail && !portalState.portalInviteStatus) {
			return `${text} — no email on file, so the Client cannot be invited yet`;
		}
		return text;
	}

	// #895: the BFF moves an Engagement from 'intake' to 'active' by
	// itself, in the same transaction as the Visit write, the first time a
	// Visit on it is left scheduled (ADR-0015). Neither Visit endpoint
	// answers with the Engagement's new status, so without this the hub
	// would keep reading "Getting started" -- and keep offering the manual
	// "active" move -- until somebody reloaded the page.
	//
	// It re-reads only when this write actually left an instant behind and
	// only while the page still believes the Engagement is at 'intake':
	// that is the one status the automatic move can change, so every other
	// page view pays nothing for it. A failed re-read is swallowed on
	// purpose -- the Visit write itself succeeded and is already reported,
	// and a stale label is not worth a second error message on top of it.
	// Nothing breaks if it is stale, either: the manual "Mark care as
	// active" button it leaves on screen re-requests a status the
	// Engagement already holds, which TransitionHandler treats as a no-op
	// and answers with the true status and moves -- so the page corrects
	// itself on the next click rather than meeting a refusal.
	async function refreshStatusAfterScheduling(scheduledAt: string | undefined) {
		if (!scheduledAt || displayStatus !== 'intake') return;
		try {
			const refreshed = await loadEngagement(apiFetchWithSession, reference);
			statusOverride = { status: refreshed.status, statusMoves: refreshed.statusMoves };
		} catch {
			// Left as it was; a reload reads the true status.
		}
	}

	async function loadVisits() {
		await visitsLoad.load(async () => {
			const first = await loadVisitsPage(apiFetchWithSession, reference, '');
			nextVisit = first.next;
			lastVisit = first.last;
			visits.reset(first);
		}, 'We could not load the Visits.');
	}

	// A kind chosen quickly twice sends two reads; only the latest one's
	// reply is drawn.
	let activityRead = 0;
	async function loadActivity() {
		const read = ++activityRead;
		const kind = activityKind;
		await activityLoad.load(async () => {
			const loaded = await loadEngagementActivityPage(apiFetchWithSession, reference, '', kind);
			if (read === activityRead) activity.reset(loaded);
		}, 'We could not load the Activity.');
	}

	$effect(() => {
		const kind = kindOfAddress();
		untrack(() => {
			if (kind === activityKind) return;
			activityKind = kind;
			isWholeLedgerShown = false;
			void loadActivity();
		});
	});

	function activityKindHref(kind: ActivityKind): string {
		return `${page.url.pathname}${kind === '' ? '' : `?activity=${kind}`}#activity`;
	}

	async function loadMessages() {
		await messagesLoad.load(async () => {
			const loaded = await loadMessagesPage<Message>(apiFetchWithSession, reference, '');
			messages = loaded.items;
			messagesCursor = loaded.nextCursor ?? '';
			isMessagesHasMore = loaded.hasMore;
			await refreshAttachmentPreviews(messages);
		}, 'We could not load the Messages.');
	}

	/** #301's Staff visibility (AC4), as the Birth Plan's read view says it
	 * (#2004): whether the Client has read it since it last changed, by
	 * her name. A Care Plan instance never carries clientAcknowledgedAt,
	 * so only the Birth Plan draws this row. */
	function birthPlanReadText(instance: Instance): string {
		if (!birthPlanOffer.offersBirthPlan) return `Not shown to ${detail.clientName}`;
		return instance.clientAcknowledgedAt
			? `${detail.clientName} read it on ${formatInstant(instance.clientAcknowledgedAt, timeZone)}`
			: `${detail.clientName} has not read it yet`;
	}

	// The Engagement read says whether each record exists (#2591), so a
	// Plan or Contract not started is never asked for, which would log a 404.
	async function loadPlan(planType: PlanType) {
		const isStarted = planType === 'care_plan' ? detail.hasCarePlan : detail.hasBirthPlan;
		if (isStarted) {
			await planState[planType].load(
				() => loadInstance(apiFetchWithSession, page.params.practiceId!, page.params.engagementId!, planType),
				'We could not load the plan.'
			);
		}
		planLoaded[planType] = true;
		planLoadFailed[planType] = isStarted && planState[planType].error !== '';
	}

	/*
	 * #2004: a Plan is facts first -- how far it is answered and, for the
	 * Birth Plan, whether the Client has read it -- and its questions
	 * appear only when asked, the record-page pattern the Visits and On
	 * call sections already follow. The open form edits a draft, so Cancel
	 * leaves the Plan as it was; a save returns to the facts with a status
	 * that takes focus.
	 */
	let shownPlanForms = $state<Record<PlanType, boolean>>({ care_plan: false, birth_plan: false });
	let planDraft = $state<Record<PlanType, Answers>>({ care_plan: {}, birth_plan: {} });
	let planSavedMessage = $state<Record<PlanType, string>>({ care_plan: '', birth_plan: '' });
	// #2103: the Plan whose Cancel hands focus back to its "Change the …"
	// opener, drawn again once the form has gone.
	let refocusedPlanOpener = $state<PlanType | undefined>();

	function openPlanForm(planType: PlanType) {
		refocusedPlanOpener = undefined;
		planDraft[planType] = { ...planState[planType].value?.answers };
		planSavedMessage[planType] = '';
		planState[planType].error = '';
		shownPlanForms[planType] = true;
	}

	function cancelPlanForm(planType: PlanType) {
		planState[planType].error = '';
		shownPlanForms[planType] = false;
		refocusedPlanOpener = planType;
	}

	// A Plan's one line (#2755): how far it is answered, its audit trail
	// (#2104, #2133: the last save once there is one, which also says the
	// Plan was started; until then, the start) and, for the Birth Plan,
	// whether the Client has read it.
	function planStateLine(instance: Instance, planType: PlanType): string {
		const parts = [planProgress(instance.fields, instance.answers)];
		if (instance.lastEdited) parts.push(`last changed ${stampText(instance.lastEdited, timeZone)}`);
		else if (instance.started) parts.push(`started ${stampText(instance.started, timeZone)}`);
		if (planType === 'birth_plan') parts.push(birthPlanReadText(instance));
		else parts.push(`${detail.clientName} never sees the Care Plan`);
		return parts.join(' · ');
	}

	// The answers are capped in height where the plan is read on the record.
	// Whether the cap hides any is measured, so a short plan has neither a
	// fade nor a "Show the whole" and a long one has both.
	let expandedPlans = $state<Record<PlanType, boolean>>({ care_plan: false, birth_plan: false });
	let clippedPlans = $state<Record<PlanType, boolean>>({ care_plan: false, birth_plan: false });

	function watchPlanClip(planType: PlanType) {
		return (element: HTMLElement) => {
			const observer = new ResizeObserver(() => {
				if (!expandedPlans[planType]) clippedPlans[planType] = element.scrollHeight > element.clientHeight + 1;
			});
			observer.observe(element);
			return () => observer.disconnect();
		};
	}

	function hasPlanAnswers(instance: Instance): boolean {
		return Object.keys(instance.answers).length > 0;
	}

	// A new Plan has nothing to read yet, so starting one opens its
	// questions at once.
	async function handleStartPlan(planType: PlanType) {
		const isStarted = await planState[planType].mutate(
			() => createInstance(apiFetchWithSession, page.params.practiceId!, page.params.engagementId!, planType),
			'We could not start the plan.'
		);
		if (!isStarted) return;
		openPlanForm(planType);
		await loadActivity();
	}

	function handlePlanAnswerChange(planType: PlanType, fieldId: string, value: unknown) {
		planDraft[planType] = setAnswer(planDraft[planType], fieldId, value);
	}

	function handlePlanToggleOption(planType: PlanType, fieldId: string, option: string) {
		planDraft[planType] = toggleMultiSelectOption(planDraft[planType], fieldId, option);
	}

	// The saved Plan is the BFF's answer, so a Staff edit's cleared
	// Birth Plan acknowledgment reads back at once.
	async function handleSavePlan(event: SubmitEvent, planType: PlanType, heading: string) {
		event.preventDefault();
		const isSaved = await planState[planType].mutate(
			() =>
				saveAnswers(
					apiFetchWithSession,
					page.params.practiceId!,
					page.params.engagementId!,
					planType,
					planDraft[planType]
				),
			'We could not save the plan.'
		);
		if (!isSaved) return;
		shownPlanForms[planType] = false;
		planSavedMessage[planType] = `${heading} saved.`;
		await loadActivity();
	}

	async function loadContractSection() {
		if (detail.hasContract) {
			await contractState.load(
				() => loadContract(apiFetchWithSession, page.params.practiceId!, page.params.engagementId!),
				'We could not load the Contract.'
			);
		}
		isContractLoaded = true;
		await loadRatesWhenStartable();
	}

	// The rates decide whether a Contract can be created, so they are read
	// only while one can be: with no Contract, or with a voided one (#2260).
	// Once, however many times the Contract is voided on this page.
	async function loadRatesWhenStartable() {
		const isStartable = contractState.value === undefined || contractState.value.status === 'voided';
		if (!isStartable || ratesState.value !== undefined) return;
		await ratesState.load(
			async () => {
				const { rates } = await loadRates(apiFetchWithSession, page.params.practiceId!);
				return rates;
			},
			'We could not load the rates.'
		);
	}

	/*
	 * #2004: the Contract is facts first -- its status and fee as rows, its
	 * text behind a disclosure -- and a Draft's merge fields appear only
	 * when asked, as a form that edits a draft, so Cancel leaves the
	 * Contract as it was. A save or a send returns to the facts with a
	 * status that takes focus.
	 */
	let isContractFormOpen = $state(false);
	let contractDraft = $state<Record<string, string>>({});
	let contractSavedMessage = $state('');
	// #2103: Cancel hands focus back to "Change the Draft Contract", drawn
	// again once the form has gone. Only Cancel sets it.
	let isContractOpenerRefocused = $state(false);

	function openContractForm() {
		isContractOpenerRefocused = false;
		contractDraft = { ...contractState.value?.values };
		contractSavedMessage = '';
		contractState.error = '';
		isContractFormOpen = true;
	}

	function cancelContractForm() {
		contractState.error = '';
		isContractFormOpen = false;
		isContractOpenerRefocused = true;
	}

	// A new Draft's merge fields are what is left to do, so creating one
	// opens them at once -- where it has any to fill.
	async function handleCreateContract() {
		const isCreated = await contractState.mutate(
			() => createContract(apiFetchWithSession, page.params.practiceId!, page.params.engagementId!),
			'We could not create the Contract.'
		);
		if (!isCreated) return;
		if (contractEditableFields.length > 0) openContractForm();
		// "Contract drafted" is on the ledger now, under who did it.
		await loadActivity();
	}

	function handleContractValueChange(key: string, value: string) {
		contractDraft = setMergeFieldValue(contractDraft, key, value);
	}

	async function handleSaveContract(event: SubmitEvent) {
		event.preventDefault();
		const isSaved = await contractState.mutate(
			() =>
				saveContractValues(
					apiFetchWithSession,
					page.params.practiceId!,
					page.params.engagementId!,
					editableValues(contractDraft)
				),
			'We could not save the Contract.'
		);
		if (!isSaved) return;
		isContractFormOpen = false;
		contractSavedMessage = 'Contract saved.';
		await loadActivity();
	}

	async function handleSendContract() {
		contractSavedMessage = '';
		const isSent = await contractState.mutate(
			() => sendContract(apiFetchWithSession, page.params.practiceId!, page.params.engagementId!),
			'We could not send the Contract.'
		);
		if (!isSent) return;
		contractSavedMessage = 'Contract sent.';
		await loadActivity();
	}

	// ContractStatus.svelte shows a refusal beside its button, so this one
	// throws it instead of writing contractState.error, as Void does.
	async function handleRemindContract() {
		contractSavedMessage = '';
		const reminded = await remindContract(apiFetchWithSession, page.params.practiceId!, page.params.engagementId!);
		contractState.value = reminded;
		contractSavedMessage = 'Reminder on its way.';
		await loadActivity();
	}

	/* The fee as this reader may see it (ADR-0008): the Practice's price
	   where the BFF sends it, or her own agreed fee (#2003). */
	const contractFeeRows = $derived<SummaryListRow[]>([
		...(contract?.values.price ? [{ label: 'Fee', value: contract.values.price }] : []),
		...(data.yourFee ? yourFeeRows(data.yourFee) : [])
	]);

	// ContractStatus.svelte owns the error display for Void (it awaits
	// the onVoid callback prop itself and shows whatever it throws in its
	// confirmation) -- unlike the other Contract handlers above, this one
	// deliberately doesn't touch contractState.error.
	async function handleVoidContract() {
		if (!contractState.value) return;
		contractState.value = await voidContract(
			apiFetchWithSession,
			page.params.practiceId!,
			page.params.engagementId!
		);
		// The ledger has a new entry, and a voided Contract can be followed
		// by the next one (#2260), whose Create waits on the rates.
		await loadRatesWhenStartable();
		await loadActivity();
	}

	// #971: a Doula's own path to a void, offered exactly where
	// handleVoidContract is not (see onRequestVoid below). Same shape as
	// handleVoidContract above -- ContractStatus.svelte awaits this
	// itself and shows whatever it throws.
	async function handleRequestVoidContract(reason: string) {
		if (!contractState.value) return;
		contractState.value = await requestContractVoid(
			apiFetchWithSession,
			page.params.practiceId!,
			page.params.engagementId!,
			reason
		);
		void loadActivity();
	}

	// The Owner/Admin side of #971's ask -- same shape as
	// handleVoidContract above.
	async function handleDeclineVoidRequest(requestId: string, reason: string) {
		if (!contractState.value) return;
		contractState.value = await declineContractVoidRequest(
			apiFetchWithSession,
			page.params.practiceId!,
			page.params.engagementId!,
			requestId,
			reason
		);
		void loadActivity();
	}

	// Same shape as handleVoidContract above -- ContractStatus.svelte
	// awaits this itself and shows whatever it throws (#302). Until #305
	// lands this 500s in local/CI, which is exactly what that display is
	// for.
	async function handleDownloadSignedContractPdf() {
		const blob = await downloadSignedContractPdf(
			apiFetchWithSession,
			page.params.practiceId!,
			page.params.engagementId!
		);
		triggerBlobDownload(blob, 'signed-contract.pdf');
	}

	async function loadInvoicesSection() {
		// #2094: a contractor reads each Invoice's dates and status and never
		// its money (ADR-0008, amended 2026-10-26), so the BFF answers her
		// with rows of their own; everyone else reads the list with money.
		if (canReadContractMoney) {
			await invoicesState.load(async () => {
				const read = await loadInvoices(apiFetchWithSession, page.params.practiceId!, page.params.engagementId!);
				contractBilling = read.billing;
				owed = read.owed;
				return read.invoices;
			}, 'We could not load the Invoices.');
		} else {
			await contractorInvoicesState.load(async () => {
				const read = await loadContractorInvoices(
					apiFetchWithSession,
					page.params.practiceId!,
					page.params.engagementId!
				);
				hasUnbilledForContractor = read.hasUnbilled;
				contractorOpen = read.open;
				return read.invoices;
			}, 'We could not load the Invoices.');
		}
		// #271: read alongside the Invoice list itself -- InvoiceSection
		// needs to know the current mode (or its absence) before it can
		// decide whether to show the "ask once" form.
		await billingModeState.load(
			() => loadBillingMode(apiFetchWithSession, page.params.practiceId!),
			'We could not load how this Practice bills.'
		);
	}

	// A new Invoice splices into the list, but what is owed is the BFF's sum
	// over every open Invoice, so the fact reads it again; a read that fails
	// says so rather than keep a figure the Invoice has made wrong.
	async function refreshOwed() {
		try {
			const read = await loadInvoices(apiFetchWithSession, page.params.practiceId!, page.params.engagementId!);
			owed = read.owed;
		} catch {
			owed = undefined;
		}
	}

	async function loadPaymentTermsSection() {
		await paymentTermsState.load(async () => {
			const terms = await loadPaymentTerms(apiFetchWithSession, page.params.practiceId!);
			return terms.netDays;
		}, 'We could not load the payment terms.');
	}

	// Reported by InvoiceSection's onCreate prop. No catch here, same as
	// the original: a refused create is left to the component the same way
	// Void Contract is (see above). #270 removed the old connectRequired
	// gate this used to route on -- InvoiceSection now decides whether to
	// show the form at all from canClientsPay, a standing fact read up
	// front, so a create attempt reaching this function is always the
	// happy path or a genuine error. #2829: amountCents is what the form
	// asked, and absent for a contractor, whose Invoice bills what is left.
	async function handleCreateInvoice(chosenBillingMode?: BillingMode, due?: InvoiceDueChoice, amountCents?: number) {
		const invoice = await createInvoice(
			apiFetchWithSession,
			page.params.practiceId!,
			page.params.engagementId!,
			chosenBillingMode,
			due,
			amountCents
		);
		// #2002: a contractor's 201 names no amount (RaisedInvoice), so her
		// new row is read back in her own shape (#2094) rather than spliced.
		billingModeState.value = invoice.billingMode;
		void loadActivity();
		if (canReadContractMoney) {
			invoicesState.value = [invoice, ...invoicesState.value];
			void refreshOwed();
if (contractBilling) {
				contractBilling = billingAfterRaising(contractBilling, invoice.amountCents);
			}
		} else {
			await loadInvoicesSection();
		}
	}

	// #271: reported by InvoiceSection's onRecordPayment/onVoidInvoice/
	// onWriteOffInvoice props. Each reloads the Invoice list on success --
	// unlike handleCreateInvoice's own in-place splice, a status/paid_at
	// flip is easier to re-fetch than to reconstruct from the Payment or
	// transition response alone.
	async function handleRecordPayment(
		invoiceId: string,
		input: { method: PaymentMethod; note?: string; paidOn: string }
	) {
		await recordPayment(apiFetchWithSession, page.params.practiceId!, invoiceId, input);
		void loadActivity();
		await loadInvoicesSection();
	}

	async function handleReversePayment(invoiceId: string, paymentId: string, reason: string) {
		await reversePayment(apiFetchWithSession, page.params.practiceId!, invoiceId, paymentId, reason);
		void loadActivity();
		await loadInvoicesSection();
	}

	// #1009: reloads the section afterwards, as every other money write
	// here does -- the Invoice's refundedCents is the BFF's to sum.
	async function handleRefundPayment(invoiceId: string, paymentId: string, input: RefundPaymentInput) {
		await refundPayment(apiFetchWithSession, page.params.practiceId!, invoiceId, paymentId, input);
		void loadActivity();
		await loadInvoicesSection();
	}

	async function handleVoidInvoice(invoiceId: string) {
		await voidInvoice(apiFetchWithSession, page.params.practiceId!, invoiceId);
		void loadActivity();
		await loadInvoicesSection();
	}

	async function handleWriteOffInvoice(invoiceId: string) {
		await writeOffInvoice(apiFetchWithSession, page.params.practiceId!, invoiceId);
		void loadActivity();
		await loadInvoicesSection();
	}

	// The roster, Offers and Doula fees reads are all Owner/Admin, and are
	// asked for only by one (#2601): a Doula is never sent to be refused,
	// so the section is left out rather than shown broken. The module
	// still answers undefined for a refusal, so these never throw in
	// practice, and offersState.error is never rendered (see its own
	// declaration above).
	async function loadOffersSection() {
		if (!isPracticeOwnerOrAdmin) return;
		await offersState.load(() => loadOffers(apiFetchWithSession, reference, loadEngagementOffers), '');
	}

	async function loadDoulaFeesSection() {
		if (!isPracticeOwnerOrAdmin) return;
		await doulaFeesState.load(
			() => loadDoulaFees(apiFetchWithSession, reference.practiceId, reference.engagementId),
			'We could not load the Doula fees.'
		);
	}

	async function handleRecordDoulaFee(attachmentId: string, input: RecordPaymentInput) {
		await recordDoulaFeePaid(apiFetchWithSession, reference.practiceId, reference.engagementId, attachmentId, input);
		void loadActivity();
		await loadDoulaFeesSection();
	}

	async function handleRemoveDoulaFee(attachmentId: string, paymentId: string, reason: string) {
		await removeDoulaFeePaid(
			apiFetchWithSession,
			reference.practiceId,
			reference.engagementId,
			attachmentId,
			paymentId,
			reason
		);
		void loadActivity();
		await loadDoulaFeesSection();
	}

	// Silent about a *refusal*, and only about a refusal: a roster read
	// this reader's role does not admit her to is her role, not a failure,
	// and loadVisitAssigneesOrNone answers it with `undefined` rather than
	// a throw. Anything else -- a 500, a dropped connection -- it
	// rethrows, and rosterState.error carries it to the Notice above the
	// Visits table, because the alternative is an Owner watching every
	// assign-shaped control on this page vanish with no reason given.
	/*
	 * Who is on call for this birth (#1093), and the two things a person
	 * changes from here: one Doula's own days, and a coverage gap.
	 *
	 * Inline rather than an organism of its own, for the reason
	 * ADR-0018's tier rule gives: this screen is its only consumer today.
	 * The moment a second surface needs it -- the roster growing an
	 * inline editor is the obvious candidate -- it becomes a component,
	 * with its own style-guide page.
	 */
	let onCall = $state<EngagementOnCall | undefined>();
	let onCallError = $state('');
	// The panel's own read once it has landed, so a saved gap moves the
	// header; the Engagement's before it. Nothing after a loss or completion.
	const isNobodyTonight = $derived(
		displayStatus !== 'completed' &&
			birthOutcome.birthOutcome === undefined &&
			(onCall ? onCall.tonight : detail.tonight)?.isNobody === true
	);

	// #2755: the sections' order, settled from the Engagement read alone so
	// nothing moves when a later read lands; "Jump to" follows it.
	const isOnCallLineOnly = $derived(
		displayStatus === 'completed' || birthOutcome.birthOutcome !== undefined || detail.kind === 'postpartum'
	);
	const onCallEndedLine = $derived.by((): string => {
		if (detail.kind === 'postpartum') return 'Postpartum care has no on-call days.';
		return detail.pregnancyEndedOn
			? `On call ended on ${formatCalendarDay(detail.pregnancyEndedOn)}.`
			: 'On call has ended.';
	});
	const orderedHeadings = $derived(
		sectionOrder({
			status: detail.status,
			kind: detail.kind,
			birthOutcome: detail.birthOutcome,
			today: todayInZone,
			dueDate: detail.dueDate,
			onCallWindow: detail.onCallWindow,
			hasDoula: detail.doulas.length > 0,
			hasVisits: detail.hasVisits,
			hasMessages: detail.hasMessages,
			hasInvoices: detail.hasInvoices,
			hasOffers: detail.hasOffers,
			hasContract: detail.hasContract,
			hasCarePlan: detail.hasCarePlan,
			hasBirthPlan: detail.hasBirthPlan,
			isOffersVisible,
			isDoulaFeesVisible,
			hasStartNote: Boolean(detail.startNote)
		})
	);
	const sectionContent = {
		'On call': () => (isOnCallLineOnly ? onCallEndedSection : onCallSection),
		'Birth outcome': () => birthOutcomeSection,
		Contract: () => contractSection,
		Visits: () => visitsSection,
		'Care Plan': () => carePlanSection,
		'Birth Plan': () => birthPlanSection,
		Invoices: () => invoicesSection,
		Offers: () => offersSection,
		'Doula fees': () => doulaFeesSection,
		'Note when work started': () => startNoteSection,
		Messages: () => messagesSection,
		Activity: () => activitySection
	} satisfies Record<SectionHeading, () => Snippet>;
	const orderedSections = $derived(
		orderedHeadings.map((heading) => ({ heading, content: sectionContent[heading]() }))
	);
	// Nothing is chosen when the form opens (#1916, ADR-0055): which Doula
	// cannot be reached is a fact about someone else, and every Owner and
	// Admin is told of the gap, so a report sent without reading the
	// picker would name the wrong Doula to the whole Practice.
	let gapStaffId = $state('');
	// Each question's id, keyed by the GapRequest field the BFF names in a
	// refusal's `details`, in the order the questions are on the page, so
	// the error summary lists the page's own check and the BFF's in field
	// order (GOV.UK's error summary) -- Go writes a map's keys sorted.
	const gapFieldIds = {
		staffId: 'coverage-gap-staff',
		startsAt: 'coverage-gap-starts',
		endsAt: 'coverage-gap-ends',
		coveringStaffId: 'coverage-gap-cover',
		reason: 'coverage-gap-reason'
	};
	const gapFieldOrder: string[] = Object.values(gapFieldIds);
	// Every refusal of the gap form, from the page's own check or the BFF:
	// the summary lists it, and each question reads its own entry from it.
	const gapSubmission = new FormSubmission();
	let gapStartsAt = $state('');
	let gapEndsAt = $state('');
	let gapReason = $state('');
	let gapCoveringStaffId = $state('');
	let gapError = $state('');
	let narrowFrom = $state<Record<string, string>>({});
	let narrowTo = $state<Record<string, string>>({});
	// #2042: the section's facts are rows, and a change opens only when
	// asked -- one Doula's days under her row, or the gap form under the
	// list. One of them at a time, so the section never holds more than
	// the one primary its open form submits with (#2019).
	type OnCallChange = 'gap' | { staffId: string };
	let openOnCallChange = $state<OnCallChange | undefined>();
	let onCallDaysError = $state('');
	// What the last change did; it takes focus, because the control she
	// pressed went with the form it was in.
	let onCallSavedMessage = $state('');

	function isDaysChangeOpen(staffId: string): boolean {
		return typeof openOnCallChange === 'object' && openOnCallChange.staffId === staffId;
	}

	// #2103: Cancel on the gap form hands focus back to its opener, drawn
	// again once the form has gone. Only Cancel sets it: a recorded gap's
	// status takes focus instead.
	let isGapOpenerRefocused = $state(false);
	let gapBeingChangedId = $state<string | undefined>();

	function openOnCall(change?: OnCallChange) {
		openOnCallChange = change;
		isGapOpenerRefocused = false;
		if (change !== 'gap') clearGapFields();
		onCallDaysError = '';
		onCallSavedMessage = '';
		gapError = '';
	}

	function toggleDaysChange(staffId: string) {
		if (isDaysChangeOpen(staffId)) openOnCall();
		else openOnCall({ staffId });
	}

	function clearGapFields() {
		gapBeingChangedId = undefined;
		gapStaffId = '';
		gapStartsAt = '';
		gapEndsAt = '';
		gapReason = '';
		gapCoveringStaffId = '';
		gapSubmission.errors = [];
	}

	function changeGap(gap: CoverageGap) {
		openOnCall('gap');
		gapBeingChangedId = gap.id;
		gapStaffId = gap.staffId;
		gapStartsAt = toDatetimeLocalValue(gap.startsAt, timeZone);
		gapEndsAt = toDatetimeLocalValue(gap.endsAt, timeZone);
		gapReason = gap.reason ?? '';
		gapCoveringStaffId = gap.coveringStaffId ?? '';
	}

	type OnCallDoulaRow = SummaryListRow & { staffId: string; from?: string; to?: string };
	type GapRow = SummaryListRow & { gap: CoverageGap };

	async function loadOnCall() {
		try {
			const panel = await loadEngagementOnCall(
				apiFetchWithSession,
				reference.practiceId,
				reference.engagementId
			);
			onCall = panel;
			narrowFrom = Object.fromEntries(panel.doulas.map((d) => [d.staffId, d.from ?? '']));
			narrowTo = Object.fromEntries(panel.doulas.map((d) => [d.staffId, d.to ?? '']));
		} catch (error) {
			onCallError = error instanceof Error ? error.message : 'We could not load who is on call.';
		}
	}

	/* An instant the BFF takes, from what `<input type="datetime-local">`
	   holds -- which is a wall-clock time with no zone on it, so it is read
	   in the Practice's zone rather than the reader's or UTC. An empty
	   control (or a half-typed one, which reports '') is `undefined`, never
	   `new Date('')`, whose `toISOString()` throws "Invalid time value"
	   (#1969). */
	function toInstant(local: string): Date | undefined {
		const instant = instantFromWall(local, timeZone);
		return local === '' || Number.isNaN(instant.getTime()) ? undefined : instant;
	}

	/* The page's own check, in the BFF's words (`oncall.MsgStaffIDNeeded`,
	   `MsgStartsAtRequired`, `MsgEndsAtRequired`, `MsgEndsBeforeStarts`),
	   in field order. */
	function coverageGapErrors(startsAt: Date | undefined, endsAt: Date | undefined): FormError[] {
		const errors: FormError[] = [];
		if (gapStaffId === '') {
			errors.push({ message: 'Select the Doula who cannot be reached', targetId: gapFieldIds.staffId });
		}
		if (!startsAt) errors.push({ message: 'Enter when the gap starts', targetId: gapFieldIds.startsAt });
		if (!endsAt) errors.push({ message: 'Enter when the gap ends', targetId: gapFieldIds.endsAt });
		if (startsAt && endsAt && endsAt <= startsAt) {
			errors.push({ message: 'The gap must end after it starts', targetId: gapFieldIds.endsAt });
		}
		return errors;
	}

	/* A refusal in the order the questions are on the page; one that names
	   no field has no target and goes first, as it is about the whole gap. */
	function inFieldOrder(errors: FormError[]): FormError[] {
		const place = (entry: FormError) => (entry.targetId ? gapFieldOrder.indexOf(entry.targetId) : -1);
		return errors.toSorted((a, b) => place(a) - place(b));
	}

	async function submitCoverageGap(event: SubmitEvent) {
		event.preventDefault();
		await gapSubmission.run(
			async () => {
				const startsAt = toInstant(gapStartsAt);
				const endsAt = toInstant(gapEndsAt);
				const errors = coverageGapErrors(startsAt, endsAt);
				if (!startsAt || !endsAt || errors.length > 0) return errors;

				await saveGap(
					apiFetchWithSession,
					reference.practiceId,
					reference.engagementId,
					{
						staffId: gapStaffId,
						startsAt: startsAt.toISOString(),
						endsAt: endsAt.toISOString(),
						reason: gapReason === '' ? undefined : gapReason,
						coveringStaffId:
							gapCoveringStaffId === '' || gapCoveringStaffId === gapStaffId ? undefined : gapCoveringStaffId
					},
					gapBeingChangedId
				);
				const name = onCall?.doulas.find((doula) => doula.staffId === gapStaffId)?.name ?? 'the Doula';
				const verb = gapBeingChangedId ? 'changed' : 'recorded';
				closeGapForm();
				void loadActivity();
				await loadOnCall();
				onCallSavedMessage = `The time ${name} cannot be reached is ${verb}.`;
			},
			async (refusal) => inFieldOrder(await orThrownErrors(gapFieldIds)(refusal))
		);
	}

	/* Closes the gap form blank, so it opens with nobody chosen next time
	   (#1916), and drops whatever it last refused. */
	function closeGapForm() {
		openOnCall();
	}

	function cancelGapForm() {
		closeGapForm();
		isGapOpenerRefocused = true;
	}

	async function clearCoverageGap(gap: CoverageGap) {
		openOnCall();
		try {
			await clearGap(apiFetchWithSession, reference.practiceId, reference.engagementId, gap.id);
			void loadActivity();
			await loadOnCall();
			onCallSavedMessage = `The gap for ${gap.staffName} is cleared.`;
		} catch (error) {
			gapError = error instanceof Error ? error.message : 'We could not clear the coverage gap.';
		}
	}

	async function saveOnCallDays(event: SubmitEvent, doula: { staffId: string; name: string }) {
		event.preventDefault();
		onCallDaysError = '';
		try {
			await saveNarrowing(apiFetchWithSession, reference.practiceId, reference.engagementId, doula.staffId, {
				from: narrowFrom[doula.staffId] === '' ? undefined : narrowFrom[doula.staffId],
				to: narrowTo[doula.staffId] === '' ? undefined : narrowTo[doula.staffId]
			});
			openOnCall();
			void loadActivity();
			await loadOnCall();
			onCallSavedMessage = `The on-call days for ${doula.name} are saved.`;
		} catch (error) {
			onCallDaysError = error instanceof Error ? error.message : 'We could not save the on-call days.';
		}
	}

	/* One row per Doula on the birth. A row carries `changes` -- and so a
	   Change -- only for an Owner or an Admin, and only while there is a
	   window to narrow; the endpoint refuses anyone else whatever is drawn.
	   With no window at all -- after a loss, say (#2220) -- the row names no
	   window that never was. */
	const sayDay = (day: string) => formatCalendarWeekdayDay(day, timeZone, gapNow);

	let isDoulaChangeFocused = $state(false);
	const putDoulaOnWords = $derived.by(() => {
		if (!canChangeDoula || attachedDoulas.length > 0) return '';
		return isAttachQuestion ? 'Put a Doula on this birth' : 'Put yourself on this birth';
	});

	async function focusDoulaChange(event: MouseEvent) {
		event.preventDefault();
		isDoulaChangeFocused = false;
		await tick();
		isDoulaChangeFocused = true;
	}

	const onCallDoulaRows = $derived(
		(onCall?.doulas ?? []).map(
			(doula): OnCallDoulaRow => ({
				staffId: doula.staffId,
				label: doula.name,
				from: doula.onCallFrom,
				to: doula.onCallTo,
				value:
					doula.onCallFrom && doula.onCallTo
						? `On call ${sayDay(doula.onCallFrom)} to ${sayDay(doula.onCallTo)}`
						: (onCall?.window
							? 'Not on call for any of this window'
							: 'Not on call'),
				...(isPracticeOwnerOrAdmin &&
					onCall?.window && {
						changes: `on-call days for ${doula.name}`,
						isDetailShown: isDaysChangeOpen(doula.staffId)
					})
			})
		)
	);

	/* The BFF's own rule (`mayWriteFor`): an Owner or an Admin writes any
	   gap, a Doula only the one that is about her. */
	function canWriteGap(gap: CoverageGap): boolean {
		return isPracticeOwnerOrAdmin || gap.staffId === callerStaffId;
	}

	const gapNow = new Date();
	const coverHint =
		'Leave this unanswered where nobody is covering yet. Every Owner and Admin is told, to arrange cover. Only a Doula on this birth can cover.';

	const gapRows = $derived(
		(onCall?.gaps ?? []).map(
			(gap): GapRow => ({
				gap,
				label: gap.staffName,
				value: formatGapTimes(gap.startsAt, gap.endsAt, timeZone, gapNow),
				...(canWriteGap(gap) && { changes: `the gap for ${gap.staffName}` })
			})
		)
	);

	const gapStaffOptions = $derived(
		(onCall?.doulas ?? [])
			.filter((doula) => isPracticeOwnerOrAdmin || doula.staffId === callerStaffId)
			.map((doula) => ({ value: doula.staffId, label: doula.name }))
	);
	const gapCoveringOptions = $derived(
		(onCall?.doulas ?? [])
			.filter((doula) => doula.staffId !== gapStaffId)
			.map((doula) => ({ value: doula.staffId, label: doula.name }))
	);
	const hasUncoveredGap = $derived((onCall?.gaps ?? []).some((gap) => !gap.coveringStaffName));

	async function loadRoster() {
		if (!isPracticeOwnerOrAdmin) return;
		await rosterState.load(
			() => loadVisitAssigneesOrNone(apiFetchWithSession, reference.practiceId, reference.engagementId),
			'We could not load the Practice roster, so there is nobody to pick from. Try again.'
		);
	}

	// The same block the create picker carries (#911): the reassign picker
	// offers the same people from the same list, so it refuses the same
	// choice with the same words.
	const reassignBlock = $derived(
		reassignStaffId
			? assigneeBlock(doulas ?? [], reassignStaffId)
			: (hasAttemptedReassign ? 'Select who takes this Visit instead' : '')
	);

	function reassignOptionsFor(visit: Visit) {
		return visitAssigneeOptions(doulas ?? [], { callerStaffId, currentAssigneeStaffId: visit.staffId });
	}

	async function handleCreateOffer(offer: NewOffer) {
		await createOffer(apiFetchWithSession, page.params.practiceId!, page.params.engagementId!, offer);
		void loadActivity();
		const updated = await loadEngagementOffers(apiFetchWithSession, page.params.practiceId!, page.params.engagementId!);
		if (offersState.value) offersState.value = updated;
	}

	async function handleWithdrawOffer(offerId: string) {
		await withdrawOffer(apiFetchWithSession, page.params.practiceId!, offerId);
		void loadActivity();
		const updated = await loadEngagementOffers(apiFetchWithSession, page.params.practiceId!, page.params.engagementId!);
		if (offersState.value) offersState.value = updated;
	}

	/*
	 * #1610: her record, read only to word the link to her details
	 * journey ("Add" or "Change") and to leave it off an erased record.
	 * Best-effort, and not awaited by the sections below: a lost read
	 * costs one link for one page load, and her record is one click away
	 * through "View Client".
	 */
	let clientRecord = $state<ClientDetail | undefined>();
	// Her phone is a fact, so a read that was lost is said there (#2755).
	let isClientRecordSettled = $state(false);

	async function loadClientRecord() {
		try {
			clientRecord = await loadClientDetail(apiFetchWithSession, page.params.practiceId!, detail.clientId);
		} catch {
			// The link stays off; the Phone fact says it could not be read.
		}
		isClientRecordSettled = true;
	}

	async function loadRosterThenOffers() {
		await loadRoster();
		await loadOffersSection();
	}

	onMount(async () => {
		void loadClientRecord();
		// The Engagement is already here, from load. The sections behind it
		// read at once and each renders as it lands (#2258); one after
		// another, a phone waited up to 0.8 s for the last. Only the Offers
		// wait, on the roster their form offers the work to.
		await Promise.all([
			loadVisits(),
			loadMessages(),
			...planSections.map((section) => loadPlan(section.type)),
			loadContractSection(),
			loadInvoicesSection(),
			loadPaymentTermsSection(),
			loadRosterThenOffers(),
			loadOnCall(),
			loadDoulaFeesSection(),
			loadActivity()
		]);

		if (isLandingOnContract) {
			await tick();
			landOnSection('contract');
			const earlierId = /^#(contract-.+)$/.exec(page.url.hash)?.[1];
			if (earlierId) document.querySelector<HTMLElement>(`#${CSS.escape(earlierId)}`)?.scrollIntoView({ block: 'start' });
		}

		// #61: an open service worker push message ("a new Message arrived
		// on this Engagement") triggers a refetch, the same content-free
		// "push wakes the client, which fetches the real content" delivery
		// ADR-0002 describes -- see push.ts's PUSH_MESSAGE_TYPE doc comment
		// for why the service worker can't just fetch this itself.
		unsubscribePushMessages = subscribeToThreadPushMessages(page.params.engagementId!, () => {
			void loadMessages();
		});
	});

	async function handleSendPortalInvite() {
		const isSent = await portalInvite.mutate(async () => {
			const created = await sendPortalInvite(apiFetchWithSession, reference);
			// #255: a fresh invite is always pending, never suppressed --
			// lifts the Contract section's block and hides this action's
			// own visibility question ("never invited") right away.
			clientPortalOverride = { portalInviteStatus: 'invited', portalInviteDelivery: 'pending', emailSuppressed: false };
			return `${location.origin}/portal/accept-invite?token=${created.inviteToken}`;
		}, 'We could not send the portal invite.');
		if (isSent) await loadActivity();
	}

	function staffName(staffId: string): string {
		return doulas?.find((doula) => doula.staffId === staffId)?.name ?? 'the Doula';
	}

	/* The smallest step of #1216's scale that holds a picker's longest
	   option, which is the count `Select` asks for (a narrower box cuts the
	   chosen name off). Past the scale's top it is the top step, never the
	   whole column (#1999): a step counts `ch`, the width of a digit, and
	   a name in a proportional face is narrower than that, so the 37
	   characters of "Trish Halvorsen (cannot be named yet)" still show
	   whole in the 30 step, as the walk of 2026-10-26 saw. */
	function widthToHold(options: readonly { label: string }[]): InputWidth {
		const longest = Math.max(0, ...options.map((option) => option.label.length));
		return INPUT_WIDTHS.find((width) => width >= longest) ?? 30;
	}

	/* Closes the add form blank. Back to untouched, not to empty: the next
	   Visit she logs starts on the same standing answer this one did. */
	function closeAddVisit() {
		newVisitScheduledAt = '';
		newVisitStaffId = undefined;
		hasAttemptedCreateVisit = false;
		visitsCreate.error = '';
		openVisitForm();
	}

	function cancelAddVisit() {
		closeAddVisit();
		// The opener is the Visits section's own button, beside its state line
		// or, with no Visit, in the empty line.
		refocusedVisitOpener = 'add';
	}

	// #268: an empty picker means "for me", which is what a Doula logging
	// her own Visit sends -- the same absent assignee this route sent
	// before the field existed.
	async function handleCreateVisit(event: SubmitEvent) {
		event.preventDefault();
		hasAttemptedCreateVisit = true;
		// #911: the request is never sent for somebody this Engagement
		// cannot admit. The refusal and its remedy are already on screen,
		// beside the field, from the moment she was picked.
		if (newVisitBlock) return;
		const scheduledAt = newVisitScheduledAt ? instantFromWall(newVisitScheduledAt, timeZone).toISOString() : undefined;
		const staffId = visitStaffId || undefined;
		let emailed: string[] = [];
		if (
			await visitsCreate.mutate(async () => {
				emailed = await createVisit(apiFetchWithSession, reference, scheduledAt, staffId);
			}, 'We could not add the Visit.')
		) {
			closeAddVisit();
			await loadVisits();
			// #2540: the same words a move uses when the Doula it names is emailed.
			const told = staffId && emailed.includes(staffId) ? ` We emailed ${staffName(staffId)} to say so.` : '';
			visitsSavedMessage =
				staffId && staffId !== callerStaffId
					? `The Visit for ${staffName(staffId)} is added.${told}`
					: 'Your Visit is added.';
			await refreshStatusAfterScheduling(scheduledAt);
			await refreshWhoIsOn();
			await loadOnCall();
			await loadActivity();
		}
	}

	/*
	 * #1999: one Visit's change, sent as the writes its fields call for --
	 * a field she left alone sends nothing. The two writes are separate
	 * endpoints (schedule #250, notes #251), so they go one after the
	 * other, and a refusal stops the rest: what was already written stays
	 * written, the rows are read again so they say so, and the form stays
	 * open on what is left to send.
	 */
	async function handleChangeVisit(event: SubmitEvent, visit: Visit) {
		event.preventDefault();
		const scheduledAt = changeScheduledAt ? instantFromWall(changeScheduledAt, timeZone).toISOString() : undefined;
		const isRescheduled = changeScheduledAt !== toDatetimeLocalValue(visit.scheduledAt, timeZone);
		const hasNewNotes = changeNotes !== (visit.notes ?? '');
		let hasWritten = false;
		const wasSaved = await visitChange.mutate(async () => {
			if (isRescheduled) {
				// An empty control clears the schedule: `undefined` is
				// scheduleVisit's own way of saying so (engagementDetail.ts).
				await scheduleVisit(apiFetchWithSession, reference, visit.visitId, scheduledAt);
				hasWritten = true;
			}
			if (hasNewNotes) {
				await saveVisitNotes(apiFetchWithSession, reference, visit.visitId, changeNotes);
				hasWritten = true;
			}
		}, 'We could not save the Visit.');
		if (wasSaved) openVisitForm();
		if (!hasWritten) return;
		await loadVisits();
		if (wasSaved) visitsSavedMessage = `The Visit for ${visit.staffName} is saved.`;
		if (isRescheduled) await refreshStatusAfterScheduling(scheduledAt);
		await loadActivity();
	}

	async function handleReassignVisit(event: SubmitEvent, visit: Visit) {
		event.preventDefault();
		hasAttemptedReassign = true;
		const newStaffId = reassignStaffId;
		if (!newStaffId || reassignBlock) return;
		let emailed: string[] = [];
		const wasSaved = await visitReassign.mutate(async () => {
			emailed = await reassignVisit(apiFetchWithSession, reference, visit.visitId, newStaffId);
		}, 'We could not reassign the Visit.');
		if (!wasSaved) return;
		openVisitForm();
		await loadVisits();
		const which = visit.scheduledAt
			? `The Visit on ${formatScheduledVisit(visit.scheduledAt, timeZone)}`
			: 'The Visit that is not yet scheduled';
		// #2436: who was emailed, never that anyone knows: an email can be
		// sent and still not be read.
		const emailedNames = emailed.map((id) => (id === newStaffId ? staffName(newStaffId) : visit.staffName));
		const told = emailedNames.length > 0 ? ` We emailed ${emailedNames.join(' and ')} to say so.` : '';
		visitsSavedMessage = `${which} is ${staffName(newStaffId)}'s now, not ${visit.staffName}'s.${told}`;
		await refreshWhoIsOn();
		await loadOnCall();
		await loadActivity();
	}

	// Reuses loadMessagesPage rather than re-fetching by hand: the query
	// string and the newest-first-to-oldest-first reversal are exactly the
	// same read, just prepended instead of replacing.
	async function handleLoadOlderMessages() {
		await messagesOlder.mutate(async () => {
			const older = await loadMessagesPage<Message>(apiFetchWithSession, reference, messagesCursor);
			messages = [...older.items, ...messages];
			messagesCursor = older.nextCursor ?? '';
			isMessagesHasMore = older.hasMore;
			await refreshAttachmentPreviews(messages);
		}, 'We could not load the earlier Messages.');
	}

	// Throws on a refusal; MessageThread shows it against its field.
	async function sendThreadMessage(body: string, attachment: File | undefined): Promise<void> {
		const created = await sendMessage<Message>(apiFetchWithSession, reference, body, attachment);
		messages = [...messages, created];
		await refreshAttachmentPreviews([created]);
	}

	async function handleDownloadAttachment(messageId: string, filename: string) {
		await messagesDownload.mutate(async () => {
			const blob = await downloadAttachment(apiFetchWithSession, reference, messageId);
			const url = URL.createObjectURL(blob);
			const link = document.createElement('a');
			link.href = url;
			link.download = filename;
			link.click();
			URL.revokeObjectURL(url);
		}, 'We could not download the attachment.');
	}
</script>

{#snippet takeOffControl()}
	<!--
		#2596: apart from the Doula row's add, so a destructive button never
		sits beside a benign one. The link opens a small form; its one
		button opens the confirm dialog, which names the act. It is behind
		the Doulas row's Change with the add, and under the list for a reader
		whose row has no Change.
	-->
	<stack-l space="var(--space-3)">
		<Button label="Take a Doula off" variant="link" expanded={isTakeOffShown} onClick={toggleTakeOff} />
		{#if isTakeOffShown}
			<StackedForm onSubmit={handleTakeOffSubmit}>
				{#if takeOffSubmission.errors.length > 0}
					<ErrorSummary errors={takeOffSubmission.errors} />
				{/if}
				<RadioGroup
					legend="Who comes off?"
					name={TAKE_OFF_GROUP}
					options={takeOffOptions}
					value={takeOffDoulaChosen?.staffId}
					onChange={(value) => (takeOffChoice = value)}
					error={takeOffSubmission.errorFor(radioFieldId(TAKE_OFF_GROUP, takeOffOptions[0]!.value))}
				/>
				<cluster-l space="var(--space-3)">
					<Button label="Take off" type="submit" size="sm" variant="destructive" loading={takeOffSubmission.isSubmitting} />
				</cluster-l>
			</StackedForm>
		{/if}
	</stack-l>
{/snippet}

{#snippet detailsRowValue(row: EngagementSummaryRow)}
	{#if row.contact}
		<Link href="{row.contact}:{row.href ?? row.value}" label={row.value} />
	{:else}
		{row.value}
	{/if}
{/snippet}

{#snippet detailsRowAction(row: EngagementSummaryRow, describedBy: string, isShown: boolean)}
	{#if row.isInvite}
		<Button
			label="Send portal invite"
			variant="link"
			{describedBy}
			onClick={handleSendPortalInvite}
			loading={isSendingPortalInvite}
		/>
	{:else}
		<Button
			label="Change"
			variant="link"
			expanded={isShown && openChange === row.change}
			{describedBy}
			onClick={() => toggleChange(row.change!)}
		/>
	{/if}
{/snippet}

{#snippet detailsRowForm(row: EngagementSummaryRow, isShown: boolean)}
	{#if isShown && row.change === 'care'}
		<!--
			#874: there is only ever one other kind to offer, so this
			is a single named button, not a question. canChangeKind
			is canWriteFact narrowed by ADR-0015's own upgrade caveat
			(see its declaration); the endpoint refuses her whether
			or not the control is drawn.
		-->
		<stack-l space="var(--space-3)">
			<cluster-l space="var(--space-3)">
				<Button
					label={`Change to ${kindLabel(otherKind)}`}
					size="sm"
					variant="secondary"
					loading={kindChange.isBusy}
					onClick={handleChangeKind}
				/>
			</cluster-l>
			{#if kindChange.error}
				<Notice variant="error" message={kindChange.error} />
			{/if}
		</stack-l>
	{/if}
{/snippet}

{#snippet details(view: 'aside' | 'section', isShown: boolean)}
	<!-- #2308: the Birth outcome section links here to reopen a completed
	     Engagement before its outcome can be removed. #2755: the region is drawn
	     twice (RecordDetail), so anything with an id, a dialog or a focus
	     behavior is drawn in the displayed copy only. -->
	<stack-l class="details-body" space="var(--space-4)">
		{#if view === 'aside'}
			<!--
				The facts' own shape (#2755): the label above, the value and its
				action on the value's line. In the 15 to 17rem column a
				`SummaryList` stacks its key and drops "Change" alone to the far edge.
			-->
			<dl class="aside-rows">
				{#each summaryRows as row, index (index)}
					<div>
						<dt>{row.label}</dt>
						<dd>
							<cluster-l space="var(--space-3)" align="baseline">
								<span class="aside-value">{@render detailsRowValue(row)}</span>
								{#if row.changes}
									{@render detailsRowAction(row, `details-${view}-${index}-changes`, isShown)}
									<span class="visually-hidden" id="details-{view}-{index}-changes">{row.changes}</span>
								{/if}
							</cluster-l>
						</dd>
						{#if isShown && row.isDetailShown}
							<dd class="aside-form">{@render detailsRowForm(row, isShown)}</dd>
						{/if}
					</div>
				{/each}
			</dl>
		{:else}
			<SummaryList rows={isShown ? summaryRows : summaryRows.map((row) => ({ ...row, isDetailShown: false }))}>
				{#snippet value(row)}{@render detailsRowValue(row)}{/snippet}
				{#snippet action(row, describedBy)}{@render detailsRowAction(row, describedBy, isShown)}{/snippet}
				{#snippet detail(row)}{@render detailsRowForm(row, isShown)}{/snippet}
			</SummaryList>
		{/if}

		{#if detailActs.length > 0}
			<ul class="acts">
				{#each detailActs as act (act.change)}
					<li>
						<Button
							label={act.label}
							variant="link"
							expanded={isShown ? openChange === act.change : false}
							focusOnAppearing={isShown && (refocusedChange === act.change || (act.change === 'doula' && isDoulaChangeFocused))}
							onClick={() => toggleChange(act.change)}
						/>
					</li>
				{/each}
			</ul>
		{/if}
		{#if isShown && openAct === 'due'}
			<Heading level={3} text={displayDueDate ? 'Change the due date' : 'Give the due date'} focusOnAppearing />
					<!--
						#2013: GOV.UK's date input, the boxes starting on the date
						held now. A refused date is a refused form, so the summary
						lists it and the box it is about is marked.
					-->
					<StackedForm onSubmit={handleDueSubmit}>
						{#if dueSubmission.errors.length > 0}
							<ErrorSummary errors={dueSubmission.errors} />
						{/if}
						<DateFields
							legend="New due date"
							hint="For example, 4 23 2027 for April 23, 2027."
							name={DUE_DATE_NAME}
							parts={dueParts}
							onChange={(next) => (dueParts = next)}
							error={dueRefusal?.message}
							invalidField={dueRefusal?.field}
						/>
						<cluster-l space="var(--space-3)">
							<Button
								label="Save the due date"
								type="submit"
								size="sm"
								loading={dueSubmission.isSubmitting}
							/>
							<Button
								label="Cancel"
								type="button"
								size="sm"
								variant="secondary"
								onClick={() => cancelChange('due')}
							/>
						</cluster-l>
					</StackedForm>
		{:else if isShown && openAct === 'carePhase'}
			<Heading level={3} text="Change the care phase" focusOnAppearing />
					<!--
						#253: exactly the moves ADR-0015's role table admits from
						the current status for this caller -- the API decides the
						set (Detail.statusMoves/TransitionResponse.statusMoves),
						this only renders it. Completing asks why first.
					-->
					<stack-l space="var(--space-3)">
						<cluster-l space="var(--space-3)">
							{#each displayStatusMoves as move (move)}
								<Button
									label={statusMoveLabel(move)}
									size="sm"
									variant="secondary"
									loading={directMove.isBusy}
									onClick={() => handleStatusMoveClick(move)}
								/>
							{/each}
						</cluster-l>
						{#if directMove.error}
							<Notice variant="error" message={directMove.error} />
						{/if}
						{#if isCompleteFormShown && birthOutcome.birthOutcome === undefined}
							<!--
								#2199: ADR-0015 completes no Engagement whose birth outcome
								is null (refuseUnexplainedCompletion), so she is told that
								first, in the order the rule needs, rather than after she
								has answered why it is ending. Once the outcome is recorded
								on this page, the overlaid `birthOutcome` brings the
								question in without a reload.
							-->
							<Notice variant="info" message={OUTCOME_BEFORE_COMPLETION} isFocusedOnAppear />
							<Link href="#birth-outcome" label="Record what happened" />
						{:else if isCompleteFormShown}
							<StackedForm onSubmit={handleCompleteSubmit}>
								{#if completeSubmission.errors.length > 0}
									<ErrorSummary errors={completeSubmission.errors} />
								{/if}
								<RadioGroup
									legend="Why is care ending?"
									name="ending-reason"
									options={endingReasons}
									value={completeReasonValue}
									onChange={(value) => (completeReasonValue = value)}
									error={completeSubmission.errorFor(endingReasonFieldId)}
								/>
								<LabeledField id="ending-note" label="Note (optional)">
									{#snippet children({ id, describedBy })}
										<Textarea
											{id}
											{describedBy}
											value={completeNoteValue}
											onInput={(value) => (completeNoteValue = value)}
										/>
									{/snippet}
								</LabeledField>
								<Button
									label="Confirm completion"
									type="submit"
									size="sm"
									loading={completeSubmission.isSubmitting}
								/>
								<Button
									label="Cancel"
									type="button"
									size="sm"
									variant="secondary"
									onClick={() => (isCompleteFormShown = false)}
								/>
							</StackedForm>
						{/if}
					</stack-l>
			<div><Button label="Cancel" variant="secondary" size="sm" onClick={() => cancelChange('carePhase')} /></div>
		{:else if isShown && openAct === 'doula'}
			<Heading level={3} text={detailActs.find((act) => act.change === 'doula')?.label ?? 'Doulas'} focusOnAppearing />
			{#if isAttachQuestion}
					<!--
						#2230: which Doula, and the save. No Cancel: the row's
						Change closes it, as it closes the care change above.
					-->
					<StackedForm onSubmit={handleAttachSubmit}>
						{#if attachSubmission.errors.length > 0}
							<ErrorSummary errors={attachSubmission.errors} />
						{/if}
						{#if isAttachSelect}
							<LabeledField
								id={ATTACH_GROUP}
								label={attachWords.question}
								hint={hasUnlistedContractor
									? 'A contractor is not listed: a contractor takes on work by accepting an Offer.'
									: undefined}
								error={attachSubmission.errorFor(ATTACH_GROUP)}
							>
								{#snippet children({ id, describedBy, invalid })}
									<Select
										{id}
										{describedBy}
										{invalid}
										options={attachOptions}
										placeholder="Choose a Doula"
										value={attachChoice}
										onChange={(value: string) => (attachChoice = value)}
									/>
								{/snippet}
							</LabeledField>
						{:else}
							<RadioGroup
								legend={attachWords.question}
								hint={hasUnlistedContractor
									? 'A contractor is not listed: a contractor takes on work by accepting an Offer.'
									: undefined}
								name={ATTACH_GROUP}
								options={attachOptions}
								value={attachChoice}
								onChange={(value) => (attachChoice = value)}
								error={attachSubmission.errorFor(attachFieldId())}
							/>
						{/if}
						<cluster-l space="var(--space-3)">
							<Button
								label={attachWords.button}
								type="submit"
								size="sm"
								loading={attachSubmission.isSubmitting}
							/>
						</cluster-l>
					</StackedForm>
					{#if canTakeOff}
						{@render takeOffControl()}
					{/if}
			{:else}
					<!--
						#1598: "Put me on this work", behind the Doula row's
						Change. The row has a Change only where the BFF says the
						press would be accepted: not for a contractor Doula, not
						for an Owner or an Admin who holds no Doula role, not for
						a person already on it, and not on a completed
						Engagement. Drawing only -- the endpoint refuses each of
						them whatever this page drew.
					-->
					<stack-l space="var(--space-3)">
						<cluster-l space="var(--space-3)">
							<Button
								label="Put me on this work"
								size="sm"
								variant="secondary"
								loading={selfAttach.isBusy}
								onClick={handleAttachSelf}
							/>
						</cluster-l>
						{#if selfAttach.error}
							<Notice variant="error" message={selfAttach.error} />
						{/if}
						{#if canTakeOff}
							{@render takeOffControl()}
						{/if}
					</stack-l>
			{/if}
			<div><Button label="Cancel" variant="secondary" size="sm" onClick={() => cancelChange('doula')} /></div>
		{/if}

		{#if isShown && canTakeOff && !canChangeDoula}
			{@render takeOffControl()}
		{/if}
		{#if isShown && takeOffDialog}
			<ConfirmDialog
				bind:open={isTakeOffConfirmShown}
				title={takeOffDialog.title}
				consequence={takeOffDialog.consequence}
				confirmLabel={takeOffDialog.confirm}
				error={takeOffError}
				onConfirm={confirmTakeOff}
			/>
		{/if}

		<!--
			What a Change did. Each takes focus: the control she pressed
			went with the form it was in, and focus would otherwise fall to
			the document (seen on #1598's 320px walk). The row above already
			shows the new value.
		-->
		{#if !isShown}
			<!-- The other copy says nothing a status would need to focus. -->
		{:else if selfAttachedMessage}
			<Notice variant="status" message={selfAttachedMessage} isFocusedOnAppear />
		{:else if kindChangedMessage}
			<Notice variant="status" message={kindChangedMessage} isFocusedOnAppear />
		{:else if dueChangedMessage}
			<Notice variant="status" message={dueChangedMessage} isFocusedOnAppear />
		{:else if statusChangedMessage}
			<Notice variant="status" message={statusChangedMessage} isFocusedOnAppear />
		{/if}

		<!--
			#500: the Client block. "View Client" alone doesn't say whose
			record it opens -- the same #513 defect the Client detail hub's
			own "Edit" link already solves, the same way: a sibling
			visually-hidden span joined by aria-describedby, so the announced
			name becomes "View Client, Pat Jordan" without a second visible
			word. Staff-only by construction rather than by a guard here: the
			portal's Engagement page is a wholly separate route
			(portal/(authenticated)/engagements/[engagementId]/+page.svelte)
			that never imports this component, matching ADR-0017's read table
			(a Client record is staff-only, never shown in the portal). The
			link itself needs no fresh access check either -- it only ever
			targets the Client this reader is already looking at through this
			Engagement, and reading this Engagement at all already passed
			ADR-0008's gate.
		-->
		<span class="visually-hidden" id="engagement-client-name-{view}">{detail!.clientName}</span>
		<cluster-l space="var(--space-4)" align="center">
			<Link href={clientDetailHref()} label="View Client" describedBy="engagement-client-name-{view}" />
			<!--
				Her other details, added from here as well as from her record
				(#1610). The journey's first Back and its save return to this
				page. Absent until her record is read, and for an erased record,
				whose edit `edit.go` refuses.
			-->
			{#if clientRecord && canAddDetails(clientRecord)}
				<span data-add-details>
					<Link
						href={detailsHref(page.params.practiceId!, detail!.clientId, page.params.engagementId!)}
						label={detailsLinkLabel(clientRecord)}
					/>
				</span>
			{/if}
		</cluster-l>

		<!--
			The outcome of the header's own action, and the only block-level
			room in the header block: `actions` is a cluster beside the h1,
			where an error banner and a full invite URL cannot go. Absorbed
			into the Template's existing regions rather than taken through one
			of ADR-0018's exits.
		-->
		<!--
			#255: an accepted Client is never offered a second invite -- the
			"Portal invite" row above already says "Accepted". It sends from
			that row's own action. A Client with no email gets a Notice naming
			the missing thing in place of the action (#270).
		-->
		{#if isShown && !hasAcceptedPortalAccess && !hasClientEmailOnFile}
			<Notice variant="info" message={noEmailForInviteMessage} />
		{/if}

		{#if isShown && portalInviteError}
			<Notice variant="error" message={portalInviteError} />
		{/if}

		{#if isShown && portalInviteLink}
			<Text text={`Invited. ${detail!.clientName} has an email with the link. To share the link yourself, here it is:`} />
			<div><code>{portalInviteLink}</code></div>
		{/if}
	</stack-l>
{/snippet}
{#snippet actions()}
	<!--
		#1998, #2755: the header holds one move, a link styled as a secondary
		button (one decision on a phone's first screen). "Message the Client"
		opens the compose in this page's own Messages section, which every
		reader of the page has, and puts focus in the Message field; with no
		compose to open (a Client who cannot read one) it goes to the section.
		"Add a Visit" is the Visits section's own first control. Nothing here
		is destructive.
	-->
	{#if !isClientErased}
		<ul class="moves">
			<li><Link href="#messages" label="Message the Client" variant="outline" onClick={handleMessageTheClient} /></li>
		</ul>
	{/if}
{/snippet}

{#snippet statusLine()}
	<EngagementStatusLine
		carePhase={engagementStatusText(displayStatus)}
		dueDay={displayDueDate && birthOutcome.birthOutcome === undefined ? displayDueDate : undefined}
		weeks={displayDueDate && birthOutcome.birthOutcome === undefined ? weeksAlong(displayDueDate, timeZone) : undefined}
		doulaLabel={doulaSummaryItem(attachedDoulas, displayStatus)?.label}
		doulas={doulaSummaryItem(attachedDoulas, displayStatus)?.value}
		isNobodyOnCallTonight={isNobodyTonight}
	/>
{/snippet}

{#snippet loadFailure(what: string, retry: () => void)}
	<!-- #2755: a section that could not be read says so, and reads again on request; the other sections stand. -->
	<Notice variant="error" message="We could not load {what}." />
	<div><Button label="Try again" variant="secondary" onClick={retry} /></div>
{/snippet}

{#snippet factValue(row: FactRow)}
	<span class="fact {row.tone ?? ''}">
		{#if row.tone}<Icon name={row.tone === 'error' ? 'warning-circle' : 'warning'} size={16} />{/if}
		{#if row.href}
			<Link href={row.href} label={row.value} ariaLabel={row.linkName ?? `${row.value}, ${row.hiddenWords}`} onClick={row.onClick} />
		{:else}
			{row.value}
		{/if}
	</span>
{/snippet}

<!--
	#2755: the facts a reader wants on arriving. Each value is the link to
	its section, with the words that name the fact in its accessible name.
	A read that failed says so, never a state it did not read.
-->
{#snippet facts()}
	<dl>
		{#each factRows as row (row.label)}
			<div>
				<dt>{row.label}</dt>
				<dd>
					{@render factValue(row)}
					{#if row.detail}<span class="fact-detail">{row.detail}</span>{/if}
				</dd>
			</div>
		{/each}
	</dl>
{/snippet}

{#snippet visitWhenCell(visit: Visit)}
	<VisitWhen at={visit.scheduledAt} {timeZone} />
{/snippet}

{#snippet visitDoulaCell(visit: Visit, view: DataTableView)}
	<VisitDoula name={visit.staffName} isOffEngagement={visit.isOffEngagement} isTracked={view === 'table'} />
{/snippet}

{#snippet visitActions(visit: Visit, view: DataTableView)}
	<!--
		#1999: a row holds its facts and the one action that opens its
		change, never the form itself. #666: DataTable renders this snippet
		into both of its trees, so the name span's id is scoped by which
		tree is asking.
	-->
	<span class="visually-hidden" id="{view}-visit-{visit.visitId}-name"
		>{visit.staffName}, {formatScheduledVisit(visit.scheduledAt, timeZone)}</span
	>
	<cluster-l class="visit-actions" space="var(--space-3)">
		<Button
			label="Change"
			variant="link"
			expanded={changingVisit?.visitId === visit.visitId}
			describedBy="{view}-visit-{visit.visitId}-name"
			onClick={() => toggleVisitChange(visit)}
			focusOnAppearing={refocusedVisitOpener === visit.visitId}
		/>
		<!-- #2231: an Owner's or an Admin's, where somebody else can take it. -->
		{#if canAssignVisits && reassignOptionsFor(visit).length > 0}
			<Button
				label="Reassign"
				variant="link"
				expanded={reassigningVisit?.visitId === visit.visitId}
				describedBy="{view}-visit-{visit.visitId}-name"
				onClick={() => toggleVisitReassign(visit)}
			/>
		{/if}
	</cluster-l>
{/snippet}

{#snippet addVisitForm()}
	<StackedForm onSubmit={handleCreateVisit}>
		<Heading level={3} text="Add a Visit" id="add-visit" focusOnAppearing />
		<!--
			Gated on hasAttemptedCreateVisit, not on newVisitBlock alone:
			#911's own live block (an ineligible pick) fires the instant
			she chooses a name, well before any submit, and ErrorSummary's
			own contract ("cannot steal focus from someone who has not
			submitted") assumes errors only ever arrive from a submit.
			Without this gate, picking an ineligible name would yank focus
			to the summary mid-selection (#1228).
		-->
		{#if hasAttemptedCreateVisit && newVisitBlock}
			<ErrorSummary errors={[{ message: newVisitBlock, targetId: 'new-visit-staff' }]} />
		{/if}
		{#if visitsCreate.error}
			<Notice variant="error" message={visitsCreate.error} />
		{/if}
		{#if canAssignVisits}
			<!--
				#909: this picker opens on the reader herself when she is on
				the roster, with her own option first and marked "(you)".
				GOV.UK's Select guidance says not to pre-select an option for
				a question, and this departs from it on purpose -- the reason
				is recorded in docs/design/govuk-alignment.md, on the commit
				that departed.
				#911: and every name says whether this Engagement can admit
				her, with the hint explaining the marker and the error
				refusing the choice before anything is sent.
			-->
			<LabeledField
				id="new-visit-staff"
				label="Who is this Visit for?"
				hint={assigneeHint}
				error={newVisitBlock}
				width={widthToHold(createAssigneeOptions)}
			>
				{#snippet children({ id, describedBy, invalid, width })}
					<Select
						{id}
						{describedBy}
						{invalid}
						{width}
						options={createAssigneeOptions}
						placeholder="Choose a Doula"
						value={visitStaffId}
						onChange={(value) => (newVisitStaffId = value)}
						required
					/>
				{/snippet}
			</LabeledField>
		{/if}
		<!-- A date-time control sizes to its own segments (#1216). -->
		<LabeledField id="new-visit-scheduled-at" label="Scheduled date and time (optional)">
			{#snippet children({ id, describedBy, invalid })}
				<TextInput
					{id}
					{describedBy}
					{invalid}
					type="datetime-local"
					value={newVisitScheduledAt}
					onInput={(value) => (newVisitScheduledAt = value)}
				/>
			{/snippet}
		</LabeledField>
		<cluster-l space="var(--space-3)">
			<Button label="Save Visit" type="submit" loading={isCreatingVisit} />
			<Button label="Cancel" variant="secondary" onClick={cancelAddVisit} />
		</cluster-l>
	</StackedForm>
{/snippet}

{#snippet visitHeading(act: string, visit: Visit, isFocused = true)}
	{#key visit.visitId}
		<Heading
			level={3}
			text={visit.scheduledAt
				? `${act} the Visit for ${visit.staffName} on ${formatScheduledVisit(visit.scheduledAt, timeZone)}`
				: `${act} the Visit for ${visit.staffName}, not yet scheduled`}
			focusOnAppearing={isFocused}
		/>
	{/key}
{/snippet}

{#snippet reassignVisitForm(visit: Visit, view: DataTableView)}
	{@const reassignFieldId = `reassign-visit-staff-${view}-${visit.visitId}`}
	{@const options = reassignOptionsFor(visit)}
	<!--
		#2231: who takes it instead, and the save. #268/#274: asked only of a
		reader the roster read admits, the Owner/Admin rule the BFF applies
		to the write. #909: the Doula it is for now is not offered. #911:
		the rest are marked the way the add picker marks them.
	-->
	<StackedForm onSubmit={(event) => handleReassignVisit(event, visit)}>
		{@render visitHeading('Reassign', visit)}
		<!-- The same gate the add form's summary carries, for #1228's reason. -->
		{#if hasAttemptedReassign && reassignBlock}
			<ErrorSummary errors={[{ message: reassignBlock, targetId: reassignFieldId }]} />
		{/if}
		{#if visitReassign.error}
			<Notice variant="error" message={visitReassign.error} />
		{/if}
		<LabeledField
			id={reassignFieldId}
			label="Who takes this Visit instead?"
			hint={assigneeHint}
			error={reassignBlock}
			width={widthToHold(options)}
		>
			{#snippet children({ id, describedBy, invalid, width })}
				<Select
					{id}
					{describedBy}
					{invalid}
					{width}
					{options}
					placeholder="Choose a Doula"
					value={reassignStaffId}
					onChange={(value) => (reassignStaffId = value)}
					required
				/>
			{/snippet}
		</LabeledField>
		<div>
			<Button label="Reassign the Visit" type="submit" loading={visitReassign.isBusy} />
		</div>
	</StackedForm>
{/snippet}

{#snippet changeVisitForm(visit: Visit, view: DataTableView)}
	<StackedForm onSubmit={(event) => handleChangeVisit(event, visit)}>
		<!-- A Visit that has happened is changed to write its notes, so focus
		     goes into Notes, the caret at the end; a Visit still to come is
		     changed to move its day, so focus stays on the heading (T3). -->
		{@const isNotesFirst = visitGroup(visit, visitsNow).key === 'earlier'}
		{@render visitHeading('Change', visit, !isNotesFirst)}
		{#if visitChange.error}
			<Notice variant="error" message={visitChange.error} />
		{/if}
		<!-- #250: an empty value clears the schedule; emptying the control
		     is the plainer way to ask for that than a separate Clear. -->
		<LabeledField
			id="change-visit-scheduled-at-{view}-{visit.visitId}"
			label="Scheduled date and time"
			hint="Leave it empty if the Visit is not scheduled."
		>
			{#snippet children({ id, describedBy, invalid })}
				<TextInput
					{id}
					{describedBy}
					{invalid}
					type="datetime-local"
					value={changeScheduledAt}
					onInput={(value) => (changeScheduledAt = value)}
				/>
			{/snippet}
		</LabeledField>
		<!-- #251: any Staff member who may read this Visit may also write its
		     notes, so this question is asked of every reader. -->
		<div {@attach isNotesFirst ? focusNotesAtEnd : undefined}>
			<LabeledField id="change-visit-notes-{view}-{visit.visitId}" label="Notes">
				{#snippet children({ id, describedBy, invalid })}
					<Textarea
						{id}
						{describedBy}
						{invalid}
						value={changeNotes}
						onInput={(value) => (changeNotes = value)}
					/>
				{/snippet}
			</LabeledField>
		</div>
		<cluster-l space="var(--space-3)">
			<Button label="Save changes" type="submit" loading={visitChange.isBusy} />
			<Button label="Cancel" variant="secondary" onClick={() => cancelVisitChange(visit)} />
		</cluster-l>
	</StackedForm>
{/snippet}

{#snippet visitForm(visit: Visit, view: DataTableView)}
	{#if changingVisit?.visitId === visit.visitId}
		{@render changeVisitForm(visit, view)}
	{:else if reassigningVisit?.visitId === visit.visitId && canAssignVisits}
		{@render reassignVisitForm(visit, view)}
	{/if}
{/snippet}

{#snippet visitsSection()}
	{#if rosterState.error}
		<Notice variant="error" message={rosterState.error} />
	{/if}

	{#if !visitsLoad.error && visitsZoneNote}
		<Text text={visitsZoneNote} tone="muted" />
	{/if}

	<!--
		#268/#274: adding is offered to a reader who can actually complete
		it -- an Owner or Admin, who picks the colleague it is for, or a
		Doula, who logs her own. A reader who is neither is offered no
		control rather than a button that 403s. #1999: one secondary
		opener, as the drawing has its actions under the table; the form,
		and its one primary, only when asked.
	-->
	{@const addOpener =
		canAddVisit && openVisitChange !== 'add'
			? {
					label: 'Add a Visit',
					onClick: () => openVisitForm('add'),
					focusOnAppearing: refocusedVisitOpener === 'add'
				}
			: undefined}
	{#if !visitsLoad.hasSettled}
		<!-- #2258: until the read lands the section says it is loading,
		     never "No Visits yet.". -->
		<Skeleton label="Loading the Visits" />
	{:else if visitsLoad.error}
		{@render loadFailure('the Visits', loadVisits)}
	{:else}
	{#if visits.items.length > 0}
		<cluster-l space="var(--space-4)" align="center">
			<Text
				text={nextVisit?.scheduledAt
					? `Next: ${formatScheduledVisit(nextVisit.scheduledAt, timeZone)}, ${nextVisit.staffName}.`
					: 'No Visit scheduled.'}
			/>
			{#if addOpener}
				<Button
					label={addOpener.label}
					variant="secondary"
					onClick={addOpener.onClick}
					focusOnAppearing={addOpener.focusOnAppearing}
				/>
			{/if}
			{#if lastVisit?.scheduledAt}
				<Link
					href="#visit-{lastVisit.visitId}"
					label="Last Visit: {formatCalendarWeekdayDay(dayIn(new Date(lastVisit.scheduledAt), timeZone), timeZone)}"
					onClick={showLastVisit}
				/>
			{/if}
		</cluster-l>
	{/if}
	{#if visitsSavedMessage}
		<Notice variant="status" message={visitsSavedMessage} isFocusedOnAppear />
	{/if}
	{#if erasedVisitsNote}
		<Notice variant="info" message={erasedVisitsNote} />
	{/if}
	{#if openVisitChange === 'add' && canAddVisit}
		{@render addVisitForm()}
	{/if}
	<DataTable
		columns={[
			{ label: 'When', accessor: (visit: Visit) => formatScheduledVisit(visit.scheduledAt, timeZone), content: visitWhenCell },
			{ label: 'Type', accessor: (visit: Visit) => visitTypeLabel(visit.type) },
			{
				label: 'Doula',
				accessor: (visit: Visit) => visitDoulaText(visit.staffName, visit.isOffEngagement),
				content: visitDoulaCell
			},
			// #2091: a note is a Doula's prose beside three facts, so it takes
			// the room they leave and wraps there, and the facts (a Doula's
			// "Priya Raman" included) keep one line each (#2755). The table's
			// floor is then the facts beside the note's longest word, under
			// DataTable's own 55rem, so this route sets no floor of its own.
			{ label: 'Notes', accessor: (visit: Visit) => visit.notes || 'No notes yet.', grows: true }
		]}
		columnWidths="one-line"
		rows={shownVisits}
		rowGroup={(visit: Visit) => visitGroup(visit, visitsNow)}
		groupHeadingLevel={3}
		rowId={visitRowId}
		rowActions={{ label: 'Actions', content: visitActions }}
		rowDetail={{ isOpen: isVisitFormOpen, content: visitForm }}
		hasMore={isEarlierVisitsShown && visits.hasMore}
		onLoadMore={() => visits.loadMore()}
		isLoadingMore={visits.isLoadingMore}
		loadMoreError={visits.loadMoreError}
		emptyMessage="No Visits yet."
		emptyAction={addOpener}
	/>
	{#if hasEarlierVisits}
		<div><Button label="Show earlier Visits" variant="secondary" onClick={() => (isEarlierVisitsShown = true)} /></div>
	{/if}
	{/if}
{/snippet}

<!--
	One body for both Plan sections, parameterised the way the `planSections`
	loop was: they are the same generic Plan Instance API with a different
	plan type (see planInstance.ts). A Template section's `content` takes no
	arguments, so each type gets a thin wrapper below rather than the loop.
-->
{#snippet planSectionBody(planType: PlanType, heading: string)}
	{@const instance = planState[planType].value}
	{#if planLoaded[planType]}
		{#if planLoadFailed[planType] && !instance}
			{@render loadFailure(`the ${heading}`, () => loadPlan(planType))}
		{:else if instance && shownPlanForms[planType]}
			<StackedForm onSubmit={(event) => handleSavePlan(event, planType, heading)}>
				<Heading level={3} text="Change the {heading}" focusOnAppearing />
				{#if planState[planType].error}
					<Notice variant="error" message={planState[planType].error} />
				{/if}
				<PlanInstanceForm
					fields={instance.fields}
					answers={planDraft[planType]}
					onAnswerChange={(fieldId, value) => handlePlanAnswerChange(planType, fieldId, value)}
					onToggleOption={(fieldId, option) => handlePlanToggleOption(planType, fieldId, option)}
					headingLevel={4}
				/>
				<cluster-l space="var(--space-3)">
					<Button label="Save {heading}" type="submit" loading={planState[planType].isBusy} />
					<Button label="Cancel" variant="secondary" onClick={() => cancelPlanForm(planType)} />
				</cluster-l>
			</StackedForm>
		{:else}
			{#if planState[planType].error}
				<Notice variant="error" message={planState[planType].error} />
			{/if}
			{#if instance}
				<Text text={planStateLine(instance, planType)} />
				{#if planSavedMessage[planType]}
					<Notice variant="status" message={planSavedMessage[planType]} isFocusedOnAppear />
				{/if}
				<!--
					#2243, #2755: the answered questions read here, at a capped
					height; the whole text stays in the accessibility tree.
					Changing them stays its own act.
				-->
				{#if hasPlanAnswers(instance)}
					<div
						class="plan-answers column"
						class:capped={!expandedPlans[planType]}
						class:care={planType === 'care_plan'}
						class:clipped={clippedPlans[planType] && !expandedPlans[planType]}
						{@attach watchPlanClip(planType)}
					>
						<PlanInstanceView fields={instance.fields} answers={instance.answers} headingLevel={3} answeredOnly width="column" />
					</div>
				{/if}
				<cluster-l space="var(--space-3)">
					<!--
						#280: the reading and taking-away surface -- its own
						address. Editing stays here.
					-->
					{#if planType === 'birth_plan' && birthPlanOffer.offersBirthPlan}
						<Link href={birthPlanHref} label="Read or print the whole Birth Plan" icon="file-text" variant="secondary" />
					{:else if clippedPlans[planType]}
						<Button
							label="Show the whole {heading}"
							variant="secondary"
							expanded={expandedPlans[planType]}
							onClick={() => (expandedPlans[planType] = !expandedPlans[planType])}
						/>
					{/if}
					<Button
						label="Change the {heading}"
						variant="secondary"
						onClick={() => openPlanForm(planType)}
						focusOnAppearing={refocusedPlanOpener === planType}
					/>
				</cluster-l>
			{:else if planType === 'birth_plan' && !birthPlanOffer.offersBirthPlanToStart}
				<EmptyState message={displayKind === 'birth' ? NO_BIRTH_PLAN_AFTER_OUTCOME : NO_BIRTH_PLAN_ON_POSTPARTUM} />
			{:else}
				<EmptyState
					message={erasedBecause(`the ${heading} cannot be started`) ?? `The ${heading} is not started yet.`}
					action={isClientErased ? undefined : { label: `Start the ${heading}`, onClick: () => handleStartPlan(planType) }}
				/>
			{/if}
		{/if}
	{:else}
		<Skeleton label="Loading the {heading}" />
	{/if}
{/snippet}

{#snippet carePlanSection()}
	{@render planSectionBody('care_plan', 'Care Plan')}
{/snippet}

{#snippet birthPlanSection()}
	{@render planSectionBody('birth_plan', 'Birth Plan')}
{/snippet}

<!--
	#1885: block over warn -- the reason a Contract cannot be created yet,
	before the click, and the way out. Until the rates are known the action
	stays withheld too: a click then could meet the same refusal. Shared by
	the first Contract and the one after a void (#2260).
-->
{#snippet contractRateBlock()}
	{#if ratesState.error}
		<Notice variant="error" message={ratesState.error} />
	{:else if isRateMissing}
		<Notice variant="info" message={noRateMessage} />
		{#if isPracticeOwnerOrAdmin}
			<Link
				href={resolve('/practices/[practiceId]/settings/rates', { practiceId: page.params.practiceId! })}
				label={`Set the ${displayKind} rate`}
			/>
		{/if}
	{/if}
{/snippet}

{#snippet contractSection()}
	{#if isContractLoaded}
		{#if detail.hasContract && !contract && contractError}
			{@render loadFailure('the Contract', loadContractSection)}
		{:else if contract && isContractFormOpen}
			<!-- #2004: a Draft's merge fields, only when asked. -->
			<StackedForm onSubmit={handleSaveContract}>
				<Heading level={3} text="Change the Draft Contract" focusOnAppearing />
				{#if contractError}
					<Notice variant="error" message={contractError} />
				{/if}
				<ContractForm
					mergeFields={contractEditableFields}
					values={contractDraft}
					readOnly={false}
					onValueChange={handleContractValueChange}
				/>
				<cluster-l space="var(--space-3)">
					<Button label="Save Contract" type="submit" loading={isContractBusy} />
					<Button label="Cancel" variant="secondary" onClick={cancelContractForm} />
				</cluster-l>
			</StackedForm>
		{:else if contract}
			{#if contractError}
				<Notice variant="error" message={contractError} />
			{/if}
			<ContractStatus
				{timeZone}
				status={contract.status}
				hasSignedPdf={contract.hasSignedPdf}
				amountChangedAt={contract.amountChangedAt}
				voidRequests={contract.voidRequests}
				viewerId={data.session.staffId}
				sent={contract.sent}
				signed={contract.signed}
				openedAt={contract.openedAt}
				reminded={contract.reminded}
				canRemindOn={contract.canRemindOn}
				remindWithheld={contract.remindWithheld}
				onRemind={contract.canRemind ? handleRemindContract : undefined}
				rows={contractFeeRows}
				onVoid={isPracticeOwnerOrAdmin ? handleVoidContract : undefined}
				onDownloadPdf={canReadContractMoney ? handleDownloadSignedContractPdf : undefined}
				onRequestVoid={isPracticeOwnerOrAdmin ? undefined : handleRequestVoidContract}
				onDeclineVoidRequest={isPracticeOwnerOrAdmin ? handleDeclineVoidRequest : undefined}
			>
				{#if contractSavedMessage}
					<Notice variant="status" message={contractSavedMessage} isFocusedOnAppear />
				{/if}
				<!--
					#258: Staff reads the same filled document the Client will,
					before sending it. #2004: behind a disclosure, closed, so the
					section is its facts first and the text is one press away.
				-->
				<Details summary={canReadContractMoney ? 'Read the Contract text' : 'Read the Client’s Contract text'}>
					{#if !canReadContractMoney}
						<Text
							text="This is the Contract the Client signs with the Practice. What you agreed to is listed above."
							tone="muted"
						/>
					{/if}
					<ContractView prose={contract.prose} values={contract.values} />
				</Details>
				{#if contract.status === 'draft'}
					<!--
						#258: block over warn -- a blank merge field is a legal
						document defect, so Send is withheld with the reason and
						the fields named, rather than reporting the server's
						refusal after the click. #255: a Contract sent to a
						never-invited Client would land in 'sent' with nothing able
						to move it back out, so Send waits for the portal invite --
						the same precondition PostSendContractHandler enforces. A
						withheld Send is absent, never disabled (govuk-alignment.md,
						the Button row): the Notice is the reason.
					-->
					{#if missingMergeFields.length > 0}
						<Notice
							variant="info"
							message={`Fill in every merge field before sending the Contract. Missing: ${missingMergeFields
								.map((key) => mergeFieldLabel(key))
								.join(', ')}.`}
						/>
					{:else if isInviteRefused}
						<Notice variant="info" message={refusedBeforeContractMessage} />
					{:else if hasNeverInvitedClient}
						<Notice variant="info" message={inviteBeforeContractMessage} />
					{:else if !canClientReachPortal}
						<Notice variant="info" message={newInviteBeforeContractMessage} />
					{/if}
					<cluster-l space="var(--space-3)">
						{#if missingMergeFields.length === 0 && canClientReachPortal}
							<Button label="Send Contract" onClick={handleSendContract} loading={isContractBusy} />
						{/if}
						{#if contractEditableFields.length > 0}
							<Button
								label="Change the Draft Contract"
								variant="secondary"
								onClick={openContractForm}
								focusOnAppearing={isContractOpenerRefocused}
							/>
						{/if}
					</cluster-l>
				{/if}
			</ContractStatus>
			{#if contract.earlier}
				<EarlierContracts earlier={contract.earlier} {timeZone} />
			{/if}
			{#if contract.status === 'voided'}
				<!--
					#2260: a voided Contract ends nothing about the Engagement, and
					the Invoices section below says to send a new one, so the
					section offers the act its own message names, under the same
					rate gate as the first (#1885). The API allows one Contract
					that is not voided beside any number that are (#72); the new
					Draft names the voided one's signed PDF as the earlier
					Contract's (#1229). Create and void are each on the
					Engagement's activity, with who did them.
				-->
				{@render contractRateBlock()}
				{#if canCreateContract}
					<cluster-l space="var(--space-3)">
						<Button label="Create a Draft Contract" variant="secondary" onClick={handleCreateContract} />
					</cluster-l>
				{/if}
			{/if}
		{:else}
			<!--
				#2003: the reader's own agreed fee, where the drawing puts it,
				even before a Contract exists. ADR-0008 gives a contractor this
				and never the Practice's price; the BFF sends it only to the
				person whose fee it is.
			-->
			{#if data.yourFee}
				<SummaryList rows={yourFeeRows(data.yourFee)} width="column" />
			{/if}
			{#if contractError}
				<Notice variant="error" message={contractError} />
			{/if}
			{@render contractRateBlock()}
			{#if contractInviteBlock}
				<!--
					#2146: a Client signs in the portal, so before there is a
					Contract the section says so and offers the invite beside
					Create, in the words the Draft's Send gate and the header
					already use. With no email address, the reason it cannot be
					sent takes the invite's place.
				-->
				<EmptyState message="No Contract yet." />
				<Notice variant="info" message={contractInviteBlock} />
				{#if !hasClientEmailOnFile && !isClientErased}
					<Notice variant="info" message={noEmailForInviteMessage} />
				{/if}
				{#if hasClientEmailOnFile || canCreateContract}
					<cluster-l space="var(--space-3)">
						{#if hasClientEmailOnFile}
							<Button
								label="Send portal invite"
								variant="secondary"
								onClick={handleSendPortalInvite}
								loading={isSendingPortalInvite}
							/>
						{/if}
						{#if canCreateContract}
							<Button label="Create a Draft Contract" variant="secondary" onClick={handleCreateContract} />
						{/if}
					</cluster-l>
				{/if}
			{:else}
				<EmptyState
					message="No Contract yet."
					action={canCreateContract ? { label: 'Create a Draft Contract', onClick: handleCreateContract } : undefined}
				/>
			{/if}
		{/if}
	{:else}
		<Skeleton label="Loading the Contract" />
	{/if}
{/snippet}

{#snippet invoicesSection()}
	<!--
		#2146: the section stands before there is a Contract, as drawn, so
		the reader learns where money will appear and what it waits for. No
		action: there is nothing to raise an Invoice against yet.
	-->
	{#if !isContractLoaded}
		<Skeleton label="Loading the Invoices" />
	{:else if contract && contract.status !== 'draft' && contract.status !== 'sent'}
		{@render invoicesBody(contract.status)}
	{:else}
		<Text text="Invoices start once {detail.clientName} signs the Contract." />
	{/if}
{/snippet}

{#snippet invoicesBody(contractStatus: string)}
	{#if invoicesError}
		{@render loadFailure('the Invoices', loadInvoicesSection)}
	{/if}
	{#if billingModeState.error}
		<Notice variant="error" message={billingModeState.error} />
	{/if}
	{#if paymentTermsState.error}
		<Notice variant="error" message={paymentTermsState.error} />
	{/if}

	<!-- #1280: InvoiceSection's own todayIsoDate reads "today" in
	     practiceTimezone with no UTC fallback (ADR-0036's own rule for a
	     zone that will not load), so it waits here for a real zone name
	     rather than mounting with '' and throwing on its own first date
	     computation. A still-loading or failed read leaves this gap
	     blank; the Notice above already says why. -->
	{#if invoiceStateLine}
		<p class="state-line" class:overdue={invoiceStateLine.isOverdue}>
			{#if invoiceStateLine.isOverdue}<Icon name="warning-circle" size={16} />{/if}
			{invoiceStateLine.text}
		</p>
	{/if}
	{#if paymentTermsDays > 0 && isInvoicesSettled && !invoicesError}
		<InvoiceSection
			{invoices}
			{contractStatus}
			{billingMode}
			clientsCanPay={canClientsPay}
			hasClientEmail={hasClientEmailOnFile}
			createRefusal={erasedBecause('no Invoice can be created')}
			isOwner={isPracticeOwner}
			isOwnerOrAdmin={isPracticeOwnerOrAdmin}
			canReadInvoiceMoney={canReadContractMoney}
			contractorInvoices={contractorInvoicesState.value}
			billing={contractBilling}
			hasUnbilled={hasUnbilledForContractor}
			{practiceTimezone}
			{paymentTermsDays}
			engagementKind={detail.kind}
			engagementDueDate={displayDueDate}
			{paymentsSettingsHref}
			{landOnInvoiceId}
			onCreate={handleCreateInvoice}
			onRecordPayment={handleRecordPayment}
			onReversePayment={handleReversePayment}
			onRefundPayment={handleRefundPayment}
			onVoidInvoice={handleVoidInvoice}
			onWriteOffInvoice={handleWriteOffInvoice}
		/>
	{:else if !paymentTermsState.error && !invoicesError}
		<Skeleton label="Loading the Invoices" />
	{/if}
{/snippet}

{#snippet startNoteSection()}
	{#if detail.startNote}
		<p class="start-note">{detail.startNote.text}</p>
		<Text
			text={`Written by ${detail.startNote.writtenBy} on ${formatInstantWithClock(detail.startNote.writtenAt, timeZone)}.`}
			step="meta"
			tone="muted"
		/>
	{/if}
{/snippet}

{#snippet offersSection()}
	<!--
		The roster without the reader (#1598, offerDoulas), and otherwise
		as it is, not `doulas ?? []` (#1432): an empty list
		tells the form that there is no one to offer the work to, and
		`undefined` tells it that the read failed. The other two meanings of
		`undefined` do not reach this section. A refused read is a reader
		who is not an Owner or Admin, and the Offers read refuses her at
		the same gate (`staffauth.OwnerAndAdmin` on both), so the section
		is not drawn. A read still in flight cannot be seen here either:
		onMount awaits loadRoster before loadOffersSection, and this
		section is drawn only once the Offers have landed.
	-->
	{#if !offersState.hasSettled}
		<Skeleton label="Loading the Offers" />
	{:else if offersState.value === undefined}
		<!-- The section stands before its read lands for an Owner or an
		     Admin (#2258), whom the read never refuses, so nothing back is
		     a failure, said as Doula fees says one. -->
		{@render loadFailure('the Offers', loadOffersSection)}
	{:else}
	<OfferSection
		{timeZone}
		{offers}
		doulas={offerDoulas}
		clientName={detail!.clientName}
		dueDate={displayDueDate ?? ''}
		onCreate={handleCreateOffer}
		onWithdraw={handleWithdrawOffer}
		createRefusal={erasedBecause('this work cannot be offered')}
	/>
	{/if}
{/snippet}

{#snippet doulaFeesSection()}
	<!-- The date paid reads "today" in the Practice's zone, so the section
	     waits for it, as InvoiceSection does. -->
	{#if !doulaFeesState.hasSettled}
		<Skeleton label="Loading the Doula fees" />
	{:else if doulaFeesState.error}
		{@render loadFailure('the Doula fees', loadDoulaFeesSection)}
	{:else}
		<DoulaFeeSection
			fees={doulaFeesState.value ?? []}
			{practiceTimezone}
			onRecord={handleRecordDoulaFee}
			onRemove={handleRemoveDoulaFee}
		/>
	{/if}
{/snippet}

{#snippet messagesSection()}
	<!--
		#2104: the thread's two facts ahead of it, as drawn -- the newest
		message, and whether the Client is waiting on a reply (GLOSSARY.md,
		Unanswered). Unanswered is a row only while it is true.
	-->
	{#if !messagesLoad.hasSettled}
		<Skeleton label="Loading the Messages" />
	{:else if messagesLoad.error}
		{@render loadFailure('the Messages', loadMessages)}
	{:else if messages.length > 0}
		{@const since = unansweredSince(messages)}
		<!-- "Show earlier Messages" shares the state line's line (#2755), as Add a Visit does. -->
		<cluster-l space="var(--space-4)" align="center">
			<p class="state-line" class:warning={!!since}>
				{#if since}
					<Icon name="warning" size={16} />Unanswered since {formatActivityTimestamp(since, timeZone)}.
				{:else}
					Answered.
				{/if}
			</p>
			{#if hasEarlierMessages}
				<Button label="Show earlier Messages" variant="secondary" onClick={() => (isEarlierMessagesShown = true)} />
			{/if}
		</cluster-l>
	{/if}
	{#if messagesLoad.hasSettled && !messagesLoad.error}
	<div class="thread">
	<MessageThread
		{timeZone}
		messages={shownMessages}
		error={messagesError}
		hasMore={isMessagesHasMore && isEarlierMessagesShown}
		isLoadingOlder={isLoadingOlderMessages}
		onLoadOlder={handleLoadOlderMessages}
		onSend={sendThreadMessage}
		onDownloadAttachment={handleDownloadAttachment}
		{attachmentPreviewURLs}
		composeOpener="Write a message"
		emptyMessage="No Messages yet."
		{composeRequest}
		cannotSendReason={messageBlock}
	/>
	</div>
	{/if}
	<!-- The way out the block names, beside it (#2004). -->
	{#if messageBlock && hasNeverInvitedClient && hasClientEmailOnFile}
		<div>
			<Button
				label="Send portal invite"
				variant="secondary"
				onClick={handleSendPortalInvite}
				loading={isSendingPortalInvite}
			/>
		</div>
	{/if}
{/snippet}

<!--
	#486 AC4: the brief's ledger treatment -- a meta date column, the event
	in body text, the actor muted. Last in `sections` (below), matching the
	design brief's #433 amendment: it sits low on every page it appears on.
-->
{#snippet onCallDays()}
	<!--
		#2042: GOV.UK's Summary list with row actions, the record-page
		pattern (govuk-alignment.md, "Sections of a record page"). Each
		Doula's days are a row; an Owner's or an Admin's Change opens the
		two dates in place under it, because the first and the last day
		belong together on one form.
	-->
	{#if onCallDoulaRows.length > 0}
		<SummaryList rows={onCallDoulaRows} width="column">
			{#snippet value(row)}
				{#if row.from && row.to}
					On call <time datetime={row.from}>{sayDay(row.from)}</time> to <time datetime={row.to}>{sayDay(row.to)}</time>
				{:else}
					{row.value}
				{/if}
			{/snippet}
			{#snippet action(row, describedBy)}
				<Button
					label="Change"
					variant="link"
					expanded={isDaysChangeOpen(row.staffId)}
					{describedBy}
					onClick={() => toggleDaysChange(row.staffId)}
				/>
			{/snippet}
			{#snippet detail(row)}
				<StackedForm onSubmit={(event) => saveOnCallDays(event, { staffId: row.staffId, name: row.label })}>
					{#if onCallDaysError}
						<Notice variant="error" message={onCallDaysError} />
					{/if}
					<!-- An empty date is the window's own (`saveNarrowing`), so the
					     form says so rather than leaving a blank to guess at. -->
					<LabeledField label="First day on call" hint="Leave it empty to start when the window does.">

						{#snippet children({ id, describedBy, invalid })}
							<TextInput
								{id}
								{describedBy}
								{invalid}
								type="date"
								value={narrowFrom[row.staffId] ?? ''}
								onInput={(value) => (narrowFrom[row.staffId] = value)}
							/>
						{/snippet}
					</LabeledField>
					<LabeledField label="Last day on call" hint="Leave it empty to end when the window does.">
						{#snippet children({ id, describedBy, invalid })}
							<TextInput
								{id}
								{describedBy}
								{invalid}
								type="date"
								value={narrowTo[row.staffId] ?? ''}
								onInput={(value) => (narrowTo[row.staffId] = value)}
							/>
						{/snippet}
					</LabeledField>
					<div><Button type="submit" label="Save on-call days" /></div>
				</StackedForm>
			{/snippet}
		</SummaryList>
	{/if}

{/snippet}

{#snippet putAnotherDoulaOn()}
	{#if isAttachQuestion}
		<div>
			<Link href="#details" label="Put another Doula on this birth" onClick={focusDoulaChange} />
		</div>
	{:else if !isPracticeOwnerOrAdmin}
		<Text text="Only an Owner or Admin can put another Doula on this birth." tone="variant" />
	{/if}
{/snippet}

{#snippet onCallGaps(panel: EngagementOnCall)}
	<Heading level={3} text="When a Doula cannot be reached" />
	{#if gapError}
		<Notice variant="error" message={gapError} />
	{/if}
	{@const canRecordGap = panel.window !== undefined && gapStaffOptions.length > 0}
	{@const isGapFormOpen = openOnCallChange === 'gap'}
	{#if panel.gaps.length === 0}
		<EmptyState
			message="No times recorded yet."
			action={canRecordGap && !isGapFormOpen
				? {
						label: 'Record a time a Doula cannot be reached',
						onClick: () => openOnCall('gap'),
						focusOnAppearing: isGapOpenerRefocused
					}
				: undefined}
		/>
	{:else}
		<SummaryList rows={gapRows} width="column">
			{#snippet value(row)}
				<stack-l space="var(--space-1)">
					<cluster-l space="var(--space-3)" align="baseline">
						<span>
							<CoverageGapWhen startsAt={row.gap.startsAt} endsAt={row.gap.endsAt} {timeZone} now={gapNow} />
						</span>
						<Badge
							label={row.gap.coveringStaffName ? `${row.gap.coveringStaffName} covering` : 'Nobody covering'}
							variant={row.gap.coveringStaffName ? 'success' : 'warning'}
						/>
					</cluster-l>
					{#if row.gap.reason}
						<Text text={row.gap.reason} step="body-sm" tone="variant" />
					{/if}
				</stack-l>
			{/snippet}
			{#snippet action(row, describedBy)}
				<cluster-l space="var(--space-3)">
					<Button
						label="Change"
						variant="link"
						expanded={isGapFormOpen && gapBeingChangedId === row.gap.id}
						{describedBy}
						onClick={() => changeGap(row.gap)}
					/>
					<Button
						label="Clear"
						variant="link"
						{describedBy}
						onClick={() => clearCoverageGap(row.gap)}
					/>
				</cluster-l>
			{/snippet}
		</SummaryList>
		{#if !isGapFormOpen && (hasUncoveredGap || canRecordGap)}
			<cluster-l space="var(--space-4)" align="center">
				{#if hasUncoveredGap}
					{@render putAnotherDoulaOn()}
				{/if}
				{#if canRecordGap}
					<Button
						label="Record a time a Doula cannot be reached"
						variant="secondary"
						onClick={() => openOnCall('gap')}
						focusOnAppearing={isGapOpenerRefocused}
					/>
				{/if}
			</cluster-l>
		{/if}
	{/if}

	{#if canRecordGap && isGapFormOpen}
		<StackedForm onSubmit={submitCoverageGap}>
			{#if gapSubmission.errors.length > 0}
				<ErrorSummary errors={gapSubmission.errors} />
			{/if}
			<LabeledField
				id={gapFieldIds.staffId}
				label="Doula who cannot be reached"
				error={gapSubmission.errorFor(gapFieldIds.staffId)}
			>
				{#snippet children({ id, describedBy, invalid })}
					<Select
						{id}
						{describedBy}
						{invalid}
						options={gapStaffOptions}
						placeholder="Choose a Doula"
						bind:value={gapStaffId}
						required
					/>
				{/snippet}
			</LabeledField>
			<LabeledField
				id={gapFieldIds.startsAt}
				label="From"
				error={gapSubmission.errorFor(gapFieldIds.startsAt)}
			>
				{#snippet children({ id, describedBy, invalid })}
					<TextInput
						{id}
						{describedBy}
						{invalid}
						type="datetime-local"
						value={gapStartsAt}
						onInput={(value) => (gapStartsAt = value)}
					/>
				{/snippet}
			</LabeledField>
			<LabeledField
				id={gapFieldIds.endsAt}
				label="Until"
				error={gapSubmission.errorFor(gapFieldIds.endsAt)}
			>
				{#snippet children({ id, describedBy, invalid })}
					<TextInput
						{id}
						{describedBy}
						{invalid}
						type="datetime-local"
						value={gapEndsAt}
						onInput={(value) => (gapEndsAt = value)}
					/>
				{/snippet}
			</LabeledField>
			{#if gapStaffId === ''}
				<LabeledField
					id={gapFieldIds.coveringStaffId}
					label="Who is covering"
					hint={coverHint}
				>
					{#snippet children({ id, describedBy, invalid })}
						<Select {id} {describedBy} {invalid} options={[]} placeholder="Choose the Doula first" disabled />
					{/snippet}
				</LabeledField>
			{:else if gapCoveringOptions.length === 0}
				<stack-l space="var(--space-label)">
					<Text text="Who is covering" step="label" />
					<Text text="Nobody else is on this birth yet, so nobody can cover." tone="variant" />
					{@render putAnotherDoulaOn()}
				</stack-l>
			{:else}
				<LabeledField
					id={gapFieldIds.coveringStaffId}
					label="Who is covering"
					hint={coverHint}
					error={gapSubmission.errorFor(gapFieldIds.coveringStaffId)}
				>
					{#snippet children({ id, describedBy, invalid })}
						<Select
							{id}
							{describedBy}
							{invalid}
							options={[{ value: '', label: 'Nobody yet' }, ...gapCoveringOptions]}
							bind:value={gapCoveringStaffId}
						/>
					{/snippet}
				</LabeledField>
			{/if}
			<LabeledField
				id={gapFieldIds.reason}
				label="Reason"
				hint="Optional. Owners, Admins and the Doulas on this birth can read it; the Client never does."
				error={gapSubmission.errorFor(gapFieldIds.reason)}
			>
				{#snippet children({ id, describedBy, invalid })}
					<TextInput
						{id}
						{describedBy}
						{invalid}
						value={gapReason}
						onInput={(value) => (gapReason = value)}
					/>
				{/snippet}
			</LabeledField>
			<cluster-l space="var(--space-3)">
				<Button type="submit" label={gapBeingChangedId ? 'Save the gap' : 'Record the gap'} loading={gapSubmission.isSubmitting} />
				<Button label="Cancel" variant="secondary" onClick={cancelGapForm} />
			</cluster-l>
		</StackedForm>
	{/if}
{/snippet}

{#snippet onCallSection()}
	{#if onCallError}
		{@render loadFailure('who is on call', loadOnCall)}
	{:else if onCall}
		{#if tonightLine}
			<p class="state-line">{tonightLine.text}</p>
		{/if}
		{#if onCall.window}
			<p>
				On call from <time datetime={onCall.window.start}>{sayDay(onCall.window.start)}</time> to <time datetime={onCall.window.end}>{sayDay(onCall.window.end)}</time>{#if onCall.rule.startRule === 'gestational_week'}, from {onCall.rule.startWeek} weeks{/if}.
			</p>
		{:else}
			<Text text={describeNoWindow(onCall.noWindowReason ?? '')} tone="muted" />
			{#if putDoulaOnWords}
				<div>
					<Link href="#details" label={putDoulaOnWords} onClick={focusDoulaChange} />
				</div>
			{/if}
		{/if}

		{#if onCallSavedMessage}
			<Notice variant="status" message={onCallSavedMessage} isFocusedOnAppear />
		{/if}

		{@render onCallDays()}
		{@render onCallGaps(onCall)}
	{:else}
		<Skeleton label="Loading who is on call" />
	{/if}
{/snippet}

{#snippet birthOutcomeSection()}
	<BirthOutcomeSection
		outcome={birthOutcome.birthOutcome}
		endedOn={birthOutcome.pregnancyEndedOn}
		canRecord={canWriteFact}
		canCorrect={isPracticeOwner}
		reopenHref={displayStatus === 'completed' ? '#details' : undefined}
		onRecord={handleRecordBirthOutcome}
	/>
{/snippet}

{#snippet activitySection()}
	<!-- #2755: the kinds are the BFF's (?kind=), each re-reading the first page. -->
	<cluster-l space="var(--space-3)" align="baseline">
		<span class="filter-label" id="activity-filter-label">Show:</span>
		<ul class="activity-kinds" aria-labelledby="activity-filter-label"> <!-- spelling:ignore: aria-labelledby is an ARIA attribute name -->
			{#each ACTIVITY_KINDS as option (option.kind)}
				<li>
					<Link
						href={activityKindHref(option.kind)}
						label={option.label}
						current={option.kind === activityKind}
						onClick={(event) => {
							event.preventDefault();
							void goto(activityKindHref(option.kind), { reset: false });
						}}
					/>
				</li>
			{/each}
		</ul>
	</cluster-l>
	{#if !activityLoad.hasSettled || activityLoad.isBusy}
		<Skeleton label="Loading the Activity" />
	{:else if activityError}
		{@render loadFailure('the Activity', loadActivity)}
	{:else}
	<DataTable
		columns={activityLedgerColumns(timeZone).map((column) => (column.label === 'What' ? { ...column, grows: true, track: '56rem' } : column))}
		columnWidths="one-line"
		rows={shownActivity}
		hasMore={hasMoreActivity}
		onLoadMore={showMoreActivity}
		isLoadingMore={activity.isLoadingMore}
		loadMoreError={activity.loadMoreError}
		emptyMessage={activityKind === '' ? 'Nothing has happened yet.' : 'Nothing of this kind yet.'}
	/>
	{/if}
{/snippet}

{#snippet onCallEndedSection()}
	<Text text={onCallEndedLine} />
{/snippet}

<!--
	Archetype D, ADR-0018. Offers is Owner/Admin-only, which is why
	`sections` is a typed array and not a run of named regions.

	The order (#2146, #2755) follows the care phase and is settled from the
	Engagement read alone (sectionOrder.ts), so it is right on first paint
	and nothing moves when a later read lands. Every section shows even when
	empty (#2015): Invoices before a Contract says what it waits for.
-->
<!-- #1611, #1710: a save that leads here says what it did, such as
     "Work with Pat started." at the end of a new Client's flow. -->
<RecordDetail
	title={detail.clientDisplayName}
	{outcome}
	backHref={back?.href(clientsHref) ?? clientsHref}
	backLabel={back?.label('Clients') ?? 'Clients'}
	onBack={back?.go}
	{statusLine}
	{facts}
	{details}
	{actions}
	isContentsShown
	sections={orderedSections}
/>

<style>
	/* A Visit's Change and Reassign stay on one line, so Notes is what wraps. */
	.visit-actions {
		flex-wrap: nowrap;
		white-space: nowrap;
	}

	.moves {
		display: flex;
		flex-wrap: wrap;
		gap: var(--space-2);
		margin: 0;
		padding: 0;
		list-style: none;
	}

	.acts {
		display: grid;
		gap: var(--space-1);
		margin: 0;
		padding: 0;
		list-style: none;
	}

	/* Details beside the sections: the facts' own shape, the label above and
	   the value with its action on the value's line. */
	.aside-rows {
		display: grid;
		row-gap: var(--space-2);
		margin: 0;
	}

	.aside-rows dt {
		margin: 0;
		font-size: var(--text-meta-size);
		line-height: var(--text-meta-leading);
		color: var(--color-on-surface-muted);
	}

	.aside-rows dd {
		margin: 0;
		min-inline-size: 0;
		overflow-wrap: anywhere;
	}

	.aside-form {
		padding-block-start: var(--space-2);
	}

	.fact {
		display: flex;
		align-items: baseline;
		gap: var(--space-1);
	}

	/* Each fact's value is a 24px target, as "Jump to"'s links are, so five rows
	   hold the first screen on a phone. */
	.fact :global(a) {
		min-block-size: var(--target-min-aa);
	}

	/* A section's first line where its state needs a tone beside the words
	   (the Invoices'; red is for overdue, DESIGN.md's One Meaning Per Hue). */
	.state-line {
		margin: 0;
		max-inline-size: var(--measure);
		font-size: var(--text-body-size);
		line-height: var(--text-body-leading);
	}

	.state-line.overdue {
		color: var(--color-error);
	}

	.state-line.warning {
		color: var(--color-warning);
	}

	.filter-label {
		color: var(--color-on-surface-muted);
	}

	.activity-kinds {
		display: flex;
		flex-wrap: wrap;
		gap: var(--space-1) var(--space-4);
		margin: 0;
		padding: 0;
		list-style: none;
	}

	/* The width rule (design call, section 1): the thread, prose and the
	   record's plain blocks stop at the reading measure. */
	.thread {
		max-inline-size: var(--measure);
	}

	.state-line > :global(svg) {
		vertical-align: text-bottom;
		margin-inline-end: var(--space-1);
	}

	/* The plan's answers take the column (`column`, ADR-0018 2026-11-15), their
	   values at the reading width, at a capped height; the text beyond the cap
	   stays in the document, clipped and faded. */
	.plan-answers.capped {
		/* tokens:ignore -- the cap is a height chosen against the first screen of a phone (design call, S4a): 32rem holds the Birth Plan's pain-relief answer on 375x740, and no spacing token is that tall. */
		max-block-size: 32rem;
		overflow: hidden;
	}

	/* The Care Plan is internal notes read in passing, so its cap is shorter than the Birth Plan's (a phone's one-answer corridor). */
	.plan-answers.capped.care {
		/* tokens:ignore -- a height chosen against the record's length (design call r2, D10); no spacing token is that tall. */
		max-block-size: 12rem;
	}

	.plan-answers.clipped {
		mask-image: linear-gradient(to bottom, black calc(100% - var(--space-8)), transparent);
	}

	.fact-detail {
		display: block;
		font-size: var(--text-meta-size);
		line-height: var(--text-meta-leading);
		color: var(--color-on-surface-muted);
	}

	.fact > :global(svg) {
		flex: none;
		align-self: flex-start;
		margin-block-start: var(--space-1);
	}

	.fact.warning {
		color: var(--color-warning);
	}

	.fact.error,
	.fact.error :global(a) {
		color: var(--color-error);
	}

	.start-note {
		margin: 0;
		max-inline-size: var(--measure);
		white-space: pre-wrap;
		overflow-wrap: anywhere;
	}
</style>
