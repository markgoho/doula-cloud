<script lang="ts">
	import { onDestroy, onMount } from 'svelte';
	import { page } from '$app/state';
	import { resolve } from '$app/paths';
	import { apiBaseURL, apiFetchWithSession } from '#lib/api.js';
	import {
		portalPushSubscriptionsPath,
		registerPushSubscription
	} from '#lib/pushRegistration.js';
	import { subscribeToThreadPushMessages } from '#lib/pushRefresh.js';
	import MessageThread, { type Message } from '#lib/components/organisms/MessageThread.svelte';
	import Notice from '#lib/components/atoms/Notice.svelte';
	import Link from '#lib/components/atoms/Link.svelte';
	import Skeleton from '#lib/components/atoms/Skeleton.svelte';
	import DescriptionList from '#lib/components/molecules/DescriptionList.svelte';
	import RecordDetail from '#lib/components/templates/RecordDetail.svelte';

	type Detail = {
		engagementId: string;
		practiceName: string;
		status: string;
		createdAt: string;
	};

	let detail = $state<Detail | undefined>();
	let error = $state('');

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

	onDestroy(() => {
		for (const url of Object.values(attachmentPreviewURLs)) {
			URL.revokeObjectURL(url);
		}
		unsubscribePushMessages();
	});

	function messagesURL() {
		return `/api/portal/engagements/${page.params.engagementId}/messages`;
	}

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

	onMount(async () => {
		const response = await apiFetchWithSession(`/api/portal/engagements/${page.params.engagementId}`);
		if (!response.ok) {
			error = await response.text();
			return;
		}

		detail = await response.json();
		await loadMessages();

		// Fire-and-forget: #61's "once per device after login" push
		// registration is best-effort and must never block landing on the
		// thread (see pushRegistration.ts's doc comment) -- a plain
		// credentialed fetch, not apiFetchWithSession, since that helper's
		// own 401 handling would sign the person out and redirect on a
		// failure this call is supposed to swallow silently.
		void registerPushSubscription(
			portalPushSubscriptionsPath(page.params.engagementId!),
			(path, init) => fetch(apiBaseURL() + path, { ...init, credentials: 'include' })
		);

		// #61: an open service worker push message ("a new Message arrived
		// on this Engagement") triggers a refetch -- see push.ts's
		// PUSH_MESSAGE_TYPE doc comment for why the service worker can't
		// just fetch this itself.
		unsubscribePushMessages = subscribeToThreadPushMessages(page.params.engagementId!, () => {
			void loadMessages();
		});
	});

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
	<DescriptionList
		items={[
			{ label: 'Status', value: detail!.status },
			{ label: 'Created', value: new Date(detail!.createdAt).toLocaleDateString() }
		]}
	/>
{/snippet}

{#snippet actions()}
	<Link
		href={resolve('/portal/(authenticated)/engagements/[engagementId]/birth-plan', { engagementId: page.params.engagementId! })}
		label="Birth Plan"
	/>
	<Link
		href={resolve('/portal/(authenticated)/engagements/[engagementId]/contract', { engagementId: page.params.engagementId! })}
		label="Contract"
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

{#if error}
	<Notice variant="error" message={error} />
{:else if detail}
	<!--
		Archetype D, ADR-0018 -- the same Template the staff Engagement page
		uses, which is the point of putting both on it. No contents region:
		one section is not a page you scroll to search, and a contents list
		above three sections is furniture.
	-->
	<RecordDetail
		title={`Welcome to ${detail.practiceName}`}
		{summary}
		{actions}
		sections={[{ heading: 'Messages', content: messagesSection }]}
	/>
{:else}
	<Skeleton variant="text" lines={5} label="Loading your Engagement" />
{/if}
