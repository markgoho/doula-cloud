<script lang="ts">
	import { onMount } from 'svelte';
	import { goto } from '$app/navigation';
	import { page } from '$app/state';
	import { resolve } from '$app/paths';
	import { getFirebaseAuth } from '#lib/firebase.js';
	import { apiFetch } from '#lib/api.js';

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
		createdAt: string;
	};

	let detail = $state<Detail | null>(null);
	let error = $state('');

	let visits = $state<Visit[]>([]);
	let visitsError = $state('');
	let creatingVisit = $state(false);
	let reassignStaffId = $state<Record<string, string>>({});
	let reassignError = $state<Record<string, string>>({});

	let messages = $state<Message[]>([]);
	let messagesError = $state('');
	let messagesCursor = $state('');
	let messagesHasMore = $state(false);
	let loadingOlderMessages = $state(false);
	let newMessageBody = $state('');
	let sendingMessage = $state(false);

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
		messages = [...data.items].reverse();
		messagesCursor = data.nextCursor ?? '';
		messagesHasMore = data.hasMore;
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
	});

	async function handleCreateVisit() {
		visitsError = '';
		creatingVisit = true;
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
		} catch (err) {
			visitsError = err instanceof Error ? err.message : 'Failed to add Visit';
		} finally {
			creatingVisit = false;
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
		} catch (err) {
			reassignError[visitId] = err instanceof Error ? err.message : 'Failed to reassign Visit';
		}
	}

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
	<h1>{detail.clientName}</h1>
	<dl>
		<dt>Status</dt>
		<dd>{detail.status}</dd>
		<dt>Created</dt>
		<dd>{new Date(detail.createdAt).toLocaleDateString()}</dd>
	</dl>

	<h2>Visits</h2>

	<button type="button" onclick={handleCreateVisit} disabled={creatingVisit}>Add a Visit</button>

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
