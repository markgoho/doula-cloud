<script lang="ts">
	import { onMount } from 'svelte';
	import { SvelteSet } from 'svelte/reactivity';
	import { page } from '$app/state';
	import { resolve } from '$app/paths';
	import { apiFetchWithSession } from '#lib/api.js';
	import DataTable from '#lib/components/organisms/DataTable.svelte';
	import Heading from '#lib/components/atoms/Heading.svelte';
	import Text from '#lib/components/atoms/Text.svelte';
	import Notice from '#lib/components/atoms/Notice.svelte';
	import Badge from '#lib/components/atoms/Badge.svelte';
	import Button from '#lib/components/atoms/Button.svelte';
	import Link from '#lib/components/atoms/Link.svelte';
	import MembershipFields from '#lib/components/molecules/MembershipFields.svelte';
	import { workStateName, workStateReportedOn } from '#lib/workStates.js';

	type StaffSummary = {
		staffId: string;
		name: string;
		email: string;
		roles: string[];
		employmentType: 'employee' | 'contractor';
		workState: string;
		workStateReportedAt: string;
	};

	type InvitationSummary = {
		invitationId: string;
		address: string;
		roles: string[];
		employmentType: 'employee' | 'contractor';
		expiresAt: string;
		expired: boolean;
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

	// The history behind one member's "Works from" value (#459). Fetched
	// when its disclosure is opened, never with the roster: the roster is
	// one row per person and would otherwise grow with every correction
	// anybody has ever made, which is the one thing this screen must not
	// do.
	type WorkStateChange = {
		eventId: string;
		previousWorkState?: string;
		workState: string;
		createdAt: string;
	};
	type WorkStateHistory = {
		memberSince: string;
		items: WorkStateChange[];
		nextCursor?: string;
		hasMore: boolean;
	};

	let histories = $state<Record<string, WorkStateHistory>>({});
	// Who has been asked for already: it stops a second open from asking
	// again, because an append-only trail that was right a second ago is
	// still right. Nothing renders it, but it is a SvelteSet because the
	// repo's lint rule admits no unreactive Set.
	const requestedHistories = new SvelteSet<string>();
	let historyLoading = $state<Record<string, boolean>>({});
	let historyError = $state<Record<string, string>>({});

	let revokingInvitationId = $state('');
	let revokeError = $state<Record<string, string>>({});

	let removingStaffId = $state('');
	let removeError = $state<Record<string, string>>({});

	const memberColumns = [
		{ label: 'Name', accessor: (member: StaffSummary) => member.name },
		{ label: 'Email', accessor: (member: StaffSummary) => member.email },
		{
			label: 'Roles',
			accessor: (member: StaffSummary) => member.roles.join(', ') || 'no roles yet'
		},
		{ label: 'Employment type', accessor: (member: StaffSummary) => member.employmentType },
		// Only the person herself may set this, so "self-reported" is
		// always the true provenance -- and the date is the only staleness
		// signal there is, since nothing prompts a re-assertion (#415). A
		// contractor who recorded it at another Practice shows up here with
		// a date older than her membership, which is the point.
		{
			label: 'Works from',
			accessor: (member: StaffSummary) =>
				`${workStateName(member.workState)} — self-reported ${workStateReportedOn(member.workStateReportedAt)}`
		}
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

	// One page of a member's work state history, appended to whatever is
	// already on screen. cursor is undefined for the first page.
	async function loadWorkStateHistory(staffId: string, cursor?: string) {
		historyError[staffId] = '';
		historyLoading[staffId] = true;
		try {
			const query = cursor ? `?cursor=${encodeURIComponent(cursor)}` : '';
			const response = await apiFetchWithSession(
				`/api/practices/${page.params.practiceId}/staff/${staffId}/work-state-history${query}`
			);
			if (!response.ok) {
				historyError[staffId] = await response.text();
				return;
			}
			const loaded: WorkStateHistory = await response.json();
			const existing = cursor ? (histories[staffId]?.items ?? []) : [];
			histories[staffId] = { ...loaded, items: [...existing, ...loaded.items] };
		} catch (error_) {
			historyError[staffId] =
				error_ instanceof Error ? error_.message : 'Failed to load work state history';
		} finally {
			historyLoading[staffId] = false;
		}
	}

	// Opening the disclosure is what asks for the history; closing and
	// reopening does not ask again, because an append-only trail that was
	// correct a second ago is still correct.
	function handleHistoryToggle(staffId: string, isOpen: boolean) {
		if (!isOpen || requestedHistories.has(staffId)) {
			return;
		}
		requestedHistories.add(staffId);
		void loadWorkStateHistory(staffId);
	}

	// What one entry says. A first assertion (no previous value, migration
	// 00043 leaves it NULL) and a move are different sentences, not the
	// same sentence with a blank in it: printing "changed from — to New
	// York" for somebody's onboarding would invent a change she never
	// made.
	//
	// Neither sentence names this Practice. Only the person herself may
	// write a work state, and 00043's table records no Practice at all, so
	// an entry says what she reported and never that she reported it here
	// -- which for a contractor doula carrying an assertion in from
	// another Practice would be untrue.
	function workStateChangeSentence(change: WorkStateChange): string {
		return change.previousWorkState
			? `Changed from ${workStateName(change.previousWorkState)} to ${workStateName(change.workState)}`
			: `Reported ${workStateName(change.workState)}`;
	}

	// An entry older than the Membership was made somewhere else, before
	// this Practice had her. Saying so is the whole of #459's "must not
	// read as she told us this here".
	function isBeforeJoining(change: WorkStateChange, memberSince: string): boolean {
		return new Date(change.createdAt) < new Date(memberSince);
	}

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

	// Ends a Membership: her reach over this Practice stops, her Staff
	// account and everything she did while she was here stay (#291).
	async function handleRemoveMembership(staffId: string) {
		removeError[staffId] = '';
		removingStaffId = staffId;
		try {
			const response = await apiFetchWithSession(
				`/api/practices/${page.params.practiceId}/staff/${staffId}/membership`,
				{ method: 'DELETE' }
			);
			if (!response.ok) {
				removeError[staffId] = await response.text();
				return;
			}
			await loadRoster();
		} catch (error_) {
			removeError[staffId] =
				error_ instanceof Error ? error_.message : 'Failed to remove membership';
		} finally {
			removingStaffId = '';
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
	<!--
		The history behind the "Works from" value on this row (#459). The
		column shows the current state and the day it was asserted, which
		answers "how did this get set?" only while the value has never
		moved; once it has, the earlier assertion -- the one every Credit
		purchase before that date was apportioned on -- had nowhere to be
		read.

		A native <details>, so opening and closing costs no JavaScript and
		the keyboard and screen-reader behaviour is the browser's own
		(GOV.UK's Details pattern, ADR-0021). The only script here is the
		fetch the first open triggers.
	-->
	{@const history = histories[member.staffId]}
	<details ontoggle={(event) => handleHistoryToggle(member.staffId, event.currentTarget.open)}>
		<summary>Work state history</summary>
		{#if historyError[member.staffId]}
			<Notice variant="error" message={historyError[member.staffId]} />
		{:else if !history}
			<Text text="Loading..." />
		{:else if history.items.length === 0}
			<Text text="Nothing recorded." />
		{:else}
			<ol>
				{#each history.items as change (change.eventId)}
					<li>
						{workStateChangeSentence(change)} &mdash;
						<time datetime={change.createdAt}>{workStateReportedOn(change.createdAt)}</time>
						{#if isBeforeJoining(change, history.memberSince)}
							<span class="elsewhere">(before joining this practice)</span>
						{/if}
					</li>
				{/each}
			</ol>
			{#if history.hasMore}
				<Button
					label="Show older changes"
					variant="secondary"
					size="sm"
					loading={historyLoading[member.staffId]}
					onClick={() => loadWorkStateHistory(member.staffId, history.nextCursor)}
				/>
			{/if}
		{/if}
	</details>
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
	<Button
		label="Remove from practice"
		variant="secondary"
		size="sm"
		onClick={() => handleRemoveMembership(member.staffId)}
		loading={removingStaffId === member.staffId}
	/>
	{#if removeError[member.staffId]}
		<Notice variant="error" message={removeError[member.staffId]} />
	{/if}
{/snippet}

{#snippet invitationActions(invitation: InvitationSummary)}
	{#if invitation.expired}
		<Badge label="Expired -- invite again or revoke" variant="neutral" />
	{/if}
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

<!--
	Inviting somebody is an action on this roster, so it belongs on the
	roster. It used to hang off the temporary header of links the shell
	replaced (#452), which is the only reason it was ever anywhere else --
	and with that header gone, nothing else in the app reaches /invite.
-->
<Link
	href={resolve('/practices/[practiceId]/invite', { practiceId: page.params.practiceId! })}
	label="Invite a Staff member"
/>

{#if error}
	<Notice variant="error" message={error} />
{:else if isLoaded}
	<!-- Two groups, not one list: a pending invitation is an address that
	     has been asked, with nobody behind it yet, and #261 found the
	     single-list shape unable to tell that apart from a member holding
	     no roles. -->
	<Heading level={2} text="Members" />
	<Text
		text="Work states are self-reported by each person and are not verified. They set how much sales tax your practice pays on credits."
	/>
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

<style>
	@layer components {
		/* A dated list of assertions, not a bulleted aside: the order is the
		   history, so it is an <ol> with its markers off and the dates doing
		   the numbering's job. */
		ol {
			margin: 0;
			padding: 0;
			list-style: none;
			font-size: var(--text-body-sm-size);
			line-height: var(--text-body-sm-leading);
		}

		li {
			padding-block: var(--space-1);
		}

		summary {
			font-size: var(--text-body-sm-size);
			cursor: pointer;
		}

		/* Quieter than the assertion it qualifies -- it is a caveat about
		   where the row came from, not part of what she said. */
		.elsewhere {
			color: var(--color-on-surface-muted);
		}
	}
</style>
