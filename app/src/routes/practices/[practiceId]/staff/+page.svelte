<script lang="ts">
	import { onMount } from 'svelte';
	import { page } from '$app/state';
	import { apiFetchWithSession } from '#lib/api.js';
	import DataTable from '#lib/components/organisms/DataTable.svelte';
	import Heading from '#lib/components/atoms/Heading.svelte';
	import Text from '#lib/components/atoms/Text.svelte';
	import Notice from '#lib/components/atoms/Notice.svelte';
	import Badge from '#lib/components/atoms/Badge.svelte';
	import Button from '#lib/components/atoms/Button.svelte';
	import MembershipFields from '#lib/components/molecules/MembershipFields.svelte';

	type StaffSummary = {
		staffId: string;
		name: string;
		email: string;
		roles: string[];
		employmentType: 'employee' | 'contractor';
	};

	type InvitationSummary = {
		invitationId: string;
		address: string;
		roles: string[];
		employmentType: 'employee' | 'contractor';
		expiresAt: string;
		deliveryFailed: boolean;
	};

	type Roster = { members: StaffSummary[]; invitations: InvitationSummary[] };

	let members = $state<StaffSummary[]>([]);
	let invitations = $state<InvitationSummary[]>([]);
	let error = $state('');
	let isLoaded = $state(false);

	let endingSessionsFor = $state<Record<string, boolean>>({});
	let endSessionsError = $state<Record<string, string>>({});
	let endSessionsDone = $state<Record<string, boolean>>({});

	// One Membership is edited at a time, in place on its own row: roles
	// and employment type on one form (RA-G2, #261), not two round trips.
	let editingStaffId = $state('');
	let editRoles = $state<string[]>([]);
	let editEmploymentType = $state<'employee' | 'contractor'>('employee');
	let isSavingEdit = $state(false);
	let editError = $state('');

	let revokingInvitationId = $state('');
	let revokeError = $state<Record<string, string>>({});

	const memberColumns = [
		{ label: 'Name', accessor: (member: StaffSummary) => member.name },
		{ label: 'Email', accessor: (member: StaffSummary) => member.email },
		{
			label: 'Roles',
			accessor: (member: StaffSummary) => member.roles.join(', ') || 'no roles yet'
		},
		{ label: 'Employment type', accessor: (member: StaffSummary) => member.employmentType }
	];

	const invitationColumns = [
		{ label: 'Email', accessor: (invitation: InvitationSummary) => invitation.address },
		{ label: 'Roles', accessor: (invitation: InvitationSummary) => invitation.roles.join(', ') },
		{
			label: 'Employment type',
			accessor: (invitation: InvitationSummary) => invitation.employmentType
		},
		{
			label: 'Expires',
			accessor: (invitation: InvitationSummary) =>
				new Date(invitation.expiresAt).toLocaleDateString()
		}
	];

	async function loadRoster() {
		const response = await apiFetchWithSession(`/api/practices/${page.params.practiceId}/staff`);
		if (!response.ok) {
			error = await response.text();
			return;
		}

		const roster: Roster = await response.json();
		members = roster.members;
		invitations = roster.invitations;
		isLoaded = true;
	}

	onMount(loadRoster);

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

	function startEditing(member: StaffSummary) {
		editingStaffId = member.staffId;
		editRoles = [...member.roles];
		editEmploymentType = member.employmentType;
		editError = '';
	}

	async function handleSaveMembership(event: SubmitEvent) {
		event.preventDefault();
		editError = '';
		isSavingEdit = true;
		try {
			const response = await apiFetchWithSession(
				`/api/practices/${page.params.practiceId}/staff/${editingStaffId}/membership`,
				{
					method: 'PATCH',
					headers: { 'Content-Type': 'application/json' },
					body: JSON.stringify({ roles: editRoles, employmentType: editEmploymentType })
				}
			);
			if (!response.ok) {
				editError = await response.text();
				return;
			}
			editingStaffId = '';
			await loadRoster();
		} catch (error_) {
			editError = error_ instanceof Error ? error_.message : 'Failed to save';
		} finally {
			isSavingEdit = false;
		}
	}

	async function handleRevoke(invitationId: string) {
		revokeError[invitationId] = '';
		revokingInvitationId = invitationId;
		try {
			const response = await apiFetchWithSession(
				`/api/practices/${page.params.practiceId}/staff/invitations/${invitationId}/revoke`,
				{ method: 'POST' }
			);
			if (!response.ok) {
				revokeError[invitationId] = await response.text();
				return;
			}
			await loadRoster();
		} catch (error_) {
			revokeError[invitationId] =
				error_ instanceof Error ? error_.message : 'Failed to revoke invitation';
		} finally {
			revokingInvitationId = '';
		}
	}
</script>

{#snippet memberActions(member: StaffSummary)}
	{#if editingStaffId === member.staffId}
		<form onsubmit={handleSaveMembership}>
			<MembershipFields
				roles={editRoles}
				employmentType={editEmploymentType}
				onRolesChange={(next) => (editRoles = next)}
				onEmploymentTypeChange={(next) => (editEmploymentType = next)}
			/>
			<Button type="submit" label="Save membership" loading={isSavingEdit} />
			<Button
				label="Cancel"
				variant="secondary"
				size="sm"
				onClick={() => (editingStaffId = '')}
			/>
			{#if editError}
				<Notice variant="error" message={editError} />
			{/if}
		</form>
	{:else}
		<Button
			label="Edit membership"
			variant="secondary"
			size="sm"
			onClick={() => startEditing(member)}
		/>
	{/if}
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

{#snippet invitationActions(invitation: InvitationSummary)}
	{#if invitation.deliveryFailed}
		<Badge label="Email could not be delivered" variant="warning" />
	{/if}
	<Button
		label="Revoke"
		variant="secondary"
		size="sm"
		onClick={() => handleRevoke(invitation.invitationId)}
		loading={revokingInvitationId === invitation.invitationId}
	/>
	{#if revokeError[invitation.invitationId]}
		<Notice variant="error" message={revokeError[invitation.invitationId]} />
	{/if}
{/snippet}

<Heading level={1} text="Staff" />

{#if error}
	<Notice variant="error" message={error} />
{:else if isLoaded}
	<!-- Two groups, not one list: a pending invitation is an address that
	     has been asked, with nobody behind it yet, and #261 found the
	     single-list shape unable to tell that apart from a member holding
	     no roles. -->
	<Heading level={2} text="Members" />
	{#if members.length === 0}
		<Text text="No Staff yet." />
	{:else}
		<DataTable
			columns={memberColumns}
			rows={members}
			rowActions={{ label: 'Actions', content: memberActions }}
			emptyMessage="No Staff yet."
		/>
	{/if}

	<Heading level={2} text="Pending invitations" />
	{#if invitations.length === 0}
		<Text text="No pending invitations." />
	{:else}
		<DataTable
			columns={invitationColumns}
			rows={invitations}
			rowActions={{ label: 'Actions', content: invitationActions }}
			emptyMessage="No pending invitations."
		/>
	{/if}
{/if}
