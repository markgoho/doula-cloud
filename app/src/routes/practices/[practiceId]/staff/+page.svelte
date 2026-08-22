<script lang="ts">
	import { onMount } from 'svelte';
	import { page } from '$app/state';
	import { apiFetchWithSession } from '#lib/api.js';
	import DataTable from '#lib/components/organisms/DataTable.svelte';
	import Heading from '#lib/components/atoms/Heading.svelte';
	import Text from '#lib/components/atoms/Text.svelte';
	import Notice from '#lib/components/atoms/Notice.svelte';
	import Button from '#lib/components/atoms/Button.svelte';

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

	const columns = [
		{ label: 'Name', accessor: (member: StaffSummary) => member.name },
		{ label: 'Email', accessor: (member: StaffSummary) => member.email },
		{
			label: 'Roles',
			accessor: (member: StaffSummary) => member.roles.join(', ') || 'no roles yet'
		}
	];

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

{#snippet staffActions(member: StaffSummary)}
	<Button
		label="End sessions everywhere"
		variant="secondary"
		size="sm"
		onClick={() => handleEndSessions(member.staffId)}
		loading={endingSessionsFor[member.staffId]}
	/>
	{#if endSessionsDone[member.staffId]}
		<Notice variant="status" message="Sessions ended." />
	{/if}
	{#if endSessionsError[member.staffId]}
		<Notice variant="error" message={endSessionsError[member.staffId]} />
	{/if}
{/snippet}

<Heading level={1} text="Staff" />

{#if error}
	<Notice variant="error" message={error} />
{:else if isLoaded}
	{#if staff.length === 0}
		<Text text="No Staff yet." />
	{:else}
		<DataTable
			{columns}
			rows={staff}
			rowActions={{ label: 'Actions', content: staffActions }}
			emptyMessage="No Staff yet."
		/>
	{/if}
{/if}
