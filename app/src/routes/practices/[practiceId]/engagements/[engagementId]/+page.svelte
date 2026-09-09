<script lang="ts">
	import { onDestroy, onMount } from 'svelte';
	import { page } from '#lib/appState.svelte.js';
	import { apiFetchWithSession } from '#lib/api.js';
	import { PaginatedList } from '#lib/paginatedList.svelte.js';
	import { SectionState } from '#lib/sectionState.svelte.js';
	import { FormSubmission, orThrownErrors } from '#lib/formSubmission.svelte.js';
	import { triggerBlobDownload } from '#lib/blobDownload.js';
	import {
		changeEngagementStatus,
		createVisit,
		downloadAttachment,
		endingReasons,
		loadAttachmentPreviews,
		loadEngagementOffersOrNone as loadOffers,
		loadMessagesPage,
		loadVisitsPage,
		reassignVisit,
		recordBirthOutcome,
		saveVisitNotes,
		scheduleVisit,
		sendMessage,
		sendPortalInvite,
		visitTypeLabel,
		type BirthOutcomeFacts,
		type BirthOutcomeRequest,
		type Visit
	} from '#lib/engagementDetail.js';
	import {
		assigneeBlock,
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
	import type { PageProps as PageProperties } from './$types';
	import { formatCalendarDay, formatInstant, formatScheduledVisit, toDatetimeLocalValue } from '#lib/dates.js';
	import { activityLedgerColumns, loadEngagementActivityPage, type ActivityEntry } from '#lib/activityLedger.js';
	import { subscribeToThreadPushMessages } from '#lib/pushRefresh.js';
	import PlanInstanceForm from '#lib/components/organisms/PlanInstanceForm.svelte';
	import {
		loadInstance,
		createInstance,
		saveAnswers,
		setAnswer,
		toggleMultiSelectOption,
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
		createInvoice,
		loadBillingMode,
		recordPayment,
		reversePayment,
		voidInvoice,
		writeOffInvoice,
		type BillingMode,
		type Invoice,
		type PaymentMethod
	} from '#lib/invoice.js';
	import OfferSection from '#lib/components/organisms/OfferSection.svelte';
	import { createOffer, loadEngagementOffers, withdrawOffer, type NewOffer, type Offer } from '#lib/offer.js';
	import { resolve } from '$app/paths';
	import MessageThread, { type Message } from '#lib/components/organisms/MessageThread.svelte';
	import Text from '#lib/components/atoms/Text.svelte';
	import Link from '#lib/components/atoms/Link.svelte';
	import Button from '#lib/components/atoms/Button.svelte';
	import Notice from '#lib/components/atoms/Notice.svelte';
	import DescriptionList from '#lib/components/molecules/DescriptionList.svelte';
	import RadioGroup from '#lib/components/molecules/RadioGroup.svelte';
	import ErrorSummary from '#lib/components/molecules/ErrorSummary.svelte';
	import DataTable from '#lib/components/organisms/DataTable.svelte';
	import RecordDetail from '#lib/components/templates/RecordDetail.svelte';
	import TextInput from '#lib/components/atoms/TextInput.svelte';
	import Textarea from '#lib/components/atoms/Textarea.svelte';
	import LabeledField from '#lib/components/molecules/LabeledField.svelte';
	import Select from '#lib/components/atoms/Select.svelte';

	type Detail = {
		engagementId: string;
		clientId: string;
		clientName: string;
		status: string;
		createdAt: string;
		dueDate?: string;
		statusMoves: string[];
		// #293/#943: ADR-0015's birth outcome and the date the pregnancy
		// ended, both absent until a Practice records them.
		birthOutcome?: string;
		pregnancyEndedOn?: string;
		// #255: the Client's portal-invite state, mirroring
		// engagementDetail.ts's own EngagementSummary fields.
		clientPortalInviteStatus?: string;
		clientEmailSuppressed?: boolean;
		clientHasEmail?: boolean;
		// #270: whether Clients can pay this Practice at all -- a standing
		// fact InvoiceSection reads before ever showing the Create Invoice
		// form, never a routed gate discovered after a submit attempt.
		clientsCanPay?: boolean;
	};

	// The Engagement comes from +page.ts's load now, not an onMount fetch
	// (#695): a refusal has to reach practices/+error.svelte rather than
	// sit in a local error string, the same move #471 made for billing.
	// The other six sections still load after mount, each rendering as it
	// lands.
	let { data }: PageProperties = $props();
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

	async function handleStatusMoveClick(move: string) {
		if (move === 'completed') {
			isCompleteFormShown = true;
			return;
		}
		isCompleteFormShown = false;
		await directMove.mutate(async () => {
			statusOverride = await changeEngagementStatus(apiFetchWithSession, reference, move);
		}, 'Failed to change status');
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
		}, orThrownErrors(completeFieldIds));
	}

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

	// The app-side mirror of api/internal/engagement/transition.go's own
	// refuseFactWrite: a contractor Doula may not record this fact, and
	// neither may a member holding none of the three roles. Drawing only,
	// never the gate -- the endpoint refuses her whether or not the
	// control was drawn (ADR-0006).
	const canRecordBirthOutcome = $derived(
		!isAmbientContractor(data.session) && (isPracticeOwnerOrAdmin || isDoula(data.session))
	);

	async function handleRecordBirthOutcome(
		request: BirthOutcomeRequest,
		fieldIds: Record<string, string>
	) {
		const result = await recordBirthOutcome(apiFetchWithSession, reference, request, fieldIds);
		if (result.kind === 'recorded') birthOutcomeOverride = result.facts;
		return result;
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

	// Named invoicesState for the same reason as contractState above.
	const invoicesState = new SectionState<Invoice[]>([]);
	const invoices = $derived(invoicesState.value);
	const invoicesError = $derived(invoicesState.error);

	// #271: undefined until this Practice has chosen a billing rail --
	// InvoiceSection itself decides what to render for that, from the
	// same standing fact clientsCanPay/hasClientEmail already are.
	const billingModeState = new SectionState<BillingMode | undefined>(undefined);
	const billingMode = $derived(billingModeState.value);

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
	const newVisitBlock = $derived(assigneeBlock(doulas ?? [], visitStaffId));
	// The Offers section turns on the Offers read alone, not on the roster.
	// The two happen to be the same pair of roles today (both Owner and
	// Admin), so tying Offers to `canAssignVisits` looked free -- but it
	// makes an unrelated roster outage take the Offers section down with
	// it, and it re-decides who may see Offers in a place that is not
	// about Offers. Each section answers for its own read.
	const isOffersVisible = $derived(offersState.value !== undefined);

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
	 * come to be" the repo asks every feature to answer.
	 *
	 * "Portal invite" (#255) states the Client's portal-invite state as
	 * standing information, not only as feedback after a Send -- the same
	 * word the Clients list column carries, via portalInviteStatusText,
	 * so the two screens never drift into two vocabularies. */
	function summaryItems(
		d: Detail,
		status: string,
		portalState: PortalInviteSubject,
		hasClientEmail: boolean
	): { label: string; value: string }[] {
		const items = [
			{ label: 'Client', value: d.clientName },
			{ label: 'Status', value: status },
			{ label: 'Portal invite', value: portalInviteSummaryText(portalState, hasClientEmail) },
			{ label: 'Created', value: new Date(d.createdAt).toLocaleDateString() }
		];
		if (d.dueDate) {
			items.push({ label: 'Due date', value: formatCalendarDay(d.dueDate) });
		}
		return items;
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
					editableValues(contractState.value!.values)
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
		await invoicesState.load(
			() => loadInvoices(apiFetchWithSession, page.params.practiceId!, page.params.engagementId!),
			'Failed to load invoices'
		);
		// #271: read alongside the Invoice list itself -- InvoiceSection
		// needs to know the current mode (or its absence) before it can
		// decide whether to show the "ask once" form.
		await billingModeState.load(
			() => loadBillingMode(apiFetchWithSession, page.params.practiceId!),
			'Failed to load billing mode'
		);
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
		invoicesState.value = [invoice, ...invoicesState.value];
		billingModeState.value = invoice.billingMode;
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

	// Silent about a *refusal*, and only about a refusal: a roster read
	// this reader's role does not admit her to is her role, not a failure,
	// and loadVisitAssigneesOrNone answers it with `undefined` rather than
	// a throw. Anything else -- a 500, a dropped connection -- it
	// rethrows, and rosterState.error carries it to the Notice above the
	// Visits table, because the alternative is an Owner watching every
	// assign-shaped control on this page vanish with no reason given.
	async function loadRoster() {
		await rosterState.load(
			() => loadVisitAssigneesOrNone(apiFetchWithSession, reference.practiceId, reference.engagementId),
			'We could not load the Practice roster, so there is nobody to pick from. Try again.'
		);
	}

	// The same block the create picker carries, asked per row: the
	// reassign picker offers the same people from the same list, so it
	// refuses the same choice with the same words.
	function reassignBlock(visitId: string): string {
		return assigneeBlock(doulas ?? [], reassignStaffId[visitId] ?? '');
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

	onMount(async () => {
		// The Engagement is already here, from load. What remains is the
		// seven sections that fill in behind it, each rendering as it lands.
		await loadVisits();
		await loadMessages();
		await Promise.all(planSections.map((section) => loadPlan(section.type)));
		await loadContractSection();
		await loadInvoicesSection();
		await loadRoster();
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
			// #255: a fresh invite is always pending, never suppressed --
			// lifts the Contract section's block and hides this action's
			// own visibility question ("never invited") right away.
			clientPortalOverride = { portalInviteStatus: 'pending', emailSuppressed: false };
			return `${location.origin}/portal/accept-invite?token=${created.inviteToken}`;
		}, 'Failed to send portal invite');
	}

	// #268: an empty picker means "for me", which is what a Doula logging
	// her own Visit sends -- the same absent assignee this route sent
	// before the field existed.
	async function handleCreateVisit(event: SubmitEvent) {
		event.preventDefault();
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
			newVisitScheduledAt = '';
			// Back to untouched, not to empty: the next Visit she logs
			// starts on the same standing answer this one did.
			newVisitStaffId = undefined;
			await loadVisits();
		}
	}

	async function handleReassign(visitId: string, event: SubmitEvent) {
		event.preventDefault();
		// #911, the same block the create form makes.
		if (reassignBlock(visitId)) return;
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
		<DescriptionList items={summaryItems(detail!, displayStatus, clientPortalState, hasClientEmailOnFile)} />

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
						loading={directMove.isBusy}
						onClick={() => handleStatusMoveClick(move)}
					/>
				{/each}
			</cluster-l>
		{/if}
		{#if directMove.error}
			<Notice variant="error" message={directMove.error} />
		{/if}
		{#if isCompleteFormShown}
			<form onsubmit={handleCompleteSubmit} novalidate>
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
				<Button label="Confirm completion" type="submit" size="sm" loading={completeSubmission.isSubmitting} />
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
			<Button label="Send portal invite" onClick={handleSendPortalInvite} loading={isSendingPortalInvite} />
		{:else}
			<Notice variant="info" message="This Client has no email address on file. Add one before sending a portal invite." />
		{/if}
	{/if}
{/snippet}

{#snippet visitActions(visit: Visit)}
	<span class="visually-hidden" id="visit-{visit.visitId}-name"
		>{visit.staffName}, {formatScheduledVisit(visit.scheduledAt)}</span
	>
	<!--
		#268: a person is picked by name, never by a staff id typed by hand
		-- no screen in the product prints one, so the free-text field this
		replaces could not be filled in. Rendered only where the roster
		read succeeded, which is the same Owner/Admin rule the BFF applies
		to the write itself (#274).
	-->
	{#if canAssignVisits}
		<!--
			#909: the person this Visit already belongs to is not a
			reassignment, so she is not offered. Once she was the only
			eligible name, there is no move left to make, and the control
			says so rather than presenting an empty picker with a
			placeholder and no explanation.
			#911: what is left is marked the same way the create picker
			marks it -- the same derivation, so the two cannot disagree.
		-->
		{@const reassignOptions = visitAssigneeOptions(doulas ?? [], {
			callerStaffId,
			currentAssigneeStaffId: visit.staffId
		})}
		{#if reassignOptions.length === 0}
			<p>There is nobody else to reassign this Visit to.</p>
		{:else}
			<form onsubmit={(event) => handleReassign(visit.visitId, event)}>
				<LabeledField
					id={`reassign-staff-${visit.visitId}`}
					label="Reassign to"
					hint={assigneeHint}
					error={reassignBlock(visit.visitId)}
				>
					{#snippet children({ id, describedBy, invalid })}
						<Select
							{id}
							{describedBy}
							{invalid}
							options={reassignOptions}
							placeholder="Choose a Doula"
							value={reassignStaffId[visit.visitId] ?? ''}
							onChange={(value) => (reassignStaffId[visit.visitId] = value)}
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
		{/if}
		{#if reassignSections[visit.visitId]?.error}
			<Notice variant="error" message={reassignSections[visit.visitId]!.error} />
		{/if}
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
	<!--
		#268/#274: the form is here for a reader who can actually complete
		it -- an Owner or Admin, who picks the colleague it is for, or a
		Doula, who logs her own. A reader who is neither is offered no
		control rather than a button that 403s.
	-->
	{#if canAssignVisits || canLogOwnVisit}
		<form onsubmit={handleCreateVisit}>
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
				>
					{#snippet children({ id, describedBy, invalid })}
						<Select
							{id}
							{describedBy}
							{invalid}
							options={createAssigneeOptions}
							placeholder="Choose a Doula"
							value={visitStaffId}
							onChange={(value) => (newVisitStaffId = value)}
							required
						/>
					{/snippet}
				</LabeledField>
			{/if}
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
	{/if}

	{#if rosterState.error}
		<Notice variant="error" message={rosterState.error} />
	{/if}

	{#if visitsError}
		<Notice variant="error" message={visitsError} />
	{/if}

	<DataTable
		columns={[
			{ label: 'Staff', accessor: (visit: Visit) => visit.staffName },
			{ label: 'Type', accessor: (visit: Visit) => visitTypeLabel(visit.type) },
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
	<!--
		#280: the reading and taking-away surface -- its own address,
		reachable regardless of whether a Birth Plan exists yet (that
		address says so plainly rather than 404ing). Editing stays here.
	-->
	<Link href={birthPlanHref} label="View printable Birth Plan" icon="file-text" variant="secondary" />
	{@render planSectionBody('birth_plan', 'Birth Plan')}
{/snippet}

{#snippet contractSection()}
	{#if contractError}
		<Notice variant="error" message={contractError} />
	{/if}

	{#if isContractLoaded}
		{#if contract}
			<ContractStatus
				status={contract.status}
				amountChangedAt={contract.amountChangedAt}
				voidRequests={contract.voidRequests}
				onVoid={isPracticeOwnerOrAdmin ? handleVoidContract : undefined}
				onDownloadPdf={canReadContractMoney ? handleDownloadSignedContractPdf : undefined}
				onRequestVoid={isPracticeOwnerOrAdmin ? undefined : handleRequestVoidContract}
				onDeclineVoidRequest={isPracticeOwnerOrAdmin ? handleDeclineVoidRequest : undefined}
			/>
			<!--
				#258: Staff reads the same filled document the Client will,
				before sending it -- the fill-and-render component used to be
				mounted only on the Client-portal Contract page, so a Doula
				sent a document she had never seen.
			-->
			<Text text="Contract text" />
			<ContractView prose={contract.prose} values={contract.values} />
			<ContractForm
				mergeFields={editableMergeFields(contract.mergeFields)}
				values={contract.values}
				readOnly={contract.status !== 'draft'}
				onValueChange={handleContractValueChange}
			/>
			{#if contract.status === 'draft'}
				<Button label="Save Contract" onClick={handleSaveContract} loading={isContractBusy} variant="secondary" />
				<!--
					#258: block over warn -- a blank merge field is a legal
					document defect, so the Send control is withheld with the
					reason and the offending fields stated, rather than
					reporting the server's refusal after the click.
					#255: a Contract sent to a never-invited Client would land
					in 'sent' with nothing able to move it back out -- reachable
					only through the portal, which is reachable only by
					accepting an invite. The precondition is the same one
					PostSendContractHandler enforces server-side; this only
					prevents the situation rather than reporting the refusal
					after the click (block over warn). The Notice names the
					ordering and the way out -- sending the portal invite,
					this same page's own header action -- removable in one
					click, which is what lifts the block without a reload
					(clientPortalOverride above).
				-->
				{#if missingMergeFields.length > 0}
					<Notice
						variant="info"
						message={`Fill in every merge field before sending the Contract. Missing: ${missingMergeFields
							.map((key) => mergeFieldLabel(key))
							.join(', ')}.`}
					/>
					<Button label="Send Contract" disabled loading={isContractBusy} />
				{:else if hasNeverInvitedClient}
					<Notice
						variant="info"
						message="Send a portal invite to this Client before sending the Contract — the Contract can only be viewed and signed once portal access exists."
					/>
					<Button label="Send Contract" disabled loading={isContractBusy} />
				{:else}
					<Button label="Send Contract" onClick={handleSendContract} loading={isContractBusy} />
				{/if}
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
	{#if billingModeState.error}
		<Notice variant="error" message={billingModeState.error} />
	{/if}

	<InvoiceSection
		{invoices}
		contractStatus={contract!.status}
		{billingMode}
		clientsCanPay={canClientsPay}
		hasClientEmail={hasClientEmailOnFile}
		isOwner={isPracticeOwner}
		isOwnerOrAdmin={isPracticeOwnerOrAdmin}
		{paymentsSettingsHref}
		onCreate={handleCreateInvoice}
		onRecordPayment={handleRecordPayment}
		onReversePayment={handleReversePayment}
		onVoidInvoice={handleVoidInvoice}
		onWriteOffInvoice={handleWriteOffInvoice}
	/>
{/snippet}

{#snippet offersSection()}
	<OfferSection
		{offers}
		doulas={doulas ?? []}
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
{#snippet birthOutcomeSection()}
	<BirthOutcomeSection
		outcome={birthOutcome.birthOutcome}
		endedOn={birthOutcome.pregnancyEndedOn}
		canRecord={canRecordBirthOutcome}
		canCorrect={isPracticeOwner}
		onRecord={handleRecordBirthOutcome}
	/>
{/snippet}

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
		{ heading: 'Birth outcome', content: birthOutcomeSection },
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
