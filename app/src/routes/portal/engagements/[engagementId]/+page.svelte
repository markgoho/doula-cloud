<script lang="ts">
	import { onMount } from 'svelte';
	import { goto } from '$app/navigation';
	import { page } from '$app/state';
	import { resolve } from '$app/paths';
	import { getFirebaseAuth } from '#lib/firebase.js';
	import { apiFetch } from '#lib/api.js';

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
	let sendingMessage = $state(false);

	function messagesURL() {
		return `/api/portal/engagements/${page.params.engagementId}/messages`;
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

			const response = await apiFetch(messagesURL(), idToken, {
				method: 'POST',
				headers: { 'Content-Type': 'application/json' },
				body: JSON.stringify({ body: newMessageBody })
			});
			if (!response.ok) {
				messagesError = await response.text();
				return;
			}

			const created = await response.json();
			messages = [...messages, created];
			newMessageBody = '';
		} catch (err) {
			messagesError = err instanceof Error ? err.message : 'Failed to send message';
		} finally {
			sendingMessage = false;
		}
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
					<p>{message.body}</p>
				</li>
			{/each}
		</ul>
	{/if}

	<form onsubmit={handleSendMessage}>
		<label>
			Message
			<textarea bind:value={newMessageBody} required></textarea>
		</label>
		<button type="submit" disabled={sendingMessage}>Send</button>
	</form>
{/if}
