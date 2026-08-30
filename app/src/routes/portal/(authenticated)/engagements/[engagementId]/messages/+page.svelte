<script lang="ts">
	/*
	 * The Client's side of the message thread, on its own route.
	 *
	 * It used to render inside the portal hub. Once Messages became a
	 * destination in the portal's nav (#431) the thread needed one home,
	 * and a nav item pointing at a section of another page is a nav item
	 * that lies about where it goes.
	 */
	import { onDestroy, onMount } from 'svelte';
	import { page } from '$app/state';
	import { apiFetchWithSession } from '#lib/api.js';
	import { subscribeToThreadPushMessages } from '#lib/pushRefresh.js';
	import MessageThread, { type Message } from '#lib/components/organisms/MessageThread.svelte';
	import RecordDetail from '#lib/components/templates/RecordDetail.svelte';

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
					(m) =>
						m.attachmentContentType?.startsWith('image/') &&
						!Object.hasOwn(attachmentPreviewURLs, m.messageId)
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
		await loadMessages();

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
			const response = await apiFetchWithSession(
				`${messagesURL()}?cursor=${encodeURIComponent(messagesCursor)}`
			);
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

{#snippet thread()}
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
	Archetype D with one section, and no contents region: a contents list
	above a single section is furniture (ADR-0018).
-->
<RecordDetail
	title="Messages"
	serviceName={page.data.practiceName}
	sections={[{ heading: 'Your thread', content: thread }]}
/>
