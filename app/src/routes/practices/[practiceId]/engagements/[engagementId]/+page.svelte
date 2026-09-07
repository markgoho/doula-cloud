<script lang="ts">
	import { onDestroy, onMount } from 'svelte';
	import { page } from '#lib/appState.svelte.js';
	import { apiFetchWithSession } from '#lib/api.js';
	import { PaginatedList } from '#lib/paginatedList.svelte.js';
	import { SectionState } from '#lib/sectionState.svelte.js';
	import { triggerBlobDownload } from '#lib/blobDownload.js';
	import {
		changeEngagementStatus,
		createVisit,
		downloadAttachment,
		endingReasons,
		loadAttachmentPreviews,
		loadMessagesPage,
		loadOffersSection as loadOffers,
		loadVisitsPage,
		reassignVisit,
		saveVisitNotes,
		scheduleVisit,
		sendMessage,
		sendPortalInvite,
		type OffersSection,
		type Visit
	} from '#lib/engagementDetail.js';
	import type { PageProps as PageProperties } from './$types';
	import { formatCalendarDay, formatInstant, formatScheduledVisit, toDatetimeLocalValue } from '#lib/dates.js';
	import { activityLedgerColumns, loadEngagementActivityPage, type ActivityEntry } from '#lib/activityLedger.js';
	import { subscribeToThreadPushMessages } from '#lib/pushRefresh.js';
	import PlanInstanceForm from '#lib/components/organisms/PlanInstanceForm.svelte';
	import {
		loadInstance,
		createInstance,
		saveAnswers,
		downloadBirthPlanPdf,
		setAnswer,
		toggleMultiSelectOption,
		type Instance
	} from '#lib/planInstance.js';
	import ContractForm from '#lib/components/molecules/ContractForm.svelte';
	import ContractStatus from '#lib/components/molecules/ContractStatus.svelte';
	import {
		loadContract,
		createContract,
		saveContractValues,
		sendContract,
		voidContract,
		downloadSignedContractPdf,
		setMergeFieldValue,
		type Contract
	} from '#lib/contract.js';
	import { isOwnerOrAdmin } from '#lib/roles.js';
	import InvoiceSection from '#lib/components/organisms/InvoiceSection.svelte';
	import { loadInvoices, createInvoice, type Invoice } from '#lib/invoice.js';
	import OfferSection from '#lib/components/organisms/OfferSection.svelte';
	import { createOffer, loadEngagementOffers, withdrawOffer, type NewOffer, type Offer } from '#lib/offer.js';
	import { connect as connectStripe } from '#lib/payments.js';
	import MessageThread, { type Message } from '#lib/components/organisms/MessageThread.svelte';
	import Text from '#lib/components/atoms/Text.svelte';
	import Link from '#lib/components/atoms/Link.svelte';
	import Button from '#lib/components/atoms/Button.svelte';
	import Notice from '#lib/components/atoms/Notice.svelte';
	import DescriptionList from '#lib/components/molecules/DescriptionList.svelte';
	import RadioGroup from '#lib/components/molecules/RadioGroup.svelte';
	import DataTable from '#lib/components/organisms/DataTable.svelte';
	import RecordDetail from '#lib/components/templates/RecordDetail.svelte';
	import TextInput from '#lib/components/atoms/TextInput.svelte';
	import Textarea from '#lib/components/atoms/Textarea.svelte';
	import LabeledField from '#lib/components/molecules/LabeledField.svelte';

	type Detail = {
		engagementId: string;
		clientId: string;
		clientName: string;
		status: string;
		createdAt: string;
		dueDate?: string;
		statusMoves: string[];
	};

	// The Engagement comes from +page.ts's load now, not an onMount fetch
	// (#695): a refusal has to reach practices/+error.svelte rather than
	// sit in a local error string, the same move #471 made for billing.
	// The other six sections still load after mount, each rendering as it
	// lands.
	let { data }: PageProperties = $props();
	const detail = $derived(data);

	// The Contract's PDF download is Owner/Admin only (ADR-0008's money
	// row, matching the endpoint's own OwnerAndAdmin gate) -- this drawing
	// decision is not the gate: contract.ts's downloadSignedContractPdf
	// hits the real endpoint, which refuses any other role on its own.
	const isPracticeOwnerOrAdmin = $derived(isOwnerOrAdmin(data.session));

	// The reference every read on this page is about. Derived rather than
	// captured, so a client-side navigation to a sibling Engagement is
	// picked up by the reads below.
	const reference = $derived({
		practiceId: page.params.practiceId!,
		engagementId: page.params.engagementId!
	});

	// #253: the Engagement's status and its next legal moves, overlaid on
	// the load-time read once a move succeeds -- the same "server answer
	// replaces the derived initial value" shape SectionState gives every
	// other in-page mutation, so the page reflects a successful move
	// without a reload. `value` starts undefined (no move made yet), in
	// which case `displayStatus`/`displayStatusMoves` below fall back to
	// `detail` itself.
	const statusChange = new SectionState<{ status: string; statusMoves: string[] } | undefined>(undefined);
	const displayStatus = $derived(statusChange.value?.status ?? detail?.status ?? '');
	const displayStatusMoves = $derived(statusChange.value?.statusMoves ?? detail?.statusMoves ?? []);

	// Completing asks for a reason first (GOV.UK's question-page pattern,
	// ADR-0021) rather than submitting the bare move -- the other three
	// moves need nothing more than the click that requests them.
	let isCompleteFormShown = $state(false);
	let completeReasonValue = $state('');
	let completeNoteValue = $state('');
	let completeReasonError = $state('');

	async function moveStatus(target: string, endingReason?: string, endingNote?: string): Promise<void> {
		await statusChange.mutate(
			() => changeEngagementStatus(apiFetchWithSession, reference, target, endingReason, endingNote),
			'Failed to change status'
		);
	}

	function handleStatusMoveClick(move: string) {
		if (move === 'completed') {
			isCompleteFormShown = true;
			return;
		}
		isCompleteFormShown = false;
		void moveStatus(move);
	}

	async function handleCompleteSubmit(event: SubmitEvent) {
		event.preventDefault();
		if (!completeReasonValue) {
			completeReasonError = 'Select why this Engagement is ending';
			return;
		}
		completeReasonError = '';
		await moveStatus('completed', completeReasonValue, completeNoteValue || undefined);
		if (!statusChange.error) {
			isCompleteFormShown = false;
			completeReasonValue = '';
			completeNoteValue = '';
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

	// Visits are newest-first from the BFF (#446); a further page is
	// appended to the end of what is already on screen rather than
	// reversed, since this is a table read top-to-bottom, not a chat
	// thread. Loading the first page and adding a Visit are separate
	// SectionState instances (#841) so neither action's busy flag lights
	// up the other's control; both write the same Notice, so their errors
	// are read together below.
	const visits = new PaginatedList<Visit>({
		first: { items: [], hasMore: false },
		loadPage: (cursor) => loadVisitsPage(apiFetchWithSession, reference, cursor),
		failureMessage: 'Failed to load more Visits'
	});
	const visitsLoad = new SectionState<void>(undefined);
	const visitsCreate = new SectionState<void>(undefined);
	const visitsError = $derived(visitsLoad.error || visitsCreate.error);
	const isCreatingVisit = $derived(visitsCreate.isBusy);
	// #250: optional at creation, the same as it is afterward -- a Doula
	// logging a meeting that already happened leaves this blank.
	let newVisitScheduledAt = $state('');

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

	const portalInvite = new SectionState('');
	const portalInviteLink = $derived(portalInvite.value);
	const portalInviteError = $derived(portalInvite.error);
	const isSendingPortalInvite = $derived(portalInvite.isBusy);

	let reassignStaffId = $state<Record<string, string>>({});
	// One SectionState per Visit row, created the first time that row is
	// reassigned (#841) -- reassignError was a Record before this too, but
	// a per-section instance is the wrong shape for it: two rows reassigned
	// at once must not share one error or one busy flag, which is exactly
	// what a single SectionState would do.
	let reassignSections = $state<Record<string, SectionState<void>>>({});

	// #250: a per-row schedule control, the same per-row-state shape
	// reassign already established just above. scheduleValue holds a
	// draft only once a row's own field has been touched -- until then
	// the field's own value falls back to the Visit's current
	// scheduledAt (see scheduleAction below), so editing one row never
	// disturbs another's.
	let scheduleValue = $state<Record<string, string>>({});
	let scheduleSections = $state<Record<string, SectionState<void>>>({});

	// #251: a per-row notes control, the same per-row-state shape schedule
	// and reassign already established just above. notesValue holds a
	// draft only once a row's own field has been touched -- until then the
	// field's own value falls back to the Visit's current notes (see
	// visitActions below), so editing one row never disturbs another's.
	let notesValue = $state<Record<string, string>>({});
	let notesSections = $state<Record<string, SectionState<void>>>({});

	let messages = $state<Message[]>([]);
	let messagesCursor = $state('');
	let isMessagesHasMore = $state(false);
	// Load (including "load older") and send are separate SectionState
	// instances so sending a Message never lights up the "Load older"
	// spinner and vice versa; downloading an attachment is a third, so it
	// never lights up either. All three still write the one Notice the
	// thread renders.
	const messagesLoad = new SectionState<void>(undefined);
	const messagesSend = new SectionState<void>(undefined);
	const messagesDownload = new SectionState<void>(undefined);
	const messagesError = $derived(messagesLoad.error || messagesSend.error || messagesDownload.error);
	const isLoadingOlderMessages = $derived(messagesLoad.isBusy);
	const isSendingMessage = $derived(messagesSend.isBusy);
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
	// #306: Birth Plan only, not Care Plan -- the Client-facing side of
	// this same download has no Care Plan page to mirror yet, and the
	// ticket's own decision scoped the PDF to Birth Plan alone.
	let isDownloadingBirthPlanPdf = $state(false);
	let downloadBirthPlanPdfError = $state('');

	// Named contractState, not contractSection: that name is already the
	// RecordDetail section snippet below (content: contractSection), and a
	// script binding can't share it.
	const contractState = new SectionState<Contract | undefined>(undefined);
	const contract = $derived(contractState.value);
	const contractError = $derived(contractState.error);
	const isContractBusy = $derived(contractState.isBusy);
	let isContractLoaded = $state(false);

	// Named invoicesState for the same reason as contractState above.
	const invoicesState = new SectionState<Invoice[]>([]);
	const invoices = $derived(invoicesState.value);
	const invoicesError = $derived(invoicesState.error);
	let connectGate = $state<{ isOwner: boolean } | undefined>();

	// Offers on this Engagement (#317). Owner/Admin only at the BFF, so a
	// Doula's load simply fails and the section stays hidden -- the read
	// table keeps who-was-asked away from her, and an error banner about
	// it would only be noise on her own screen. loadOffersSection
	// (engagementDetail.ts) already turns that refusal into `undefined`
	// rather than a throw, so offersState.error is never rendered here on
	// purpose -- the section's rule is silence, not a Notice. Named
	// offersState for the same reason as contractState above.
	const offersState = new SectionState<OffersSection | undefined>(undefined);
	const isOffersVisible = $derived(offersState.value !== undefined);
	const offers = $derived((offersState.value?.offers as Offer[] | undefined) ?? []);
	const doulas = $derived(offersState.value?.doulas ?? []);

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

	/** The summary row's own facts (#538). `dueDate` is left out of the
	 * array entirely, rather than shown with a placeholder, when null --
	 * ADR-0017's postpartum-only Engagement genuinely has none, matching
	 * the portal's own answer to the same null (#505). `Created` stays: on
	 * this page it is a fact for the Staff working the Engagement, not one
	 * the record's own subject didn't ask for -- the same "how did this
	 * come to be" the repo asks every feature to answer. */
	function summaryItems(d: Detail, status: string): { label: string; value: string }[] {
		const items = [
			{ label: 'Client', value: d.clientName },
			{ label: 'Status', value: status },
			{ label: 'Created', value: new Date(d.createdAt).toLocaleDateString() }
		];
		if (d.dueDate) {
			items.push({ label: 'Due date', value: formatCalendarDay(d.dueDate) });
		}
		return items;
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

	/** #301's Staff visibility (AC4): whether the Client has read her
	 * Birth Plan since it last changed. Only ever called for
	 * planType === 'birth_plan' -- a Care Plan instance never carries
	 * clientAcknowledgedAt. */
	function birthPlanReviewStatus(instance: Instance): string {
		return instance.clientAcknowledgedAt
			? `Reviewed by client on ${formatInstant(instance.clientAcknowledgedAt)}`
			: 'Not yet reviewed by client';
	}

	async function loadPlan(planType: PlanType) {
		await planState[planType].load(
			() => loadInstance(apiFetchWithSession, page.params.practiceId!, page.params.engagementId!, planType),
			'Failed to load plan'
		);
		planLoaded[planType] = true;
	}

	async function handleCreatePlan(planType: PlanType) {
		await planState[planType].mutate(
			() => createInstance(apiFetchWithSession, page.params.practiceId!, page.params.engagementId!, planType),
			'Failed to create plan'
		);
	}

	function handlePlanAnswerChange(planType: PlanType, fieldId: string, value: unknown) {
		const instance = planState[planType].value;
		if (!instance) return;
		instance.answers = setAnswer(instance.answers, fieldId, value);
	}

	function handlePlanToggleOption(planType: PlanType, fieldId: string, option: string) {
		const instance = planState[planType].value;
		if (!instance) return;
		instance.answers = toggleMultiSelectOption(instance.answers, fieldId, option);
	}

	async function handleSavePlan(planType: PlanType) {
		const instance = planState[planType].value;
		if (!instance) return;
		await planState[planType].mutate(
			() =>
				saveAnswers(
					apiFetchWithSession,
					page.params.practiceId!,
					page.params.engagementId!,
					planType,
					instance.answers
				),
			'Failed to save plan'
		);
	}

	// #306: built fresh from the Birth Plan's current answers, never a
	// stored snapshot -- the same PDF a Client can download from the
	// portal, mirrored on this side (#280 later moves this control onto
	// its own Doula-facing page).
	async function handleDownloadBirthPlanPdf() {
		downloadBirthPlanPdfError = '';
		isDownloadingBirthPlanPdf = true;
		try {
			const blob = await downloadBirthPlanPdf(apiFetchWithSession, page.params.practiceId!, page.params.engagementId!);
			triggerBlobDownload(blob, 'birth-plan.pdf');
		} catch (error_) {
			downloadBirthPlanPdfError = error_ instanceof Error ? error_.message : 'Failed to download Birth Plan';
		} finally {
			isDownloadingBirthPlanPdf = false;
		}
	}

	async function loadContractSection() {
		await contractState.load(
			() => loadContract(apiFetchWithSession, page.params.practiceId!, page.params.engagementId!),
			'Failed to load contract'
		);
		isContractLoaded = true;
	}

	async function handleCreateContract() {
		await contractState.mutate(
			() => createContract(apiFetchWithSession, page.params.practiceId!, page.params.engagementId!),
			'Failed to create contract'
		);
	}

	function handleContractValueChange(key: string, value: string) {
		if (!contractState.value) return;
		contractState.value.values = setMergeFieldValue(contractState.value.values, key, value);
	}

	async function handleSaveContract() {
		if (!contractState.value) return;
		await contractState.mutate(
			() =>
				saveContractValues(
					apiFetchWithSession,
					page.params.practiceId!,
					page.params.engagementId!,
					contractState.value!.values
				),
			'Failed to save contract'
		);
	}

	async function handleSendContract() {
		if (!contractState.value) return;
		await contractState.mutate(
			() => sendContract(apiFetchWithSession, page.params.practiceId!, page.params.engagementId!),
			'Failed to send contract'
		);
	}

	// ContractStatus.svelte owns the error display for Void (it awaits
	// the onVoid callback prop itself and renders whatever it throws) --
	// unlike the other Contract handlers above, this one deliberately
	// doesn't touch contractState.error.
	async function handleVoidContract() {
		if (!contractState.value) return;
		contractState.value = await voidContract(
			apiFetchWithSession,
			page.params.practiceId!,
			page.params.engagementId!
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
		await invoicesState.load(
			() => loadInvoices(apiFetchWithSession, page.params.practiceId!, page.params.engagementId!),
			'Failed to load invoices'
		);
	}

	// Reported by InvoiceSection's onCreate prop -- see its own doc comment
	// for why it owns the resulting state change (invoices list vs.
	// connectGate) rather than the component itself. No catch here in the
	// original either: a refused create is left to the component the same
	// way Void Contract is (see above).
	async function handleCreateInvoice(amountCents: number) {
		const result = await createInvoice(
			apiFetchWithSession,
			page.params.practiceId!,
			page.params.engagementId!,
			amountCents
		);
		connectGate = result.connectRequired ? { isOwner: result.isOwner ?? false } : undefined;
		if (result.invoice) {
			invoicesState.value = [result.invoice, ...invoicesState.value];
		}
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

	async function handleCreateOffer(offer: NewOffer) {
		await createOffer(apiFetchWithSession, page.params.practiceId!, page.params.engagementId!, offer);
		const updated = await loadEngagementOffers(apiFetchWithSession, page.params.practiceId!, page.params.engagementId!);
		if (offersState.value) offersState.value = { ...offersState.value, offers: updated };
	}

	async function handleWithdrawOffer(offerId: string) {
		await withdrawOffer(apiFetchWithSession, page.params.practiceId!, offerId);
		const updated = await loadEngagementOffers(apiFetchWithSession, page.params.practiceId!, page.params.engagementId!);
		if (offersState.value) offersState.value = { ...offersState.value, offers: updated };
	}

	async function handleConnectInvoicing() {
		const onboardingUrl = await connectStripe(apiFetchWithSession, page.params.practiceId!);
		location.assign(onboardingUrl);
	}

	onMount(async () => {
		// The Engagement is already here, from load. What remains is the
		// seven sections that fill in behind it, each rendering as it lands.
		await loadVisits();
		await loadMessages();
		await Promise.all(planSections.map((section) => loadPlan(section.type)));
		await loadContractSection();
		await loadInvoicesSection();
		await loadOffersSection();
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
			return `${location.origin}/portal/accept-invite?token=${created.inviteToken}`;
		}, 'Failed to send portal invite');
	}

	async function handleCreateVisit(event: SubmitEvent) {
		event.preventDefault();
		const scheduledAt = newVisitScheduledAt ? new Date(newVisitScheduledAt).toISOString() : undefined;
		if (
			await visitsCreate.mutate(() => createVisit(apiFetchWithSession, reference, scheduledAt), 'Failed to add Visit')
		) {
			newVisitScheduledAt = '';
			await loadVisits();
		}
	}

	async function handleReassign(visitId: string, event: SubmitEvent) {
		event.preventDefault();
		const section = (reassignSections[visitId] ??= new SectionState<void>(undefined));
		const wasReassigned = await section.mutate(
			() => reassignVisit(apiFetchWithSession, reference, visitId, reassignStaffId[visitId] ?? ''),
			'Failed to reassign Visit'
		);
		if (wasReassigned) {
			reassignStaffId[visitId] = '';
			await loadVisits();
		}
	}

	// #250: an empty control clears the schedule -- `datetime-local`
	// reports "" the same way whether the field was never touched or was
	// deliberately emptied, and `undefined` is scheduleVisit's own way of
	// saying "clear it" (engagementDetail.ts).
	async function handleSchedule(visitId: string, event: SubmitEvent) {
		event.preventDefault();
		const section = (scheduleSections[visitId] ??= new SectionState<void>(undefined));
		const raw = scheduleValue[visitId] ?? '';
		const scheduledAt = raw ? new Date(raw).toISOString() : undefined;
		const wasScheduled = await section.mutate(
			() => scheduleVisit(apiFetchWithSession, reference, visitId, scheduledAt),
			'Failed to update Visit schedule'
		);
		if (wasScheduled) {
			delete scheduleValue[visitId];
			await loadVisits();
		}
	}

	// #251: the field's own current draft, or "" for a fresh visit not yet
	// touched -- saveVisitNotes always sends a string, never undefined,
	// since a Textarea has no separate "unset" state the way an empty
	// datetime-local control does.
	async function handleSaveNotes(visitId: string, event: SubmitEvent) {
		event.preventDefault();
		const section = (notesSections[visitId] ??= new SectionState<void>(undefined));
		const wasSaved = await section.mutate(
			() => saveVisitNotes(apiFetchWithSession, reference, visitId, notesValue[visitId] ?? ''),
			'Failed to save Visit notes'
		);
		if (wasSaved) {
			delete notesValue[visitId];
			await loadVisits();
		}
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

	async function didSendMessage(body: string, attachment: File | undefined): Promise<boolean> {
		return messagesSend.mutate(async () => {
			const created = await sendMessage<Message>(apiFetchWithSession, reference, body, attachment);
			messages = [...messages, created];
			await refreshAttachmentPreviews([created]);
		}, 'Failed to send message');
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
	<stack-l space="var(--space-4)">
		<DescriptionList items={summaryItems(detail!, displayStatus)} />

		<!--
			#253: exactly the moves ADR-0015's role table admits from the
			current status for this caller -- the API decides the set
			(Detail.statusMoves/TransitionResponse.statusMoves), this only
			renders it. A contractor Doula or a status with no legal move
			at all sees no controls here, rather than a disabled one.
		-->
		{#if displayStatusMoves.length > 0}
			<cluster-l space="var(--space-3)">
				{#each displayStatusMoves as move (move)}
					<Button
						label={statusMoveLabel(move)}
						size="sm"
						variant="secondary"
						loading={statusChange.isBusy}
						onClick={() => handleStatusMoveClick(move)}
					/>
				{/each}
			</cluster-l>
		{/if}
		{#if statusChange.error}
			<Notice variant="error" message={statusChange.error} />
		{/if}
		{#if isCompleteFormShown}
			<form onsubmit={handleCompleteSubmit}>
				<RadioGroup
					legend="Why is this Engagement ending?"
					name="ending-reason"
					options={endingReasons}
					value={completeReasonValue}
					onChange={(value) => (completeReasonValue = value)}
					error={completeReasonError}
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
				<Button label="Confirm completion" type="submit" size="sm" loading={statusChange.isBusy} />
				<Button
					label="Cancel"
					type="button"
					size="sm"
					variant="secondary"
					onClick={() => (isCompleteFormShown = false)}
				/>
			</form>
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
		<Link href={clientDetailHref()} label="View Client" describedBy="engagement-client-name" />
		<span class="visually-hidden" id="engagement-client-name">{detail!.clientName}</span>

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
			<Text text="Invited. An email has been sent to them. If you need to share the link directly, here it is:" />
			<div><code>{portalInviteLink}</code></div>
		{/if}
	</stack-l>
{/snippet}

{#snippet actions()}
	<Button label="Send portal invite" onClick={handleSendPortalInvite} loading={isSendingPortalInvite} />
{/snippet}

{#snippet visitActions(visit: Visit)}
	<span class="visually-hidden" id="visit-{visit.visitId}-name"
		>{visit.staffName}, {formatScheduledVisit(visit.scheduledAt)}</span
	>
	<form onsubmit={(event) => handleReassign(visit.visitId, event)}>
		<LabeledField id={`reassign-staff-${visit.visitId}`} label="Reassign to Staff id">
			{#snippet children({ id, describedBy, invalid })}
				<TextInput
					{id}
					{describedBy}
					{invalid}
					value={reassignStaffId[visit.visitId] ?? ''}
					onInput={(value) => (reassignStaffId[visit.visitId] = value)}
					required
				/>
			{/snippet}
		</LabeledField>
		<Button
			label="Reassign"
			type="submit"
			size="sm"
			variant="secondary"
			describedBy="visit-{visit.visitId}-name"
		/>
	</form>
	{#if reassignSections[visit.visitId]?.error}
		<Notice variant="error" message={reassignSections[visit.visitId]!.error} />
	{/if}

	<!--
		#250: an empty value clears the schedule -- there is no separate
		"Clear" control, since emptying the field the datetime picker
		already offers is the plainer way to ask for the same thing.
	-->
	<form onsubmit={(event) => handleSchedule(visit.visitId, event)}>
		<LabeledField id={`schedule-visit-${visit.visitId}`} label="Scheduled date and time">
			{#snippet children({ id, describedBy, invalid })}
				<TextInput
					{id}
					{describedBy}
					{invalid}
					type="datetime-local"
					value={scheduleValue[visit.visitId] ?? toDatetimeLocalValue(visit.scheduledAt)}
					onInput={(value) => (scheduleValue[visit.visitId] = value)}
				/>
			{/snippet}
		</LabeledField>
		<Button
			label="Update schedule"
			type="submit"
			size="sm"
			variant="secondary"
			describedBy="visit-{visit.visitId}-name"
		/>
	</form>
	{#if scheduleSections[visit.visitId]?.error}
		<Notice variant="error" message={scheduleSections[visit.visitId]!.error} />
	{/if}

	<!--
		#251: any Staff member who may read this Visit may also write its
		notes -- there is no Doula-only restriction on this form the way
		reassign and schedule carry, matching the read rule.
	-->
	<form onsubmit={(event) => handleSaveNotes(visit.visitId, event)}>
		<LabeledField id={`notes-visit-${visit.visitId}`} label="Notes">
			{#snippet children({ id, describedBy, invalid })}
				<Textarea
					{id}
					{describedBy}
					{invalid}
					value={notesValue[visit.visitId] ?? visit.notes ?? ''}
					onInput={(value) => (notesValue[visit.visitId] = value)}
				/>
			{/snippet}
		</LabeledField>
		<Button
			label="Save notes"
			type="submit"
			size="sm"
			variant="secondary"
			describedBy="visit-{visit.visitId}-name"
		/>
	</form>
	{#if notesSections[visit.visitId]?.error}
		<Notice variant="error" message={notesSections[visit.visitId]!.error} />
	{/if}
{/snippet}

{#snippet visitsSection()}
	<form onsubmit={handleCreateVisit}>
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
		<Button label="Add a Visit" type="submit" loading={isCreatingVisit} />
	</form>

	{#if visitsError}
		<Notice variant="error" message={visitsError} />
	{/if}

	<DataTable
		columns={[
			{ label: 'Staff', accessor: (visit: Visit) => visit.staffName },
			{ label: 'Date', accessor: (visit: Visit) => formatScheduledVisit(visit.scheduledAt) },
			{ label: 'Notes', accessor: (visit: Visit) => visit.notes || 'No notes yet.' }
		]}
		rows={visits.items}
		rowActions={{ label: 'Actions', content: visitActions }}
		hasMore={visits.hasMore}
		onLoadMore={() => visits.loadMore()}
		isLoadingMore={visits.isLoadingMore}
		loadMoreError={visits.loadMoreError}
		emptyMessage="No Visits yet."
	/>
{/snippet}

<!--
	One body for both Plan sections, parameterised the way the `planSections`
	loop was: they are the same generic Plan Instance API with a different
	plan type (see planInstance.ts). A Template section's `content` takes no
	arguments, so each type gets a thin wrapper below rather than the loop.
-->
{#snippet planSectionBody(planType: PlanType, heading: string)}
	{#if planState[planType].error}
		<Notice variant="error" message={planState[planType].error} />
	{/if}

	{#if planLoaded[planType]}
		{#if planState[planType].value}
			{#if planType === 'birth_plan'}
				<p>{birthPlanReviewStatus(planState[planType].value!)}</p>
			{/if}
			<PlanInstanceForm
				fields={planState[planType].value!.fields}
				answers={planState[planType].value!.answers}
				onAnswerChange={(fieldId, value) => handlePlanAnswerChange(planType, fieldId, value)}
				onToggleOption={(fieldId, option) => handlePlanToggleOption(planType, fieldId, option)}
			/>
			<Button label="Save {heading}" onClick={() => handleSavePlan(planType)} loading={planState[planType].isBusy} />
		{:else}
			<Button label="Create {heading}" onClick={() => handleCreatePlan(planType)} loading={planState[planType].isBusy} />
		{/if}
	{/if}
{/snippet}

{#snippet carePlanSection()}
	{@render planSectionBody('care_plan', 'Care Plan')}
{/snippet}

{#snippet birthPlanSection()}
	{@render planSectionBody('birth_plan', 'Birth Plan')}
	{#if planLoaded.birth_plan && planState.birth_plan.value}
		<Button
			label="Download Birth Plan (PDF)"
			icon="file-text"
			variant="secondary"
			onClick={handleDownloadBirthPlanPdf}
			loading={isDownloadingBirthPlanPdf}
		/>
		{#if downloadBirthPlanPdfError}
			<p role="alert">{downloadBirthPlanPdfError}</p>
		{/if}
	{/if}
{/snippet}

{#snippet contractSection()}
	{#if contractError}
		<Notice variant="error" message={contractError} />
	{/if}

	{#if isContractLoaded}
		{#if contract}
			<ContractStatus
				status={contract.status}
				onVoid={handleVoidContract}
				onDownloadPdf={isPracticeOwnerOrAdmin ? handleDownloadSignedContractPdf : undefined}
			/>
			<ContractForm
				mergeFields={contract.mergeFields}
				values={contract.values}
				readOnly={contract.status !== 'draft'}
				onValueChange={handleContractValueChange}
			/>
			{#if contract.status === 'draft'}
				<Button label="Save Contract" onClick={handleSaveContract} loading={isContractBusy} variant="secondary" />
				<Button label="Send Contract" onClick={handleSendContract} loading={isContractBusy} />
			{/if}
		{:else}
			<Button label="Create Draft Contract" onClick={handleCreateContract} loading={isContractBusy} />
		{/if}
	{/if}
{/snippet}

{#snippet invoicesSection()}
	{#if invoicesError}
		<Notice variant="error" message={invoicesError} />
	{/if}

	<InvoiceSection {invoices} {connectGate} onCreate={handleCreateInvoice} onConnect={handleConnectInvoicing} />
{/snippet}

{#snippet offersSection()}
	<OfferSection
		{offers}
		{doulas}
		clientName={detail!.clientName}
		onCreate={handleCreateOffer}
		onWithdraw={handleWithdrawOffer}
	/>
{/snippet}

{#snippet messagesSection()}
	<MessageThread
		{messages}
		error={messagesError}
		hasMore={isMessagesHasMore}
		isLoadingOlder={isLoadingOlderMessages}
		isSending={isSendingMessage}
		onLoadOlder={handleLoadOlderMessages}
		onSend={didSendMessage}
		onDownloadAttachment={handleDownloadAttachment}
		{attachmentPreviewURLs}
	/>
{/snippet}

<!--
	#486 AC4: the brief's ledger treatment -- a meta date column, the event
	in body text, the actor muted. Last in `sections` (below), matching the
	design brief's #433 amendment: it sits low on every page it appears on.
-->
{#snippet activitySection()}
	{#if activityError}
		<Notice variant="error" message={activityError} />
	{/if}
	<DataTable
		columns={activityLedgerColumns()}
		rows={activity.items}
		hasMore={activity.hasMore}
		onLoadMore={() => activity.loadMore()}
		isLoadingMore={activity.isLoadingMore}
		loadMoreError={activity.loadMoreError}
		emptyMessage="Nothing has happened yet."
	/>
{/snippet}

<!--
	Archetype D, ADR-0018. The Invoices and Offers sections are still
	conditional exactly as before -- Invoices needs a Contract to exist
	and Offers is Owner/Admin-only -- which is why `sections` is a typed
	array and not a run of named regions.
-->
<!--
	No loading or loadError: the Engagement is loaded before this page
	renders, and a refusal reaches practices/+error.svelte instead (#695).
	The sections below still fill in after mount, each with its own state.
-->
<RecordDetail
	title={detail.clientName}
	{summary}
	{actions}
	isContentsShown
	sections={[
		{ heading: 'Visits', content: visitsSection },
		{ heading: 'Care Plan', content: carePlanSection },
		{ heading: 'Birth Plan', content: birthPlanSection },
		{ heading: 'Contract', content: contractSection },
		...(contract ? [{ heading: 'Invoices', content: invoicesSection }] : []),
		...(isOffersVisible ? [{ heading: 'Offers', content: offersSection }] : []),
		{ heading: 'Messages', content: messagesSection },
		{ heading: 'Activity', content: activitySection }
	]}
/>
