<script lang="ts">
	import { onDestroy, onMount } from 'svelte';
	import { goto } from '$app/navigation';
	import { page } from '$app/state';
	import { resolve } from '$app/paths';
	import { getFirebaseAuth } from '#lib/firebase.js';
	import { apiFetch } from '#lib/api.js';
	import { registerPushSubscription } from '#lib/pushRegistration.js';
	import { subscribeToThreadPushMessages } from '#lib/pushRefresh.js';

	type Detail = {
		engagementId: string;
		practiceName: string;
		status: string;
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

	let detail = $state<Detail | null>(null);
	let error = $state('');

	let messages = $state<Message[]>([]);
	let messagesError = $state('');
	let messagesCursor = $state('');
	let messagesHasMore = $state(false);
	let loadingOlderMessages = $state(false);
	let newMessageBody = $state('');
	let newMessageAttachment = $state<File | null>(null);
	let sendingMessage = $state(false);
	// Object URLs for image attachments, keyed by messageId, so images
	// render inline in the thread (not just downloadable) -- fetched via
	// apiFetch since the attachment endpoint requires the caller's auth
	// header, which a plain <img src> can't send.
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

	async function loadAttachmentPreviews(idToken: string, items: Message[]) {
		await Promise.all(
			items
				.filter((m) => m.attachmentContentType?.startsWith('image/') && !attachmentPreviewURLs[m.messageId])
				.map(async (m) => {
					const response = await apiFetch(`${messagesURL()}/${m.messageId}/attachment`, idToken);
					if (!response.ok) return;
					const blob = await response.blob();
					attachmentPreviewURLs[m.messageId] = URL.createObjectURL(blob);
				})
		);
	}

	async function loadMessages(idToken: string) {
		const response = await apiFetch(messagesURL(), idToken);
		if (!response.ok) {
			messagesError = await response.text();
			return;
		}
		const data = await response.json();
		messages = [...data.items].reverse();
		messagesCursor = data.nextCursor ?? '';
		messagesHasMore = data.hasMore;
		await loadAttachmentPreviews(idToken, messages);
	}

	onMount(async () => {
		const user = getFirebaseAuth().currentUser;
		if (!user) {
			await goto(resolve('/portal/login'));
			return;
		}

		const idToken = await user.getIdToken();
		const response = await apiFetch(`/api/portal/engagements/${page.params.engagementId}`, idToken);
		if (!response.ok) {
			error = await response.text();
			return;
		}

		detail = await response.json();
		await loadMessages(idToken);

		// Fire-and-forget: #61's "once per device after login" push
		// registration is best-effort and must never block landing on the
		// thread (see pushRegistration.ts's doc comment).
		void registerPushSubscription(
			`/api/portal/engagements/${page.params.engagementId}/push-subscriptions`,
			idToken
		);

		// #61: an open service worker push message ("a new Message arrived
		// on this Engagement") triggers a refetch -- see push.ts's
		// PUSH_MESSAGE_TYPE doc comment for why the service worker can't
		// just fetch this itself.
		unsubscribePushMessages = subscribeToThreadPushMessages(page.params.engagementId!, () => {
			void (async () => {
				const current = getFirebaseAuth().currentUser;
				if (!current) return;
				await loadMessages(await current.getIdToken());
			})();
		});
	});

	async function handleLoadOlderMessages() {
		messagesError = '';
		loadingOlderMessages = true;
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
			messages = [...[...data.items].reverse(), ...messages];
			messagesCursor = data.nextCursor ?? '';
			messagesHasMore = data.hasMore;
			await loadAttachmentPreviews(idToken, messages);
		} catch (err) {
			messagesError = err instanceof Error ? err.message : 'Failed to load older messages';
		} finally {
			loadingOlderMessages = false;
		}
	}

	async function handleSendMessage(event: SubmitEvent) {
		event.preventDefault();
		messagesError = '';
		sendingMessage = true;
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
			newMessageAttachment = null;
			await loadAttachmentPreviews(idToken, [created]);
		} catch (err) {
			messagesError = err instanceof Error ? err.message : 'Failed to send message';
		} finally {
			sendingMessage = false;
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
	<h1>Welcome to {detail.practiceName}</h1>
	<dl>
		<dt>Status</dt>
		<dd>{detail.status}</dd>
		<dt>Created</dt>
		<dd>{new Date(detail.createdAt).toLocaleDateString()}</dd>
	</dl>

	<h2>Messages</h2>

	{#if messagesError}
		<p role="alert">{messagesError}</p>
	{/if}

	{#if messagesHasMore}
		<button type="button" onclick={handleLoadOlderMessages} disabled={loadingOlderMessages}>
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
				onchange={(event) => (newMessageAttachment = event.currentTarget.files?.[0] ?? null)}
			/>
		</label>
		<button type="submit" disabled={sendingMessage}>Send</button>
	</form>
{/if}
