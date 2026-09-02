<script lang="ts">
	import { onDestroy, onMount } from 'svelte';
	import { page } from '$app/state';
	import { apiErrorMessage, apiFetchWithSession } from '#lib/api.js';
	import { formatCalendarDay } from '#lib/dates.js';
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
	import {
		loadContract,
		createContract,
		saveContractValues,
		sendContract,
		voidContract,
		setMergeFieldValue,
		type Contract
	} from '#lib/contract.js';
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
	import DataTable from '#lib/components/organisms/DataTable.svelte';
	import RecordDetail from '#lib/components/templates/RecordDetail.svelte';
	import TextInput from '#lib/components/atoms/TextInput.svelte';
	import LabeledField from '#lib/components/molecules/LabeledField.svelte';

	type Detail = {
		engagementId: string;
		clientId: string;
		clientName: string;
		status: string;
		createdAt: string;
		dueDate?: string;
	};

	type Visit = {
		visitId: string;
		staffId: string;
		staffName: string;
		createdAt: string;
	};

	let detail = $state<Detail | undefined>();
	let error = $state('');

	let visits = $state<Visit[]>([]);
	let visitsError = $state('');
	let isCreatingVisit = $state(false);
	let visitsCursor = $state('');
	let isMoreVisitsAvailable = $state(false);
	let isLoadingMoreVisits = $state(false);
	let loadMoreVisitsError = $state('');

	let portalInviteLink = $state('');
	let portalInviteError = $state('');
	let isSendingPortalInvite = $state(false);
	let reassignStaffId = $state<Record<string, string>>({});
	let reassignError = $state<Record<string, string>>({});

	let messages = $state<Message[]>([]);
	let messagesError = $state('');
	let messagesCursor = $state('');
	let isMessagesHasMore = $state(false);
	let isLoadingOlderMessages = $state(false);
	let isSendingMessage = $state(false);
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
	let planInstances = $state<Record<PlanType, Instance | undefined>>({
		care_plan: undefined,
		birth_plan: undefined
	});
	let planLoaded = $state<Record<PlanType, boolean>>({ care_plan: false, birth_plan: false });
	let planError = $state<Record<PlanType, string>>({ care_plan: '', birth_plan: '' });
	let planBusy = $state<Record<PlanType, boolean>>({ care_plan: false, birth_plan: false });

	let contract = $state<Contract | undefined>();
	let isContractLoaded = $state(false);
	let contractError = $state('');
	let isContractBusy = $state(false);

	let invoices = $state<Invoice[]>([]);
	let invoicesError = $state('');
	let connectGate = $state<{ isOwner: boolean } | undefined>();

	// Offers on this Engagement (#317). Owner/Admin only at the BFF, so a
	// Doula's load simply fails and the section stays hidden -- the read
	// table keeps who-was-asked away from her, and an error banner about
	// it would only be noise on her own screen.
	let offers = $state<Offer[]>([]);
	let doulas = $state<{ staffId: string; name: string; employmentType: string }[]>([]);
	let isOffersVisible = $state(false);

	onDestroy(() => {
		for (const url of Object.values(attachmentPreviewURLs)) {
			URL.revokeObjectURL(url);
		}
		unsubscribePushMessages();
	});

	async function loadAttachmentPreviews(items: Message[]) {
		await Promise.all(
			items
				.filter(
					(m) => m.attachmentContentType?.startsWith('image/') && !Object.hasOwn(attachmentPreviewURLs, m.messageId)
				)
				.map(async (m) => {
					const response = await apiFetchWithSession(`${messagesURL()}/${m.messageId}/attachment`);
					if (!response.ok) return;
					const blob = await response.blob();
					attachmentPreviewURLs[m.messageId] = URL.createObjectURL(blob);
				})
		);
	}

	function portalInviteURL() {
		return `/api/practices/${page.params.practiceId}/engagements/${page.params.engagementId}/portal-invite`;
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
	function summaryItems(d: Detail): { label: string; value: string }[] {
		const items = [
			{ label: 'Client', value: d.clientName },
			{ label: 'Status', value: d.status },
			{ label: 'Created', value: new Date(d.createdAt).toLocaleDateString() }
		];
		if (d.dueDate) {
			items.push({ label: 'Due date', value: formatCalendarDay(d.dueDate) });
		}
		return items;
	}

	function visitsURL() {
		return `/api/practices/${page.params.practiceId}/engagements/${page.params.engagementId}/visits`;
	}

	async function loadVisits() {
		const response = await apiFetchWithSession(visitsURL());
		if (!response.ok) {
			visitsError = await response.text();
			return;
		}
		const loaded = await response.json();
		visits = loaded.items;
		visitsCursor = loaded.nextCursor ?? '';
		isMoreVisitsAvailable = loaded.hasMore;
	}

	// Visits are newest-first from the BFF (#446); appended to the end of
	// what's already on screen rather than reversed, since this is a
	// table read top-to-bottom, not a chat thread.
	async function handleLoadMoreVisits() {
		loadMoreVisitsError = '';
		isLoadingMoreVisits = true;
		try {
			const response = await apiFetchWithSession(`${visitsURL()}?cursor=${encodeURIComponent(visitsCursor)}`);
			if (!response.ok) {
				loadMoreVisitsError = await response.text();
				return;
			}
			const loaded = await response.json();
			visits = [...visits, ...loaded.items];
			visitsCursor = loaded.nextCursor ?? '';
			isMoreVisitsAvailable = loaded.hasMore;
		} catch (error_) {
			loadMoreVisitsError = error_ instanceof Error ? error_.message : 'Failed to load more Visits';
		} finally {
			isLoadingMoreVisits = false;
		}
	}

	function messagesURL() {
		return `/api/practices/${page.params.practiceId}/engagements/${page.params.engagementId}/messages`;
	}

	async function loadMessages() {
		const response = await apiFetchWithSession(messagesURL());
		if (!response.ok) {
			messagesError = await response.text();
			return;
		}
		const data = await response.json();
		messages = data.items.toReversed();
		messagesCursor = data.nextCursor ?? '';
		isMessagesHasMore = data.hasMore;
		await loadAttachmentPreviews(messages);
	}

	async function loadPlan(planType: PlanType) {
		planError[planType] = '';
		try {
			planInstances[planType] = await loadInstance(
				apiFetchWithSession,
				page.params.practiceId!,
				page.params.engagementId!,
				planType
			);
		} catch (error_) {
			planError[planType] = error_ instanceof Error ? error_.message : 'Failed to load plan';
		} finally {
			planLoaded[planType] = true;
		}
	}

	async function handleCreatePlan(planType: PlanType) {
		planError[planType] = '';
		planBusy[planType] = true;
		try {
			planInstances[planType] = await createInstance(
				apiFetchWithSession,
				page.params.practiceId!,
				page.params.engagementId!,
				planType
			);
		} catch (error_) {
			planError[planType] = error_ instanceof Error ? error_.message : 'Failed to create plan';
		} finally {
			planBusy[planType] = false;
		}
	}

	function handlePlanAnswerChange(planType: PlanType, fieldId: string, value: unknown) {
		const instance = planInstances[planType];
		if (!instance) return;
		instance.answers = setAnswer(instance.answers, fieldId, value);
	}

	function handlePlanToggleOption(planType: PlanType, fieldId: string, option: string) {
		const instance = planInstances[planType];
		if (!instance) return;
		instance.answers = toggleMultiSelectOption(instance.answers, fieldId, option);
	}

	async function handleSavePlan(planType: PlanType) {
		const instance = planInstances[planType];
		if (!instance) return;
		planError[planType] = '';
		planBusy[planType] = true;
		try {
			planInstances[planType] = await saveAnswers(
				apiFetchWithSession,
				page.params.practiceId!,
				page.params.engagementId!,
				planType,
				instance.answers
			);
		} catch (error_) {
			planError[planType] = error_ instanceof Error ? error_.message : 'Failed to save plan';
		} finally {
			planBusy[planType] = false;
		}
	}

	async function loadContractSection() {
		contractError = '';
		try {
			contract = await loadContract(apiFetchWithSession, page.params.practiceId!, page.params.engagementId!);
		} catch (error_) {
			contractError = error_ instanceof Error ? error_.message : 'Failed to load contract';
		} finally {
			isContractLoaded = true;
		}
	}

	async function handleCreateContract() {
		contractError = '';
		isContractBusy = true;
		try {
			contract = await createContract(apiFetchWithSession, page.params.practiceId!, page.params.engagementId!);
		} catch (error_) {
			contractError = error_ instanceof Error ? error_.message : 'Failed to create contract';
		} finally {
			isContractBusy = false;
		}
	}

	function handleContractValueChange(key: string, value: string) {
		if (!contract) return;
		contract.values = setMergeFieldValue(contract.values, key, value);
	}

	async function handleSaveContract() {
		if (!contract) return;
		contractError = '';
		isContractBusy = true;
		try {
			contract = await saveContractValues(
				apiFetchWithSession,
				page.params.practiceId!,
				page.params.engagementId!,
				contract.values
			);
		} catch (error_) {
			contractError = error_ instanceof Error ? error_.message : 'Failed to save contract';
		} finally {
			isContractBusy = false;
		}
	}

	async function handleSendContract() {
		if (!contract) return;
		contractError = '';
		isContractBusy = true;
		try {
			contract = await sendContract(apiFetchWithSession, page.params.practiceId!, page.params.engagementId!);
		} catch (error_) {
			contractError = error_ instanceof Error ? error_.message : 'Failed to send contract';
		} finally {
			isContractBusy = false;
		}
	}

	// ContractStatus.svelte owns the error display for Void (it awaits
	// the onVoid callback prop itself and renders whatever it throws) --
	// unlike the other Contract handlers above, this one deliberately
	// doesn't set contractError.
	async function handleVoidContract() {
		if (!contract) return;
		contract = await voidContract(apiFetchWithSession, page.params.practiceId!, page.params.engagementId!);
	}

	async function loadInvoicesSection() {
		invoicesError = '';
		try {
			invoices = await loadInvoices(apiFetchWithSession, page.params.practiceId!, page.params.engagementId!);
		} catch (error_) {
			invoicesError = error_ instanceof Error ? error_.message : 'Failed to load invoices';
		}
	}

	// Reported by InvoiceSection's onCreate prop -- see its own doc comment
	// for why it owns the resulting state change (invoices list vs.
	// connectGate) rather than the component itself.
	async function handleCreateInvoice(amountCents: number) {
		const result = await createInvoice(
			apiFetchWithSession,
			page.params.practiceId!,
			page.params.engagementId!,
			amountCents
		);
		connectGate = result.connectRequired ? { isOwner: result.isOwner ?? false } : undefined;
		if (result.invoice) {
			invoices = [result.invoice, ...invoices];
		}
	}

	// The roster read and the Offers read are both Owner/Admin; either
	// refusing is what tells this page the caller is a Doula, so the
	// section is left out rather than shown broken.
	async function loadOffersSection() {
		try {
			offers = await loadEngagementOffers(apiFetchWithSession, page.params.practiceId!, page.params.engagementId!);
			const response = await apiFetchWithSession(`/api/practices/${page.params.practiceId}/staff`);
			if (!response.ok) return;
			const roster = await response.json();
			doulas = roster.members
				.filter((member: { roles: string[] }) => member.roles.includes('doula'))
				.map((member: { staffId: string; name: string; employmentType: string }) => ({
					staffId: member.staffId,
					name: member.name,
					employmentType: member.employmentType
				}));
			isOffersVisible = true;
		} catch {
			// Not permitted to read who was offered this work -- see above.
		}
	}

	async function handleCreateOffer(offer: NewOffer) {
		await createOffer(apiFetchWithSession, page.params.practiceId!, page.params.engagementId!, offer);
		offers = await loadEngagementOffers(apiFetchWithSession, page.params.practiceId!, page.params.engagementId!);
	}

	async function handleWithdrawOffer(offerId: string) {
		await withdrawOffer(apiFetchWithSession, page.params.practiceId!, offerId);
		offers = await loadEngagementOffers(apiFetchWithSession, page.params.practiceId!, page.params.engagementId!);
	}

	async function handleConnectInvoicing() {
		const onboardingUrl = await connectStripe(apiFetchWithSession, page.params.practiceId!);
		location.assign(onboardingUrl);
	}

	onMount(async () => {
		const response = await apiFetchWithSession(
			`/api/practices/${page.params.practiceId}/engagements/${page.params.engagementId}`
		);
		if (!response.ok) {
			error = await response.text();
			return;
		}

		detail = await response.json();
		await loadVisits();
		await loadMessages();
		await Promise.all(planSections.map((section) => loadPlan(section.type)));
		await loadContractSection();
		await loadInvoicesSection();
		await loadOffersSection();

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
		portalInviteError = '';
		isSendingPortalInvite = true;
		try {
			const response = await apiFetchWithSession(portalInviteURL(), { method: 'POST' });
			if (!response.ok) {
				portalInviteError = await apiErrorMessage(response);
				return;
			}

			const created: { inviteToken: string } = await response.json();
			portalInviteLink = `${location.origin}/portal/accept-invite?token=${created.inviteToken}`;
		} catch (error_) {
			portalInviteError = error_ instanceof Error ? error_.message : 'Failed to send portal invite';
		} finally {
			isSendingPortalInvite = false;
		}
	}

	async function handleCreateVisit() {
		visitsError = '';
		isCreatingVisit = true;
		try {
			const response = await apiFetchWithSession(visitsURL(), { method: 'POST' });
			if (!response.ok) {
				visitsError = await response.text();
				return;
			}

			await loadVisits();
		} catch (error_) {
			visitsError = error_ instanceof Error ? error_.message : 'Failed to add Visit';
		} finally {
			isCreatingVisit = false;
		}
	}

	async function handleReassign(visitId: string, event: SubmitEvent) {
		event.preventDefault();
		reassignError[visitId] = '';
		try {
			const response = await apiFetchWithSession(`${visitsURL()}/${visitId}`, {
				method: 'PATCH',
				headers: { 'Content-Type': 'application/json' },
				body: JSON.stringify({ staffId: reassignStaffId[visitId] ?? '' })
			});
			if (!response.ok) {
				reassignError[visitId] = await response.text();
				return;
			}

			reassignStaffId[visitId] = '';
			await loadVisits();
		} catch (error_) {
			reassignError[visitId] = error_ instanceof Error ? error_.message : 'Failed to reassign Visit';
		}
	}

	async function handleLoadOlderMessages() {
		messagesError = '';
		isLoadingOlderMessages = true;
		try {
			const response = await apiFetchWithSession(`${messagesURL()}?cursor=${encodeURIComponent(messagesCursor)}`);
			if (!response.ok) {
				messagesError = await response.text();
				return;
			}

			const data = await response.json();
			messages = [...data.items.toReversed(), ...messages];
			messagesCursor = data.nextCursor ?? '';
			isMessagesHasMore = data.hasMore;
			await loadAttachmentPreviews(messages);
		} catch (error_) {
			messagesError = error_ instanceof Error ? error_.message : 'Failed to load older messages';
		} finally {
			isLoadingOlderMessages = false;
		}
	}

	async function didSendMessage(body: string, attachment: File | undefined): Promise<boolean> {
		messagesError = '';
		isSendingMessage = true;
		try {
			let response: Response;
			if (attachment) {
				const form = new FormData();
				form.set('body', body);
				form.set('attachment', attachment);
				response = await apiFetchWithSession(messagesURL(), { method: 'POST', body: form });
			} else {
				response = await apiFetchWithSession(messagesURL(), {
					method: 'POST',
					headers: { 'Content-Type': 'application/json' },
					body: JSON.stringify({ body })
				});
			}
			if (!response.ok) {
				messagesError = await response.text();
				return false;
			}

			const created = await response.json();
			messages = [...messages, created];
			await loadAttachmentPreviews([created]);
			return true;
		} catch (error_) {
			messagesError = error_ instanceof Error ? error_.message : 'Failed to send message';
			return false;
		} finally {
			isSendingMessage = false;
		}
	}

	async function handleDownloadAttachment(messageId: string, filename: string) {
		const response = await apiFetchWithSession(`${messagesURL()}/${messageId}/attachment`);
		if (!response.ok) {
			messagesError = await response.text();
			return;
		}

		const blob = await response.blob();
		const url = URL.createObjectURL(blob);
		const link = document.createElement('a');
		link.href = url;
		link.download = filename;
		link.click();
		URL.revokeObjectURL(url);
	}
</script>

{#snippet summary()}
	<stack-l space="var(--space-4)">
		<DescriptionList items={summaryItems(detail!)} />

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

{#snippet reassignAction(visit: Visit)}
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
			describedBy="visit-{visit.visitId}-reassign-name"
		/>
		<span class="visually-hidden" id="visit-{visit.visitId}-reassign-name"
			>{visit.staffName}, {new Date(visit.createdAt).toLocaleDateString()}</span
		>
	</form>
	{#if reassignError[visit.visitId]}
		<Notice variant="error" message={reassignError[visit.visitId]} />
	{/if}
{/snippet}

{#snippet visitsSection()}
	<Button label="Add a Visit" onClick={handleCreateVisit} loading={isCreatingVisit} />

	{#if visitsError}
		<Notice variant="error" message={visitsError} />
	{/if}

	<DataTable
		columns={[
			{ label: 'Staff', accessor: (visit: Visit) => visit.staffName },
			{ label: 'Date', accessor: (visit: Visit) => new Date(visit.createdAt).toLocaleDateString() }
		]}
		rows={visits}
		rowActions={{ label: 'Reassign', content: reassignAction }}
		hasMore={isMoreVisitsAvailable}
		onLoadMore={handleLoadMoreVisits}
		isLoadingMore={isLoadingMoreVisits}
		loadMoreError={loadMoreVisitsError}
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
	{#if planError[planType]}
		<Notice variant="error" message={planError[planType]} />
	{/if}

	{#if planLoaded[planType]}
		{#if planInstances[planType]}
			<PlanInstanceForm
				fields={planInstances[planType]!.fields}
				answers={planInstances[planType]!.answers}
				onAnswerChange={(fieldId, value) => handlePlanAnswerChange(planType, fieldId, value)}
				onToggleOption={(fieldId, option) => handlePlanToggleOption(planType, fieldId, option)}
			/>
			<Button label="Save {heading}" onClick={() => handleSavePlan(planType)} loading={planBusy[planType]} />
		{:else}
			<Button label="Create {heading}" onClick={() => handleCreatePlan(planType)} loading={planBusy[planType]} />
		{/if}
	{/if}
{/snippet}

{#snippet carePlanSection()}
	{@render planSectionBody('care_plan', 'Care Plan')}
{/snippet}

{#snippet birthPlanSection()}
	{@render planSectionBody('birth_plan', 'Birth Plan')}
{/snippet}

{#snippet contractSection()}
	{#if contractError}
		<Notice variant="error" message={contractError} />
	{/if}

	{#if isContractLoaded}
		{#if contract}
			<ContractStatus status={contract.status} onVoid={handleVoidContract} />
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
	Archetype D, ADR-0018. The Invoices and Offers sections are still
	conditional exactly as before -- Invoices needs a Contract to exist
	and Offers is Owner/Admin-only -- which is why `sections` is a typed
	array and not a run of named regions.
-->
<RecordDetail
	title={detail ? detail.clientName : ''}
	{summary}
	{actions}
	isContentsShown
	sections={detail
		? [
				{ heading: 'Visits', content: visitsSection },
				{ heading: 'Care Plan', content: carePlanSection },
				{ heading: 'Birth Plan', content: birthPlanSection },
				{ heading: 'Contract', content: contractSection },
				...(contract ? [{ heading: 'Invoices', content: invoicesSection }] : []),
				...(isOffersVisible ? [{ heading: 'Offers', content: offersSection }] : []),
				{ heading: 'Messages', content: messagesSection }
			]
		: []}
	loading={detail || error ? undefined : 'Loading the Engagement'}
	loadError={error || undefined}
/>
