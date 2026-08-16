<script lang="ts">
	import { onDestroy, onMount } from 'svelte';
	import { goto } from '$app/navigation';
	import { page } from '$app/state';
	import { resolve } from '$app/paths';
	import { getFirebaseAuth } from '#lib/firebase.js';
	import { apiFetch } from '#lib/api.js';
	import { subscribeToThreadPushMessages } from '#lib/pushRefresh.js';
	import PlanInstanceForm from '#lib/PlanInstanceForm.svelte';
	import {
		loadInstance,
		createInstance,
		saveAnswers,
		setAnswer,
		toggleMultiSelectOption,
		type Instance,
		type Fetcher
	} from '#lib/planInstance.js';

	type Detail = {
		engagementId: string;
		clientId: string;
		clientName: string;
		status: string;
		createdAt: string;
	};

	type Visit = {
		visitId: string;
		staffId: string;
		staffName: string;
		createdAt: string;
	};

	type Message = {
		messageId: string;
		senderType: 'staff' | 'client';
		senderId: string;
		senderName: string;
		body: string;
		attachmentFilename?: string;
		attachmentContentType?: string;
		createdAt: string;
	};

	let detail = $state<Detail | undefined>();
	let error = $state('');

	let visits = $state<Visit[]>([]);
	let visitsError = $state('');
	let isCreatingVisit = $state(false);
	let reassignStaffId = $state<Record<string, string>>({});
	let reassignError = $state<Record<string, string>>({});

	let messages = $state<Message[]>([]);
	let messagesError = $state('');
	let messagesCursor = $state('');
	let isMessagesHasMore = $state(false);
	let isLoadingOlderMessages = $state(false);
	let newMessageBody = $state('');
	let newMessageAttachment = $state<File | undefined>();
	let isSendingMessage = $state(false);
	// Object URLs for image attachments, keyed by messageId, so images
	// render inline in the thread (not just downloadable) -- fetched via
	// apiFetch since the attachment endpoint requires the caller's auth
	// header, which a plain <img src> can't send.
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

	onDestroy(() => {
		for (const url of Object.values(attachmentPreviewURLs)) {
			URL.revokeObjectURL(url);
		}
		unsubscribePushMessages();
	});

	async function loadAttachmentPreviews(idToken: string, items: Message[]) {
		await Promise.all(
			items
				.filter(
					(m) => m.attachmentContentType?.startsWith('image/') && !Object.hasOwn(attachmentPreviewURLs, m.messageId)
				)
				.map(async (m) => {
					const response = await apiFetch(`${messagesURL()}/${m.messageId}/attachment`, idToken);
					if (!response.ok) return;
					const blob = await response.blob();
					attachmentPreviewURLs[m.messageId] = URL.createObjectURL(blob);
				})
		);
	}

	function visitsURL() {
		return `/api/practices/${page.params.practiceId}/engagements/${page.params.engagementId}/visits`;
	}

	async function loadVisits(idToken: string) {
		const response = await apiFetch(visitsURL(), idToken);
		if (!response.ok) {
			visitsError = await response.text();
			return;
		}
		visits = await response.json();
	}

	function messagesURL() {
		return `/api/practices/${page.params.practiceId}/engagements/${page.params.engagementId}/messages`;
	}

	async function loadMessages(idToken: string) {
		const response = await apiFetch(messagesURL(), idToken);
		if (!response.ok) {
			messagesError = await response.text();
			return;
		}
		const data = await response.json();
		messages = data.items.toReversed();
		messagesCursor = data.nextCursor ?? '';
		isMessagesHasMore = data.hasMore;
		await loadAttachmentPreviews(idToken, messages);
	}

	function planFetcher(idToken: string): Fetcher {
		return (path, init) => apiFetch(path, idToken, init);
	}

	async function loadPlan(idToken: string, planType: PlanType) {
		planError[planType] = '';
		try {
			planInstances[planType] = await loadInstance(
				planFetcher(idToken),
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
			const user = getFirebaseAuth().currentUser;
			if (!user) {
				planError[planType] = 'You must be logged in to create a plan';
				return;
			}
			const idToken = await user.getIdToken();
			planInstances[planType] = await createInstance(
				planFetcher(idToken),
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
			const user = getFirebaseAuth().currentUser;
			if (!user) {
				planError[planType] = 'You must be logged in to save a plan';
				return;
			}
			const idToken = await user.getIdToken();
			planInstances[planType] = await saveAnswers(
				planFetcher(idToken),
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

	onMount(async () => {
		const user = getFirebaseAuth().currentUser;
		if (!user) {
			await goto(resolve('/login'));
			return;
		}

		const idToken = await user.getIdToken();
		const response = await apiFetch(
			`/api/practices/${page.params.practiceId}/engagements/${page.params.engagementId}`,
			idToken
		);
		if (!response.ok) {
			error = await response.text();
			return;
		}

		detail = await response.json();
		await loadVisits(idToken);
		await loadMessages(idToken);
		await Promise.all(planSections.map((section) => loadPlan(idToken, section.type)));

		// #61: an open service worker push message ("a new Message arrived
		// on this Engagement") triggers a refetch, the same content-free
		// "push wakes the client, which fetches the real content" delivery
		// ADR-0002 describes -- see push.ts's PUSH_MESSAGE_TYPE doc comment
		// for why the service worker can't just fetch this itself.
		unsubscribePushMessages = subscribeToThreadPushMessages(page.params.engagementId!, () => {
			void (async () => {
				const current = getFirebaseAuth().currentUser;
				if (!current) return;
				await loadMessages(await current.getIdToken());
			})();
		});
	});

	async function handleCreateVisit() {
		visitsError = '';
		isCreatingVisit = true;
		try {
			const user = getFirebaseAuth().currentUser;
			if (!user) {
				visitsError = 'You must be logged in to add a Visit';
				return;
			}
			const idToken = await user.getIdToken();

			const response = await apiFetch(visitsURL(), idToken, { method: 'POST' });
			if (!response.ok) {
				visitsError = await response.text();
				return;
			}

			await loadVisits(idToken);
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
			const user = getFirebaseAuth().currentUser;
			if (!user) {
				reassignError[visitId] = 'You must be logged in to reassign a Visit';
				return;
			}
			const idToken = await user.getIdToken();

			const response = await apiFetch(`${visitsURL()}/${visitId}`, idToken, {
				method: 'PATCH',
				headers: { 'Content-Type': 'application/json' },
				body: JSON.stringify({ staffId: reassignStaffId[visitId] ?? '' })
			});
			if (!response.ok) {
				reassignError[visitId] = await response.text();
				return;
			}

			reassignStaffId[visitId] = '';
			await loadVisits(idToken);
		} catch (error_) {
			reassignError[visitId] = error_ instanceof Error ? error_.message : 'Failed to reassign Visit';
		}
	}

	async function handleLoadOlderMessages() {
		messagesError = '';
		isLoadingOlderMessages = true;
		try {
			const user = getFirebaseAuth().currentUser;
			if (!user) {
				messagesError = 'You must be logged in to load messages';
				return;
			}
			const idToken = await user.getIdToken();

			const response = await apiFetch(
				`${messagesURL()}?cursor=${encodeURIComponent(messagesCursor)}`,
				idToken
			);
			if (!response.ok) {
				messagesError = await response.text();
				return;
			}

			const data = await response.json();
			messages = [...data.items.toReversed(), ...messages];
			messagesCursor = data.nextCursor ?? '';
			isMessagesHasMore = data.hasMore;
			await loadAttachmentPreviews(idToken, messages);
		} catch (error_) {
			messagesError = error_ instanceof Error ? error_.message : 'Failed to load older messages';
		} finally {
			isLoadingOlderMessages = false;
		}
	}

	async function handleSendMessage(event: SubmitEvent) {
		event.preventDefault();
		messagesError = '';
		isSendingMessage = true;
		try {
			const user = getFirebaseAuth().currentUser;
			if (!user) {
				messagesError = 'You must be logged in to send a message';
				return;
			}
			const idToken = await user.getIdToken();

			let response: Response;
			if (newMessageAttachment) {
				const form = new FormData();
				form.set('body', newMessageBody);
				form.set('attachment', newMessageAttachment);
				response = await apiFetch(messagesURL(), idToken, { method: 'POST', body: form });
			} else {
				response = await apiFetch(messagesURL(), idToken, {
					method: 'POST',
					headers: { 'Content-Type': 'application/json' },
					body: JSON.stringify({ body: newMessageBody })
				});
			}
			if (!response.ok) {
				messagesError = await response.text();
				return;
			}

			const created = await response.json();
			messages = [...messages, created];
			newMessageBody = '';
			newMessageAttachment = undefined;
			await loadAttachmentPreviews(idToken, [created]);
		} catch (error_) {
			messagesError = error_ instanceof Error ? error_.message : 'Failed to send message';
		} finally {
			isSendingMessage = false;
		}
	}

	async function handleDownloadAttachment(messageId: string, filename: string) {
		const user = getFirebaseAuth().currentUser;
		if (!user) {
			messagesError = 'You must be logged in to download an attachment';
			return;
		}
		const idToken = await user.getIdToken();

		const response = await apiFetch(`${messagesURL()}/${messageId}/attachment`, idToken);
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

{#if error}
	<p role="alert">{error}</p>
{:else if detail}
	<h1>{detail.clientName}</h1>
	<dl>
		<dt>Status</dt>
		<dd>{detail.status}</dd>
		<dt>Created</dt>
		<dd>{new Date(detail.createdAt).toLocaleDateString()}</dd>
	</dl>

	<h2>Visits</h2>

	<button type="button" onclick={handleCreateVisit} disabled={isCreatingVisit}>Add a Visit</button>

	{#if visitsError}
		<p role="alert">{visitsError}</p>
	{/if}

	{#if visits.length === 0}
		<p>No Visits yet.</p>
	{:else}
		<ul>
			{#each visits as visit (visit.visitId)}
				<li>
					{visit.staffName} — {new Date(visit.createdAt).toLocaleDateString()}
					<form onsubmit={(event) => handleReassign(visit.visitId, event)}>
						<label>
							Reassign to Staff id
							<input type="text" bind:value={reassignStaffId[visit.visitId]} required />
						</label>
						<button type="submit">Reassign</button>
					</form>
					{#if reassignError[visit.visitId]}
						<p role="alert">{reassignError[visit.visitId]}</p>
					{/if}
				</li>
			{/each}
		</ul>
	{/if}

	{#each planSections as section (section.type)}
		<h2>{section.heading}</h2>

		{#if planError[section.type]}
			<p role="alert">{planError[section.type]}</p>
		{/if}

		{#if planLoaded[section.type]}
			{#if planInstances[section.type]}
				<PlanInstanceForm
					fields={planInstances[section.type]!.fields}
					answers={planInstances[section.type]!.answers}
					onAnswerChange={(fieldId, value) => handlePlanAnswerChange(section.type, fieldId, value)}
					onToggleOption={(fieldId, option) => handlePlanToggleOption(section.type, fieldId, option)}
				/>
				<button type="button" onclick={() => handleSavePlan(section.type)} disabled={planBusy[section.type]}>
					Save {section.heading}
				</button>
			{:else}
				<button type="button" onclick={() => handleCreatePlan(section.type)} disabled={planBusy[section.type]}>
					Create {section.heading}
				</button>
			{/if}
		{/if}
	{/each}

	<h2>Messages</h2>

	{#if messagesError}
		<p role="alert">{messagesError}</p>
	{/if}

	{#if isMessagesHasMore}
		<button type="button" onclick={handleLoadOlderMessages} disabled={isLoadingOlderMessages}>
			Load older messages
		</button>
	{/if}

	{#if messages.length === 0}
		<p>No messages yet.</p>
	{:else}
		<ul>
			{#each messages as message (message.messageId)}
				<li>
					<strong>{message.senderName}</strong> ({message.senderType}) —
					{new Date(message.createdAt).toLocaleString()}
					{#if message.body}
						<p>{message.body}</p>
					{/if}
					{#if message.attachmentFilename}
						{#if attachmentPreviewURLs[message.messageId]}
							<img
								src={attachmentPreviewURLs[message.messageId]}
								alt={message.attachmentFilename}
								style="max-width: 240px; max-height: 240px; display: block;"
							/>
						{/if}
						<button
							type="button"
							onclick={() => handleDownloadAttachment(message.messageId, message.attachmentFilename ?? '')}
						>
							📎 {message.attachmentFilename}
						</button>
					{/if}
				</li>
			{/each}
		</ul>
	{/if}

	<form onsubmit={handleSendMessage}>
		<label>
			Message
			<textarea bind:value={newMessageBody}></textarea>
		</label>
		<label>
			Attachment (image or PDF, up to 10MB)
			<input
				type="file"
				accept="image/*,application/pdf"
				onchange={(event) => (newMessageAttachment = event.currentTarget.files?.[0])}
			/>
		</label>
		<button type="submit" disabled={isSendingMessage}>Send</button>
	</form>
{/if}
