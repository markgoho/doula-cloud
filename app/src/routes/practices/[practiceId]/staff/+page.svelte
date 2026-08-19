<script lang="ts">
	import { onMount } from 'svelte';
	import { page } from '$app/state';
	import { apiFetchWithSession } from '#lib/api.js';

	type StaffSummary = {
		staffId: string;
		name: string;
		email: string;
		roles: string[];
	};

	let staff = $state<StaffSummary[]>([]);
	let error = $state('');
	let isLoaded = $state(false);
	let endingSessionsFor = $state<Record<string, boolean>>({});
	let endSessionsError = $state<Record<string, string>>({});
	let endSessionsDone = $state<Record<string, boolean>>({});

	onMount(async () => {
		const response = await apiFetchWithSession(`/api/practices/${page.params.practiceId}/staff`);
		if (!response.ok) {
			error = await response.text();
			return;
		}

		staff = await response.json();
		isLoaded = true;
	});

	// Offboarding or a lost device: ends every session that Staff member
	// holds, on every device, at once -- not the same thing as sign-out,
	// which only ends the browser making the request (#154).
	async function handleEndSessions(staffId: string) {
		endSessionsError[staffId] = '';
		endSessionsDone[staffId] = false;
		endingSessionsFor[staffId] = true;
		try {
			const response = await apiFetchWithSession(
				`/api/practices/${page.params.practiceId}/staff/${staffId}/sessions`,
				{ method: 'DELETE' }
			);
			if (!response.ok) {
				endSessionsError[staffId] = await response.text();
				return;
			}
			endSessionsDone[staffId] = true;
		} catch (error_) {
			endSessionsError[staffId] =
				error_ instanceof Error ? error_.message : 'Failed to end sessions';
		} finally {
			endingSessionsFor[staffId] = false;
		}
	}
</script>

<h1>Staff</h1>

{#if error}
	<p role="alert">{error}</p>
{:else if isLoaded}
	{#if staff.length === 0}
		<p>No Staff yet.</p>
	{:else}
		<ul>
			{#each staff as member (member.staffId)}
				<li>
					{member.name} — {member.email} — {member.roles.join(', ') || 'no roles yet'}
					<button
						type="button"
						onclick={() => handleEndSessions(member.staffId)}
						disabled={endingSessionsFor[member.staffId]}
					>
						End sessions everywhere
					</button>
					{#if endSessionsDone[member.staffId]}
						<span role="status">Sessions ended.</span>
					{/if}
					{#if endSessionsError[member.staffId]}
						<span role="alert">{endSessionsError[member.staffId]}</span>
					{/if}
				</li>
			{/each}
		</ul>
	{/if}
{/if}
