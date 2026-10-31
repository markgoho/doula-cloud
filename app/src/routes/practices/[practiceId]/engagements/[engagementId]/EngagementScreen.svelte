<script lang="ts">
	import { onDestroy, onMount } from 'svelte';
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
		attachSelf,
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
		visitTypeLabel,
		engagementStatusText,
		yourFeeValue,
		type BirthOutcomeFacts,
		type BirthOutcomeRequest,
		type EngagementSummary,
		type Visit,
		type WhoIsOn
	} from '#lib/engagementDetail.js';
	import { kindLabel } from '#lib/engagementRequest.js';
	import {
		assigneeBlock,
		attachableDoulaOptions,
		loadVisitAssigneesOrNone,
		unnameableHint,
		visitAssigneeOptions,
		type VisitAssignee
	} from '#lib/staff.js';
	import {
		hasAcceptedPortalInvite,
		hasNeverBeenInvited,
		portalInviteStatusText,
		type PortalInviteSubject
	} from '#lib/portalInvite.js';
	import type { PageData } from './$types';
	import { stampText } from '#lib/stamp.js';
	import { latestMessageText, unansweredSince } from '#lib/threadSummary.js';
	import { formatCalendarDay, formatInstant, formatScheduledVisit, toDatetimeLocalValue } from '#lib/dates.js';
	import {
		activityLedgerColumns,
		loadEngagementActivityPage,
		RECENT_ACTIVITY_ROWS,
		type ActivityEntry
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
	import ContractStatus from '#lib/components/molecules/ContractStatus.svelte';
	import ContractView from '#lib/components/molecules/ContractView.svelte';
	import {
		loadContract,
		createContract,
		saveContractValues,
		sendContract,
		voidContract,
		requestContractVoid,
		declineContractVoidRequest,
		downloadSignedContractPdf,
		setMergeFieldValue,
		mergeFieldLabel,
		missingMergeFieldKeys,
		editableMergeFields,
		editableValues,
		type Contract
	} from '#lib/contract.js';
	import { isAmbientContractor, isDoula, isOwner, isOwnerOrAdmin } from '#lib/roles.js';
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
		type BillingMode,
		type ContractorInvoice,
		type Invoice,
		type PaymentMethod,
		type RecordPaymentInput,
		type RefundPaymentInput
	} from '#lib/invoice.js';
	import { loadPracticeTimezone } from '#lib/practiceTimezone.js';
	import { loadRates, type Rate } from '#lib/rates.js';
	import OfferSection from '#lib/components/organisms/OfferSection.svelte';
	import DoulaFeeSection from '#lib/components/organisms/DoulaFeeSection.svelte';
	import { loadDoulaFees, recordDoulaFeePaid, removeDoulaFeePaid, type DoulaFee } from '#lib/doulaFee.js';
	import { createOffer, loadEngagementOffers, withdrawOffer, type NewOffer, type Offer } from '#lib/offer.js';
	import { resolve } from '$app/paths';
	import MessageThread, { type Message } from '#lib/components/organisms/MessageThread.svelte';
	import Text from '#lib/components/atoms/Text.svelte';
	import Link from '#lib/components/atoms/Link.svelte';
	import Button from '#lib/components/atoms/Button.svelte';
	import Notice from '#lib/components/atoms/Notice.svelte';
	import SummaryList, { type SummaryListRow } from '#lib/components/molecules/SummaryList.svelte';
	import PlanInstanceView from '#lib/components/molecules/PlanInstanceView.svelte';
	import RadioGroup, { radioFieldId } from '#lib/components/molecules/RadioGroup.svelte';
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
		outcome
	}: { data: Omit<PageData, 'read'> & EngagementSummary; outcome?: string } = $props();
	const detail = $derived(data);

	// The Contract's PDF download opens to an Owner, an Admin, and an
	// employed Doula (ADR-0008's money row as amended by #282) and refuses
	// only a contractor -- this drawing decision is not the gate:
	// contract.ts's downloadSignedContractPdf hits the real endpoint,
	// which refuses a contractor on its own.
	const isPracticeOwnerOrAdmin = $derived(isOwnerOrAdmin(data.session));
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

	// #1998: the drawing's "← Clients" back link, to the Practice's
	// Clients list, where every Engagement is reached from.
	const clientsHref = $derived(
		resolve('/practices/[practiceId]/clients', { practiceId: page.params.practiceId! })
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
	// moves, the kind change, and "Put me on this Engagement". One row is
	// open at a time. A save closes it and says what changed in a status
	// that takes focus, because the control she pressed goes with the
	// form; a refusal leaves it open, beside the refusal.
	type SummaryChange = 'care' | 'due' | 'carePhase' | 'doula';
	type EngagementSummaryRow = SummaryListRow & { change?: SummaryChange };
	const changeWords: Record<SummaryChange, string> = {
		care: 'care',
		due: 'due date',
		carePhase: 'care phase',
		doula: 'Doula'
	};
	let openChange = $state<SummaryChange | undefined>();
	let statusChangedMessage = $state('');

	function toggleChange(change: SummaryChange) {
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
		}, 'Failed to change status');
		if (!hasMoved) return;
		closeChange(`The care phase is now ${engagementStatusText(displayStatus)}.`);
		await refreshWhoIsOn();
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
				return [{ message: 'Select why this Engagement is ending', targetId: endingReasonFieldId }];
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
			await refreshWhoIsOn();
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
		if (result.kind === 'recorded') birthOutcomeOverride = result.facts;
		return result;
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
		await kindChange.mutate(async () => {
			const result = await changeEngagementKind(apiFetchWithSession, reference, target);
			kindOverride = result.kind;
			openChange = undefined;
			kindChangedMessage = `Now set to ${kindLabel(result.kind)}.`;
		}, 'Failed to change what the Practice sold');
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
	// whether "Put me on this Engagement" is hers to press
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
			selfAttachedMessage = 'You are on this Engagement.';
		}, 'We could not put you on this Engagement. Try again.');
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
			// "You are on this Engagement." was true when it was said. A
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
		failureMessage: 'Failed to load more Visits'
	});
	const visitsLoad = new SectionState<void>(undefined);
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
	const activity = new PaginatedList<ActivityEntry>({
		first: { items: [], hasMore: false },
		loadPage: (cursor) => loadEngagementActivityPage(apiFetchWithSession, reference, cursor),
		failureMessage: 'Failed to load more activity'
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
	let isWholeLedgerShown = $state(false);
	const shownActivity = $derived(
		isWholeLedgerShown ? activity.items : activity.items.slice(0, RECENT_ACTIVITY_ROWS)
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
	// action's own visibility updates without a reload. 'pending' is the
	// outbox row's own default status (portalinvite.invite()), so this
	// is exactly what a fresh send actually leaves behind.
	let clientPortalOverride = $state<PortalInviteSubject | undefined>();
	const clientPortalState = $derived<PortalInviteSubject>(
		clientPortalOverride ?? {
			portalInviteStatus: detail?.clientPortalInviteStatus,
			emailSuppressed: detail?.clientEmailSuppressed
		}
	);
	const hasNeverInvitedClient = $derived(hasNeverBeenInvited(clientPortalState));
	const hasAcceptedPortalAccess = $derived(hasAcceptedPortalInvite(clientPortalState));
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
	const messageBlock = $derived.by(() => {
		if (hasAcceptedPortalAccess) return;
		const name = detail.clientName;
		if (!hasClientEmailOnFile) {
			return `${name} has no email address on file, so cannot be invited to the portal to read a message. Add an email address first.`;
		}
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
	const noEmailForInviteMessage = 'This Client has no email address on file. Add one before sending a portal invite.';
	// #2146: what the empty Contract section says while the Client has no
	// portal account: the same three states as messageBlock above. An
	// invite already sent does not hold Send back (PostSendContractHandler
	// asks only that one was sent), so that state names the acceptance
	// signing waits on rather than asking for an invite again.
	const contractInviteBlock = $derived.by(() => {
		if (hasAcceptedPortalAccess) return;
		if (hasNeverInvitedClient || !hasClientEmailOnFile) return inviteBeforeContractMessage;
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
			changeScheduledAt = toDatetimeLocalValue(change.visit.scheduledAt);
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

	// The scheduled Visits by date, then the rest (#1999).
	const sortedVisits = $derived(sortVisits(visits.items));

	let messages = $state<Message[]>([]);
	let messagesCursor = $state('');
	let isMessagesHasMore = $state(false);
	const messageThreadRows = $derived.by<SummaryListRow[]>(() => {
		const latest = latestMessageText(messages);
		if (latest === undefined) return [];
		const since = unansweredSince(messages);
		return [
			{ label: 'Latest', value: latest },
			...(since ? [{ label: 'Unanswered', value: `Since ${formatInstant(since)}` }] : [])
		];
	});
	// Load (including "load older") and download are separate SectionState
	// instances so downloading an attachment never lights up the "Load
	// older" spinner. Both write the one Notice the thread renders. A
	// send is the compose form's own: MessageThread holds its refusals and
	// its busy state, against the field each names (#1952).
	const messagesLoad = new SectionState<void>(undefined);
	const messagesDownload = new SectionState<void>(undefined);
	const messagesError = $derived(messagesLoad.error || messagesDownload.error);
	const isLoadingOlderMessages = $derived(messagesLoad.isBusy);
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
	const invoicesError = $derived(invoicesState.error || contractorInvoicesState.error);

	// #271: undefined until this Practice has chosen a billing rail --
	// InvoiceSection itself decides what to render for that, from the
	// same standing fact clientsCanPay/hasClientEmail already are.
	const billingModeState = new SectionState<BillingMode | undefined>(undefined);
	const billingMode = $derived(billingModeState.value);

	// #1280: the Practice's own zone (ADR-0036) InvoiceSection reads
	// "today" in for the payment-date field's default and its
	// future-date ceiling, matching PostManualPaymentHandler's own
	// guard (#1167). Loaded once at mount (loadPracticeTimezoneSection
	// below) -- '' means "not loaded yet", which gates the Invoices
	// section out below rather than handing InvoiceSection a zone that
	// would fall back to UTC's own day.
	const practiceTimezoneState = new SectionState('');
	const practiceTimezone = $derived(practiceTimezoneState.value);

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
	const doulaFeesState = new SectionState<DoulaFee[] | undefined>(undefined);
	const isDoulaFeesVisible = $derived((doulaFeesState.value?.length ?? 0) > 0 || doulaFeesState.error !== '');

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
	const isOffersVisible = $derived(offersState.value !== undefined);
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
	let attachChoice = $state<string | undefined>();
	const attachSubmission = new FormSubmission();

	async function handleAttachSubmit(event: SubmitEvent) {
		event.preventDefault();
		selfAttachedMessage = '';
		let isAttached = false;
		await attachSubmission.run(async () => {
			const chosen = attachOptions.find((option) => option.value === attachChoice);
			if (!chosen) {
				return [{ message: 'Select who the Doula is', targetId: radioFieldId(ATTACH_GROUP, attachOptions[0]!.value) }];
			}
			whoIsOnOverride = await attachDoula(apiFetchWithSession, reference, chosen.value);
			openChange = undefined;
			attachChoice = undefined;
			selfAttachedMessage =
				chosen.value === callerStaffId
					? 'You are on this Engagement.'
					: `${doulas!.find((doula) => doula.staffId === chosen.value)!.name} is on this Engagement.`;
			isAttached = true;
		}, orThrownMessage);
		if (!isAttached) return;
		await loadOnCall();
		await loadActivity();
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
		if (displayDueDate) {
			rows.push({ label: 'Due', value: formatCalendarDay(displayDueDate), ...changeable('due', canWriteFact) });
		} else if (displayKind === 'birth' && birthOutcome.birthOutcome === undefined) {
			rows.push({ label: 'Due', value: 'Not given', ...changeable('due', canWriteFact) });
		}
		rows.push({
			label: 'Care phase',
			value: engagementStatusText(displayStatus),
			...changeable('carePhase', displayStatusMoves.length > 0)
		});
		// #1598: who is on it. No row for a completed Engagement nobody is
		// on (doulaSummaryItem).
		const doulaItem = doulaSummaryItem(attachedDoulas, displayStatus);
		if (doulaItem) rows.push({ ...doulaItem, ...changeable('doula', canChangeDoula) });
		rows.push(
			{ label: 'Portal invite', value: portalInviteSummaryText(clientPortalState, hasClientEmailOnFile) },
			// createdAt is an instant (engagement/detail.go).
			{ label: 'Created', value: formatInstant(detail.createdAt) }
		);
		return rows;
	});

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
		const text = portalInviteStatusText(portalState);
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
			visits.reset(await loadVisitsPage(apiFetchWithSession, reference, ''));
		}, 'Failed to load Visits');
	}

	async function loadActivity() {
		await activityLoad.load(async () => {
			activity.reset(await loadEngagementActivityPage(apiFetchWithSession, reference, ''));
		}, 'Failed to load activity');
	}

	async function loadMessages() {
		await messagesLoad.load(async () => {
			const loaded = await loadMessagesPage<Message>(apiFetchWithSession, reference, '');
			messages = loaded.items;
			messagesCursor = loaded.nextCursor ?? '';
			isMessagesHasMore = loaded.hasMore;
			await refreshAttachmentPreviews(messages);
		}, 'Failed to load Messages');
	}

	/** #301's Staff visibility (AC4), as the Birth Plan's read view says it
	 * (#2004): whether the Client has read it since it last changed, by
	 * her name. A Care Plan instance never carries clientAcknowledgedAt,
	 * so only the Birth Plan draws this row. */
	function birthPlanReadText(instance: Instance): string {
		return instance.clientAcknowledgedAt
			? `${detail.clientName} read this on ${formatInstant(instance.clientAcknowledgedAt)}`
			: `${detail.clientName} has not read this yet`;
	}

	async function loadPlan(planType: PlanType) {
		await planState[planType].load(
			() => loadInstance(apiFetchWithSession, page.params.practiceId!, page.params.engagementId!, planType),
			'Failed to load plan'
		);
		planLoaded[planType] = true;
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

	// A Plan's one audit row (#2104, #2133): the last save once there is
	// one, which also says the Plan was started; until then, the start.
	function planTrailRows(instance: Instance): SummaryListRow[] {
		if (instance.lastEdited) return [{ label: 'Last edited', value: stampText(instance.lastEdited) }];
		if (instance.started) return [{ label: 'Started', value: stampText(instance.started) }];
		return [];
	}

	// A new Plan has nothing to read yet, so starting one opens its
	// questions at once.
	async function handleStartPlan(planType: PlanType) {
		const isStarted = await planState[planType].mutate(
			() => createInstance(apiFetchWithSession, page.params.practiceId!, page.params.engagementId!, planType),
			'Failed to create plan'
		);
		if (isStarted) openPlanForm(planType);
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
			'Failed to save plan'
		);
		if (!isSaved) return;
		shownPlanForms[planType] = false;
		planSavedMessage[planType] = `${heading} saved.`;
	}

	async function loadContractSection() {
		await contractState.load(
			() => loadContract(apiFetchWithSession, page.params.practiceId!, page.params.engagementId!),
			'Failed to load contract'
		);
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
			'Failed to load rates'
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
			'Failed to create contract'
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
			'Failed to save contract'
		);
		if (!isSaved) return;
		isContractFormOpen = false;
		contractSavedMessage = 'Contract saved.';
	}

	async function handleSendContract() {
		contractSavedMessage = '';
		const isSent = await contractState.mutate(
			() => sendContract(apiFetchWithSession, page.params.practiceId!, page.params.engagementId!),
			'Failed to send contract'
		);
		if (isSent) contractSavedMessage = 'Contract sent.';
	}

	/* The fee as this reader may see it (ADR-0008): the Practice's price
	   where the BFF sends it, or her own agreed fee (#2003). */
	const contractFeeRows = $derived<SummaryListRow[]>([
		...(contract?.values.price ? [{ label: 'Fee', value: contract.values.price }] : []),
		...(data.yourFee ? [{ label: 'Your fee', value: yourFeeValue(data.yourFee) }] : [])
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
			await invoicesState.load(
				() => loadInvoices(apiFetchWithSession, page.params.practiceId!, page.params.engagementId!),
				'Failed to load invoices'
			);
		} else {
			await contractorInvoicesState.load(
				() => loadContractorInvoices(apiFetchWithSession, page.params.practiceId!, page.params.engagementId!),
				'Failed to load invoices'
			);
		}
		// #271: read alongside the Invoice list itself -- InvoiceSection
		// needs to know the current mode (or its absence) before it can
		// decide whether to show the "ask once" form.
		await billingModeState.load(
			() => loadBillingMode(apiFetchWithSession, page.params.practiceId!),
			'Failed to load billing mode'
		);
	}

	// #1280: a Practice's zone (ADR-0036) does not move when a Payment is
	// recorded, voided, reversed, or refunded, so this is loaded once at
	// mount rather than inside loadInvoicesSection above -- which those
	// four writes re-run -- to avoid re-reading a fact none of them
	// changes.
	async function loadPracticeTimezoneSection() {
		await practiceTimezoneState.load(async () => {
			const timezone = await loadPracticeTimezone(apiFetchWithSession, page.params.practiceId!);
			return timezone.timezone;
		}, 'Failed to load Practice timezone');
	}

	// Reported by InvoiceSection's onCreate prop. No catch here, same as
	// the original: a refused create is left to the component the same way
	// Void Contract is (see above). #270 removed the old connectRequired
	// gate this used to route on -- InvoiceSection now decides whether to
	// show the form at all from canClientsPay, a standing fact read up
	// front, so a create attempt reaching this function is always the
	// happy path or a genuine error. #947 removed the amount this used to
	// pass on: an Invoice is raised for whatever the Contract carries.
	async function handleCreateInvoice(chosenBillingMode?: BillingMode) {
		const invoice = await createInvoice(
			apiFetchWithSession,
			page.params.practiceId!,
			page.params.engagementId!,
			chosenBillingMode
		);
		// #2002: a contractor's 201 names no amount (RaisedInvoice), so her
		// new row is read back in her own shape (#2094) rather than spliced.
		billingModeState.value = invoice.billingMode;
		if (canReadContractMoney) {
			invoicesState.value = [invoice, ...invoicesState.value];
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
		await loadInvoicesSection();
	}

	async function handleReversePayment(invoiceId: string, paymentId: string, reason: string) {
		await reversePayment(apiFetchWithSession, page.params.practiceId!, invoiceId, paymentId, reason);
		await loadInvoicesSection();
	}

	// #1009: reloads the section afterwards, as every other money write
	// here does -- the Invoice's refundedCents is the BFF's to sum.
	async function handleRefundPayment(invoiceId: string, paymentId: string, input: RefundPaymentInput) {
		await refundPayment(apiFetchWithSession, page.params.practiceId!, invoiceId, paymentId, input);
		await loadInvoicesSection();
	}

	async function handleVoidInvoice(invoiceId: string) {
		await voidInvoice(apiFetchWithSession, page.params.practiceId!, invoiceId);
		await loadInvoicesSection();
	}

	async function handleWriteOffInvoice(invoiceId: string) {
		await writeOffInvoice(apiFetchWithSession, page.params.practiceId!, invoiceId);
		await loadInvoicesSection();
	}

	// The roster read and the Offers read are both Owner/Admin; either
	// refusing is what tells this page the caller is a Doula, so the
	// section is left out rather than shown broken. The module answers
	// undefined for exactly that, which says "not for you" in the type
	// where this used to be a bare empty catch -- so this never throws in
	// practice, and offersState.error is never rendered (see its own
	// declaration above).
	async function loadOffersSection() {
		await offersState.load(() => loadOffers(apiFetchWithSession, reference, loadEngagementOffers), '');
	}

	async function loadDoulaFeesSection() {
		await doulaFeesState.load(
			() => loadDoulaFees(apiFetchWithSession, reference.practiceId, reference.engagementId),
			'We could not load the Doula fees.'
		);
	}

	async function handleRecordDoulaFee(attachmentId: string, input: RecordPaymentInput) {
		await recordDoulaFeePaid(apiFetchWithSession, reference.practiceId, reference.engagementId, attachmentId, input);
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

	function openOnCall(change?: OnCallChange) {
		openOnCallChange = change;
		isGapOpenerRefocused = false;
		onCallDaysError = '';
		onCallSavedMessage = '';
		gapError = '';
	}

	function toggleDaysChange(staffId: string) {
		if (isDaysChangeOpen(staffId)) openOnCall();
		else openOnCall({ staffId });
	}

	type OnCallDoulaRow = SummaryListRow & { staffId: string };
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
	   holds -- which is a local wall-clock time with no zone on it, so it
	   is read in the reader's own zone rather than parsed as UTC. An empty
	   control (or a half-typed one, which reports '') is `undefined`, never
	   `new Date('')`, whose `toISOString()` throws "Invalid time value"
	   (#1969). */
	function toInstant(local: string): Date | undefined {
		const instant = new Date(local);
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

				await saveGap(apiFetchWithSession, reference.practiceId, reference.engagementId, {
					staffId: gapStaffId,
					startsAt: startsAt.toISOString(),
					endsAt: endsAt.toISOString(),
					reason: gapReason === '' ? undefined : gapReason,
					coveringStaffId: gapCoveringStaffId === '' ? undefined : gapCoveringStaffId
				});
				const name = onCall?.doulas.find((doula) => doula.staffId === gapStaffId)?.name ?? 'the Doula';
				closeGapForm();
				await loadOnCall();
				onCallSavedMessage = `The time ${name} cannot be reached is recorded.`;
			},
			async (refusal) => inFieldOrder(await orThrownErrors(gapFieldIds)(refusal))
		);
	}

	/* Closes the gap form blank, so it opens with nobody chosen next time
	   (#1916), and drops whatever it last refused. */
	function closeGapForm() {
		gapStaffId = '';
		gapStartsAt = '';
		gapEndsAt = '';
		gapReason = '';
		gapCoveringStaffId = '';
		gapSubmission.errors = [];
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
	const onCallDoulaRows = $derived(
		(onCall?.doulas ?? []).map(
			(doula): OnCallDoulaRow => ({
				staffId: doula.staffId,
				label: doula.name,
				value:
					doula.onCallFrom && doula.onCallTo
						? `On call ${formatCalendarDay(doula.onCallFrom)} to ${formatCalendarDay(doula.onCallTo)}`
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

	const gapRows = $derived(
		(onCall?.gaps ?? []).map(
			(gap): GapRow => ({
				gap,
				label: gap.staffName,
				value: `${formatInstant(gap.startsAt)} to ${formatInstant(gap.endsAt)}`,
				changes: `the gap for ${gap.staffName}`
			})
		)
	);

	async function loadRoster() {
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
		const updated = await loadEngagementOffers(apiFetchWithSession, page.params.practiceId!, page.params.engagementId!);
		if (offersState.value) offersState.value = updated;
	}

	async function handleWithdrawOffer(offerId: string) {
		await withdrawOffer(apiFetchWithSession, page.params.practiceId!, offerId);
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

	async function loadClientRecord() {
		try {
			clientRecord = await loadClientDetail(apiFetchWithSession, page.params.practiceId!, detail.clientId);
		} catch {
			// The link stays off; see above.
		}
	}

	onMount(async () => {
		void loadClientRecord();
		// The Engagement is already here, from load. What remains is the
		// seven sections that fill in behind it, each rendering as it lands.
		await loadVisits();
		await loadMessages();
		await Promise.all(planSections.map((section) => loadPlan(section.type)));
		await loadContractSection();
		await loadInvoicesSection();
		await loadPracticeTimezoneSection();
		await loadRoster();
		await loadOnCall();
		await loadOffersSection();
		await loadDoulaFeesSection();
		await loadActivity();

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
		await portalInvite.mutate(async () => {
			const created = await sendPortalInvite(apiFetchWithSession, reference);
			// #255: a fresh invite is always pending, never suppressed --
			// lifts the Contract section's block and hides this action's
			// own visibility question ("never invited") right away.
			clientPortalOverride = { portalInviteStatus: 'pending', emailSuppressed: false };
			return `${location.origin}/portal/accept-invite?token=${created.inviteToken}`;
		}, 'Failed to send portal invite');
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
		const scheduledAt = newVisitScheduledAt ? new Date(newVisitScheduledAt).toISOString() : undefined;
		const staffId = visitStaffId || undefined;
		if (
			await visitsCreate.mutate(
				() => createVisit(apiFetchWithSession, reference, scheduledAt, staffId),
				'Failed to add Visit'
			)
		) {
			closeAddVisit();
			await loadVisits();
			visitsSavedMessage =
				staffId && staffId !== callerStaffId ? `The Visit for ${staffName(staffId)} is added.` : 'Your Visit is added.';
			await refreshStatusAfterScheduling(scheduledAt);
			await refreshWhoIsOn();
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
		const scheduledAt = changeScheduledAt ? new Date(changeScheduledAt).toISOString() : undefined;
		const isRescheduled = changeScheduledAt !== toDatetimeLocalValue(visit.scheduledAt);
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
		}, 'Failed to save the Visit');
		if (wasSaved) openVisitForm();
		if (!hasWritten) return;
		await loadVisits();
		if (wasSaved) visitsSavedMessage = `The Visit for ${visit.staffName} is saved.`;
		if (isRescheduled) await refreshStatusAfterScheduling(scheduledAt);
	}

	async function handleReassignVisit(event: SubmitEvent, visit: Visit) {
		event.preventDefault();
		hasAttemptedReassign = true;
		const newStaffId = reassignStaffId;
		if (!newStaffId || reassignBlock) return;
		const wasSaved = await visitReassign.mutate(
			() => reassignVisit(apiFetchWithSession, reference, visit.visitId, newStaffId),
			'Failed to reassign the Visit'
		);
		if (!wasSaved) return;
		openVisitForm();
		await loadVisits();
		const which = visit.scheduledAt
			? `The Visit on ${formatScheduledVisit(visit.scheduledAt)}`
			: 'The Visit that is not yet scheduled';
		visitsSavedMessage = `${which} is ${staffName(newStaffId)}'s now, not ${visit.staffName}'s.`;
		await refreshWhoIsOn();
		await loadActivity();
	}

	// Reuses loadMessagesPage rather than re-fetching by hand: the query
	// string and the newest-first-to-oldest-first reversal are exactly the
	// same read, just prepended instead of replacing.
	async function handleLoadOlderMessages() {
		await messagesLoad.mutate(async () => {
			const older = await loadMessagesPage<Message>(apiFetchWithSession, reference, messagesCursor);
			messages = [...older.items, ...messages];
			messagesCursor = older.nextCursor ?? '';
			isMessagesHasMore = older.hasMore;
			await refreshAttachmentPreviews(messages);
		}, 'Failed to load older messages');
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
		}, 'Failed to download attachment');
	}
</script>

{#snippet summary()}
	<!-- #2308: the Birth outcome section links here to reopen a completed
	     Engagement before its outcome can be removed. -->
	<stack-l id="engagement-summary" space="var(--space-4)">
		<!--
			#1998: GOV.UK's Summary list with row actions, as the drawing
			has it -- one bordered card, a Change at the end of each row
			this reader may change. Change is a button, not a link, because
			it opens a form in place rather than going anywhere; it says
			whether that form is open (aria-expanded), and the visually-
			hidden words SummaryList draws name the row it changes.
		-->
		<SummaryList rows={summaryRows} isBordered>
			{#snippet action(row, describedBy)}
				<Button
					label="Change"
					variant="link"
					expanded={openChange === row.change}
					{describedBy}
					onClick={() => toggleChange(row.change!)}
				/>
			{/snippet}
			{#snippet detail(row)}
				{#if row.change === 'care'}
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
				{:else if row.change === 'due'}
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
								onClick={() => (openChange = undefined)}
							/>
						</cluster-l>
					</StackedForm>
				{:else if row.change === 'carePhase'}
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
									legend="Why is this Engagement ending?"
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
				{:else if isAttachQuestion}
					<!--
						#2230: which Doula, and the save. No Cancel: the row's
						Change closes it, as it closes the care change above.
					-->
					<StackedForm onSubmit={handleAttachSubmit}>
						{#if attachSubmission.errors.length > 0}
							<ErrorSummary errors={attachSubmission.errors} />
						{/if}
						<RadioGroup
							legend="Who is the Doula?"
							hint={hasUnlistedContractor
								? 'A contractor is not listed: a contractor goes on an Engagement by accepting an Offer.'
								: undefined}
							name={ATTACH_GROUP}
							options={attachOptions}
							value={attachChoice}
							onChange={(value) => (attachChoice = value)}
							error={attachSubmission.errorFor(radioFieldId(ATTACH_GROUP, attachOptions[0]!.value))}
						/>
						<cluster-l space="var(--space-3)">
							<Button
								label="Save the Doula"
								type="submit"
								size="sm"
								loading={attachSubmission.isSubmitting}
							/>
						</cluster-l>
					</StackedForm>
				{:else}
					<!--
						#1598: "Put me on this Engagement", behind the Doula row's
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
								label="Put me on this Engagement"
								size="sm"
								variant="secondary"
								loading={selfAttach.isBusy}
								onClick={handleAttachSelf}
							/>
						</cluster-l>
						{#if selfAttach.error}
							<Notice variant="error" message={selfAttach.error} />
						{/if}
					</stack-l>
				{/if}
			{/snippet}
		</SummaryList>

		<!--
			What a Change did. Each takes focus: the control she pressed
			went with the form it was in, and focus would otherwise fall to
			the document (seen on #1598's 320px walk). The row above already
			shows the new value.
		-->
		{#if selfAttachedMessage}
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
		<span class="visually-hidden" id="engagement-client-name">{detail!.clientName}</span>
		<cluster-l space="var(--space-4)" align="center">
			<Link href={clientDetailHref()} label="View Client" describedBy="engagement-client-name" />
			<!--
				Her other details, added from here as well as from her record
				(#1610). The journey's first Back and its save return to this
				page. Absent until her record is read, and for an erased record,
				whose edit `edit.go` refuses.
			-->
			{#if clientRecord && canAddDetails(clientRecord)}
				<Link
					href={detailsHref(page.params.practiceId!, detail!.clientId, page.params.engagementId!)}
					label={detailsLinkLabel(clientRecord)}
				/>
			{/if}
		</cluster-l>

		<!--
			The outcome of the header's own action, and the only block-level
			room in the header block: `actions` is a cluster beside the h1,
			where an error banner and a full invite URL cannot go. Absorbed
			into the Template's existing regions rather than taken through one
			of ADR-0018's exits.
		-->
		{#if portalInviteError}
			<Notice variant="error" message={portalInviteError} />
		{/if}

		{#if portalInviteLink}
			<Text text={`Invited. ${detail!.clientName} has an email with the link. To share the link yourself, here it is:`} />
			<div><code>{portalInviteLink}</code></div>
		{/if}
	</stack-l>
{/snippet}

{#snippet actions()}
	<!--
		#1998: the drawing's header actions. "Message the Client" goes to
		this page's own Messages section, which every reader of the page
		has. Both are secondary, as drawn: the header names what can be
		done, and the page's one accent is spent lower down.
	-->
	<Link href="#messages" label="Message the Client" variant="secondary" />
	<!--
		#255: an accepted Client is never offered a second invite -- the
		"Portal invite" summary row above already says "Accepted", so
		hiding this action here states the same fact rather than a second,
		clickable copy of it. A Client with no email gets a Notice naming
		the missing thing in place of the control (#270 converted this from
		a disabled button with a visually-hidden hint -- the same pattern
		#275 used at InvoiceSection.svelte:109 -- block over warn, never a
		control silently withheld with only a hidden explanation).
	-->
	{#if !hasAcceptedPortalAccess}
		{#if hasClientEmailOnFile}
			<Button
				label="Send portal invite"
				variant="secondary"
				onClick={handleSendPortalInvite}
				loading={isSendingPortalInvite}
			/>
		{:else}
			<Notice variant="info" message={noEmailForInviteMessage} />
		{/if}
	{/if}
{/snippet}

{#snippet visitActions(visit: Visit, view: DataTableView)}
	<!--
		#1999: a row holds its facts and the one action that opens its
		change, never the form itself. #666: DataTable renders this snippet
		into both of its trees, so the name span's id is scoped by which
		tree is asking.
	-->
	<span class="visually-hidden" id="{view}-visit-{visit.visitId}-name"
		>{visit.staffName}, {formatScheduledVisit(visit.scheduledAt)}</span
	>
	<cluster-l space="var(--space-3)">
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
		<Heading level={3} text="Add a Visit" focusOnAppearing />
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

{#snippet visitHeading(act: string, visit: Visit)}
	{#key visit.visitId}
		<Heading
			level={3}
			text={visit.scheduledAt
				? `${act} the Visit for ${visit.staffName} on ${formatScheduledVisit(visit.scheduledAt)}`
				: `${act} the Visit for ${visit.staffName}, not yet scheduled`}
			focusOnAppearing
		/>
	{/key}
{/snippet}

{#snippet reassignVisitForm(visit: Visit)}
	{@const reassignFieldId = `reassign-visit-staff-${visit.visitId}`}
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

{#snippet changeVisitForm(visit: Visit)}
	<StackedForm onSubmit={(event) => handleChangeVisit(event, visit)}>
		{@render visitHeading('Change', visit)}
		{#if visitChange.error}
			<Notice variant="error" message={visitChange.error} />
		{/if}
		<!-- #250: an empty value clears the schedule; emptying the control
		     is the plainer way to ask for that than a separate Clear. -->
		<LabeledField
			id="change-visit-scheduled-at-{visit.visitId}"
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
		<LabeledField id="change-visit-notes-{visit.visitId}" label="Notes">
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
		<cluster-l space="var(--space-3)">
			<Button label="Save changes" type="submit" loading={visitChange.isBusy} />
			<Button label="Cancel" variant="secondary" onClick={() => cancelVisitChange(visit)} />
		</cluster-l>
	</StackedForm>
{/snippet}

{#snippet visitsSection()}
	{#if rosterState.error}
		<Notice variant="error" message={rosterState.error} />
	{/if}

	{#if visitsLoad.error}
		<Notice variant="error" message={visitsLoad.error} />
	{/if}

	<!--
		#268/#274: adding is offered to a reader who can actually complete
		it -- an Owner or Admin, who picks the colleague it is for, or a
		Doula, who logs her own. A reader who is neither is offered no
		control rather than a button that 403s. #1999: one secondary
		opener, as the drawing has its actions under the table; the form,
		and its one primary, only when asked.
	-->
	{@const canAddVisit = canAssignVisits || canLogOwnVisit}
	{@const addOpener =
		canAddVisit && openVisitChange !== 'add'
			? {
					label: 'Add a Visit',
					onClick: () => openVisitForm('add'),
					focusOnAppearing: refocusedVisitOpener === 'add'
				}
			: undefined}
	<DataTable
		columns={[
			{ label: 'Date', accessor: (visit: Visit) => formatScheduledVisit(visit.scheduledAt) },
			{ label: 'Type', accessor: (visit: Visit) => visitTypeLabel(visit.type) },
			{ label: 'Doula', accessor: (visit: Visit) => visit.staffName },
			// #2091: a note is a Doula's prose beside four facts, so it takes
			// the room they leave and wraps there. The table's floor is then the
			// facts beside the note's longest word, 615px on the typical screen in
			// macOS/Chromium, under DataTable's own 55rem, so this route sets no
			// floor of its own.
			{ label: 'Notes', accessor: (visit: Visit) => visit.notes || 'No notes yet.', wraps: true }
		]}
		rows={sortedVisits}
		rowActions={{ label: 'Actions', content: visitActions }}
		hasMore={visits.hasMore}
		onLoadMore={() => visits.loadMore()}
		isLoadingMore={visits.isLoadingMore}
		loadMoreError={visits.loadMoreError}
		emptyMessage="No Visits yet."
		emptyAction={addOpener}
	/>

	{#if visitsSavedMessage}
		<Notice variant="status" message={visitsSavedMessage} isFocusedOnAppear />
	{/if}

	{#if addOpener && sortedVisits.length > 0}
		<div>
			<Button
				label={addOpener.label}
				variant="secondary"
				onClick={addOpener.onClick}
				focusOnAppearing={addOpener.focusOnAppearing}
			/>
		</div>
	{/if}

	{#if openVisitChange === 'add' && canAddVisit}
		{@render addVisitForm()}
	{:else if changingVisit}
		{@render changeVisitForm(changingVisit)}
	{:else if reassigningVisit && canAssignVisits}
		{@render reassignVisitForm(reassigningVisit)}
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
		{#if instance && shownPlanForms[planType]}
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
				<SummaryList
					rows={[
						{ label: 'Progress', value: planProgress(instance.fields, instance.answers) },
						// One audit row per Plan, as drawn: who last saved it (#2104), or,
						// until a first save, who started it (#2133).
						...planTrailRows(instance),
						...(planType === 'birth_plan' ? [{ label: 'Client', value: birthPlanReadText(instance) }] : [])
					]}
				/>
				{#if planSavedMessage[planType]}
					<Notice variant="status" message={planSavedMessage[planType]} isFocusedOnAppear />
				{/if}
				<cluster-l space="var(--space-3)">
					<Button
						label="Change the {heading}"
						variant="secondary"
						onClick={() => openPlanForm(planType)}
						focusOnAppearing={refocusedPlanOpener === planType}
					/>
					<!--
						#280: the reading and taking-away surface -- its own
						address. Editing stays here.
					-->
					{#if planType === 'birth_plan'}
						<Link href={birthPlanHref} label="Print for the hospital" icon="file-text" variant="secondary" />
					{/if}
				</cluster-l>
				<!-- #2243: the answers read here; changing them stays its own act. -->
				<PlanInstanceView fields={instance.fields} answers={instance.answers} headingLevel={3} />
			{:else}
				<EmptyState
					message="The {heading} is not started yet."
					action={{ label: `Start the ${heading}`, onClick: () => handleStartPlan(planType) }}
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
		{#if contract && isContractFormOpen}
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
				status={contract.status}
				hasSignedPdf={contract.hasSignedPdf}
				amountChangedAt={contract.amountChangedAt}
				voidRequests={contract.voidRequests}
				sent={contract.sent}
				signed={contract.signed}
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
				<Details summary="Read the Contract text">
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
					{:else if hasNeverInvitedClient}
						<Notice variant="info" message={inviteBeforeContractMessage} />
					{/if}
					<cluster-l space="var(--space-3)">
						{#if missingMergeFields.length === 0 && !hasNeverInvitedClient}
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
				{#if hasRateForKind}
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
				<SummaryList rows={[{ label: 'Your fee', value: yourFeeValue(data.yourFee) }]} />
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
				{#if !hasClientEmailOnFile}
					<Notice variant="info" message={noEmailForInviteMessage} />
				{/if}
				{#if hasClientEmailOnFile || hasRateForKind}
					<cluster-l space="var(--space-3)">
						{#if hasClientEmailOnFile}
							<Button
								label="Send portal invite"
								variant="secondary"
								onClick={handleSendPortalInvite}
								loading={isSendingPortalInvite}
							/>
						{/if}
						{#if hasRateForKind}
							<Button label="Create a Draft Contract" variant="secondary" onClick={handleCreateContract} />
						{/if}
					</cluster-l>
				{/if}
			{:else}
				<EmptyState
					message="No Contract yet."
					action={hasRateForKind ? { label: 'Create a Draft Contract', onClick: handleCreateContract } : undefined}
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
	{:else if contract}
		{@render invoicesBody(contract.status)}
	{:else}
		<EmptyState message="An Invoice is raised against a signed Contract." />
	{/if}
{/snippet}

{#snippet invoicesBody(contractStatus: string)}
	{#if invoicesError}
		<Notice variant="error" message={invoicesError} />
	{/if}
	{#if billingModeState.error}
		<Notice variant="error" message={billingModeState.error} />
	{/if}
	{#if practiceTimezoneState.error}
		<Notice variant="error" message={practiceTimezoneState.error} />
	{/if}

	<!-- #1280: InvoiceSection's own todayIsoDate reads "today" in
	     practiceTimezone with no UTC fallback (ADR-0036's own rule for a
	     zone that will not load), so it waits here for a real zone name
	     rather than mounting with '' and throwing on its own first date
	     computation. A still-loading or failed read leaves this gap
	     blank; the Notice above already says why. -->
	{#if practiceTimezone}
		<InvoiceSection
			{invoices}
			{contractStatus}
			{billingMode}
			clientsCanPay={canClientsPay}
			hasClientEmail={hasClientEmailOnFile}
			isOwner={isPracticeOwner}
			isOwnerOrAdmin={isPracticeOwnerOrAdmin}
			canReadInvoiceMoney={canReadContractMoney}
			contractorInvoices={contractorInvoicesState.value}
			{practiceTimezone}
			{paymentsSettingsHref}
			{landOnInvoiceId}
			onCreate={handleCreateInvoice}
			onRecordPayment={handleRecordPayment}
			onReversePayment={handleReversePayment}
			onRefundPayment={handleRefundPayment}
			onVoidInvoice={handleVoidInvoice}
			onWriteOffInvoice={handleWriteOffInvoice}
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
	<OfferSection
		{offers}
		doulas={offerDoulas}
		clientName={detail!.clientName}
		dueDate={displayDueDate ?? ''}
		onCreate={handleCreateOffer}
		onWithdraw={handleWithdrawOffer}
	/>
{/snippet}

{#snippet doulaFeesSection()}
	{#if doulaFeesState.error}
		<Notice variant="error" message={doulaFeesState.error} />
	{/if}
	<!-- The date paid reads "today" in the Practice's zone, so the section
	     waits for it, as InvoiceSection does. -->
	{#if practiceTimezone}
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
	{#if messageThreadRows.length > 0}
		<SummaryList rows={messageThreadRows} />
	{/if}
	<MessageThread
		{messages}
		error={messagesError}
		hasMore={isMessagesHasMore}
		isLoadingOlder={isLoadingOlderMessages}
		onLoadOlder={handleLoadOlderMessages}
		onSend={sendThreadMessage}
		onDownloadAttachment={handleDownloadAttachment}
		{attachmentPreviewURLs}
		composeOpener="Write a message"
		cannotSendReason={messageBlock}
	/>
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
{#snippet onCallSection()}
	{#if onCallError}
		<Notice variant="error" message={onCallError} />
	{:else if onCall}
		{#if onCall.window}
			<Text
				text={`On call from ${formatCalendarDay(onCall.window.start)} to ${formatCalendarDay(onCall.window.end)}.`}
			/>
		{:else}
			<Text text={describeNoWindow(onCall.noWindowReason ?? '')} tone="muted" />
		{/if}

		<!--
			#2042: GOV.UK's Summary list with row actions, the record-page
			pattern (govuk-alignment.md, "Sections of a record page"). Each
			Doula's days are a row; an Owner's or an Admin's Change opens the
			two dates in place under it, because the first and the last day
			belong together on one form.
		-->
		{#if onCallDoulaRows.length > 0}
			<SummaryList rows={onCallDoulaRows}>
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

		{#if onCallSavedMessage}
			<Notice variant="status" message={onCallSavedMessage} isFocusedOnAppear />
		{/if}

		<Heading level={3} text="When a doula cannot be reached" />
		{#if gapError}
			<Notice variant="error" message={gapError} />
		{/if}
		{@const canRecordGap = onCall.window !== undefined && onCall.doulas.length > 0}
		{@const isGapFormOpen = openOnCallChange === 'gap'}
		<!-- One secondary opener; the form, and its one primary, only when asked. -->
		{#if onCall.gaps.length === 0}
			<EmptyState
				message="No times recorded yet."
				action={canRecordGap && !isGapFormOpen
					? {
							label: 'Record a time a doula cannot be reached',
							onClick: () => openOnCall('gap'),
							focusOnAppearing: isGapOpenerRefocused
						}
					: undefined}
			/>
		{:else}
			<!-- Clear is secondary, and small: it ends a gap rather than
			     destroying a record, and the row's own words say whose. -->
			<SummaryList rows={gapRows}>
				{#snippet value(row)}
					<stack-l space="var(--space-2)">
						<span>{row.value}</span>
						<div>
							<Badge
								label={row.gap.coveringStaffName ? `${row.gap.coveringStaffName} covering` : 'Nobody covering'}
								variant={row.gap.coveringStaffName ? 'success' : 'warning'}
							/>
						</div>
						{#if row.gap.reason}
							<Text text={row.gap.reason} step="body-sm" tone="variant" />
						{/if}
					</stack-l>
				{/snippet}
				{#snippet action(row, describedBy)}
					<Button
						label="Clear"
						variant="secondary"
						size="sm"
						{describedBy}
						onClick={() => clearCoverageGap(row.gap)}
					/>
				{/snippet}
			</SummaryList>
			{#if canRecordGap && !isGapFormOpen}
				<div>
					<Button
						label="Record a time a doula cannot be reached"
						variant="secondary"
						onClick={() => openOnCall('gap')}
						focusOnAppearing={isGapOpenerRefocused}
					/>
				</div>
			{/if}
		{/if}

		{#if canRecordGap && isGapFormOpen}
			{@const panel = onCall}
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
							options={panel.doulas.map((doula) => ({ value: doula.staffId, label: doula.name }))}
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
				<LabeledField
					id={gapFieldIds.coveringStaffId}
					label="Who is covering"
					hint="Leave this unanswered where nobody is covering yet. Every owner and admin is told so they can arrange it."
					error={gapSubmission.errorFor(gapFieldIds.coveringStaffId)}
				>
					{#snippet children({ id, describedBy, invalid })}
						<Select
							{id}
							{describedBy}
							{invalid}
							options={[
								{ value: '', label: 'Nobody yet' },
								...panel.doulas
									.filter((doula) => doula.staffId !== gapStaffId)
									.map((doula) => ({ value: doula.staffId, label: doula.name }))
							]}
							bind:value={gapCoveringStaffId}
						/>
					{/snippet}
				</LabeledField>
				<LabeledField
					id={gapFieldIds.reason}
					label="Reason"
					hint="Optional, and read only by this Practice."
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
					<Button type="submit" label="Record the gap" loading={gapSubmission.isSubmitting} />
					<Button label="Cancel" variant="secondary" onClick={cancelGapForm} />
				</cluster-l>
			</StackedForm>
		{/if}
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
		reopenHref={displayStatus === 'completed' ? '#engagement-summary' : undefined}
		onRecord={handleRecordBirthOutcome}
	/>
{/snippet}

{#snippet activitySection()}
	{#if activityError}
		<Notice variant="error" message={activityError} />
	{/if}
	<DataTable
		columns={activityLedgerColumns()}
		rows={shownActivity}
		hasMore={hasMoreActivity}
		onLoadMore={showMoreActivity}
		isLoadingMore={activity.isLoadingMore}
		loadMoreError={activity.loadMoreError}
		emptyMessage="Nothing has happened yet."
	/>
{/snippet}

<!--
	Archetype D, ADR-0018. Offers is Owner/Admin-only, which is why
	`sections` is a typed array and not a run of named regions.

	The order (#2146) is the order a Doula works in on an open Engagement:
	who is on call and how the birth went first, because during a birth
	those are what she opens the page for (#1093, #943). Then the
	Contract, before the Visits, because a new Engagement waits on the
	signed agreement and the money that follows it -- the drawing's reason
	for putting Contract first. Then the care she gives in the order she
	gives it (Visits, Care Plan, Birth Plan), the money raised against the
	Contract (Invoices), who else was offered the work (Offers), and the
	conversation and the record of what happened last. The "On this page"
	rail and the "Jump to" strip are derived from this array, so they
	follow it. Every section shows even when empty (#2015): Invoices
	before a Contract says what it waits for.
-->
<!-- #1611, #1710: a save that leads here says what it did, such as
     "Work with Pat started." at the end of a new Client's flow. -->
<RecordDetail
	title={detail.clientDisplayName}
	{outcome}
	backHref={clientsHref}
	backLabel="Clients"
	{summary}
	{actions}
	isContentsShown
	sections={[
		{ heading: 'On call', content: onCallSection },
		{ heading: 'Birth outcome', content: birthOutcomeSection },
		{ heading: 'Contract', content: contractSection },
		{ heading: 'Visits', content: visitsSection },
		{ heading: 'Care Plan', content: carePlanSection },
		{ heading: 'Birth Plan', content: birthPlanSection },
		{ heading: 'Invoices', content: invoicesSection },
		...(isOffersVisible ? [{ heading: 'Offers', content: offersSection }] : []),
		...(isDoulaFeesVisible ? [{ heading: 'Doula fees', content: doulaFeesSection }] : []),
		{ heading: 'Messages', content: messagesSection },
		{ heading: 'Activity', content: activitySection }
	]}
/>
