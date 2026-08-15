<script lang="ts">
	import { onMount } from 'svelte';
	import { goto } from '$app/navigation';
	import { page } from '$app/state';
	import { resolve } from '$app/paths';
	import { getFirebaseAuth } from '$lib/firebase';
	import { apiFetch } from '$lib/api';

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

	let detail = $state<Detail | null>(null);
	let error = $state('');

	let visits = $state<Visit[]>([]);
	let visitsError = $state('');
	let creatingVisit = $state(false);
	let reassignStaffId = $state<Record<string, string>>({});
	let reassignError = $state<Record<string, string>>({});

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
{/if}
