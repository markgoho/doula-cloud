<script lang="ts">
	import { onMount } from 'svelte';
	import { SvelteMap } from 'svelte/reactivity';
	import { page } from '#lib/appState.svelte.js';
	import { resolve } from '$app/paths';
	import { apiFetchWithSession } from '#lib/api.js';
	import {
		endSessions,
		loadMembershipHistory as fetchMembershipHistory,
		loadStaff,
		loadWorkStateHistory as fetchWorkStateHistory,
		removeMember,
		revokeInvitation,
		updateMembership,
		type InvitationSummary,
		type MembershipChange,
		type StaffSummary,
		type WorkStateChange
	} from '#lib/staff.js';
	import {
		PaginatedList,
		type DeferredPaginatedList,
		type PageLoader
	} from '#lib/paginatedList.svelte.js';
	import DataTable, { type DataTableView } from '#lib/components/organisms/DataTable.svelte';
	import Heading from '#lib/components/atoms/Heading.svelte';
	import Text from '#lib/components/atoms/Text.svelte';
	import Notice from '#lib/components/atoms/Notice.svelte';
	import Badge from '#lib/components/atoms/Badge.svelte';
	import Button from '#lib/components/atoms/Button.svelte';
	import Link from '#lib/components/atoms/Link.svelte';
	import MembershipFields from '#lib/components/molecules/MembershipFields.svelte';
	import ConfirmDialog from '#lib/components/molecules/ConfirmDialog.svelte';
	import HistoryDisclosure from '#lib/components/molecules/HistoryDisclosure.svelte';
	import StackedForm from '#lib/components/molecules/StackedForm.svelte';
	import ListPage from '#lib/components/templates/ListPage.svelte';
	import { formatActivityTimestamp } from '#lib/dates.js';
	import { membershipChangeSentence } from '#lib/membershipHistory.js';
	import { workStateName, workStateReportedOn } from '#lib/workStates.js';
	import { isOwner, rolesLabel, employmentTypeLabel, type EmploymentType } from '#lib/roles.js';
	import type { PracticeSession } from '../+layout.js';

	// #694: the Membership comes off practices/[practiceId]/+layout.ts's
	// already-resolved read (#835), not a fetch of this page's own.
	const isPracticeOwner = $derived(isOwner((page.data as { session: PracticeSession }).session));

	let members = $state<StaffSummary[]>([]);
	// Only the Invitations grow: the Members roster stays whole (#446 --
	// it is bounded, unlike the Invitation history), so it needs no cursor
	// of its own.
	const invitations = new PaginatedList<InvitationSummary>({
		first: { items: [], hasMore: false },
		loadPage: async (cursor) => {
			const roster = await loadStaff(apiFetchWithSession, page.params.practiceId!, cursor);
			return roster.invitations;
		},
		failureMessage: 'Failed to load more invitations'
	});
	let error = $state('');
	let isLoaded = $state(false);

	let endingSessionsFor = $state<Record<string, boolean>>({});
	let endSessionsError = $state<Record<string, string>>({});
	let endSessionsDone = $state<Record<string, boolean>>({});

	// One Membership is edited at a time, in place on its own row: roles
	// and employment type on one form (RA-G2, #261), not two round trips.
	let editingStaffId = $state('');
	let editRoles = $state<string[]>([]);
	let editEmploymentType = $state<EmploymentType>('employee');
	let isSavingEdit = $state(false);
	let editError = $state('');

	/*
	 * One work state entry as this screen reads it: the endpoint's own
	 * fields, plus whether the assertion predates her Membership here. A
	 * contractor doula who recorded her work state at another Practice
	 * carries that row into this one, and the screen must not read as
	 * though she said it here (#459) -- so the answer travels with the
	 * entry rather than the snippet asking a second question about the
	 * page the entry came on.
	 */
	type WorkStateEntry = WorkStateChange & { beforeJoining: boolean };

	/*
	 * The two histories a roster row carries: what is behind her "Works
	 * from" value (#459), and what is behind the row itself (#872). Each
	 * is fetched when its disclosure is opened, never with the roster --
	 * the roster is one row per person, and would otherwise grow with
	 * every correction and every role change anybody has ever made.
	 *
	 * One deferred `PaginatedList` per member per history (#1149), rather
	 * than the four parallel records per disclosure this screen used to
	 * keep: the pages so far, whether one is in flight, the last failure,
	 * and who had been asked for at all. The list owns all four, plus the
	 * three guards the roster's own pair never had -- a superseded page
	 * cannot land, a repeated ask is one request, and a zero-item page
	 * with more to come is paged past (#709). A third history on this row
	 * is a third map, not a third copy of the machinery.
	 */
	const workStateHistories = new SvelteMap<string, DeferredPaginatedList<WorkStateEntry>>();
	const membershipHistories = new SvelteMap<string, DeferredPaginatedList<MembershipChange>>();

	let revokeError = $state<Record<string, string>>({});

	let removeError = $state<Record<string, string>>({});

	let confirmEndSessionsFor = $state('');
	let confirmRemoveStaffId = $state('');
	let confirmRevokeInvitationId = $state('');

	const memberColumns = [
		{ label: 'Name', accessor: (member: StaffSummary) => member.name },
		{ label: 'Email', accessor: (member: StaffSummary) => member.email },
		{
			label: 'Roles',
			accessor: (member: StaffSummary) => rolesLabel(member.roles) || 'no roles yet'
		},
		{
			label: 'Employment type',
			accessor: (member: StaffSummary) => employmentTypeLabel(member.employmentType)
		},
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
		{ label: 'Roles', accessor: (invitation: InvitationSummary) => rolesLabel(invitation.roles) },
		{
			label: 'Employment type',
			accessor: (invitation: InvitationSummary) => employmentTypeLabel(invitation.employmentType)
		},
		{
			label: 'Expires',
			accessor: (invitation: InvitationSummary) =>
				new Date(invitation.expiresAt).toLocaleDateString()
		}
	];

	async function loadRoster() {
		try {
			const roster = await loadStaff(apiFetchWithSession, page.params.practiceId!);
			members = roster.members;
			invitations.reset(roster.invitations);
			isLoaded = true;
		} catch (error_) {
			error = error_ instanceof Error ? error_.message : 'Failed to load the roster';
		}
	}

	onMount(loadRoster);

	/*
	 * The list holding one member's entry in `histories`, made on the first
	 * ask and kept from then on -- reopening a disclosure
	 * finds the entries it already has rather than fetching them twice,
	 * because an append-only trail that was correct a second ago is still
	 * correct.
	 */
	function historyFor<Item>(
		histories: SvelteMap<string, DeferredPaginatedList<Item>>,
		staffId: string,
		loadPage: PageLoader<Item>,
		failureMessage: string
	): DeferredPaginatedList<Item> {
		const existing = histories.get(staffId);
		if (existing) return existing;
		const history = PaginatedList.deferred<Item>({ loadPage, failureMessage });
		histories.set(staffId, history);
		return history;
	}

	/*
	 * Opening the disclosure is what asks for the history; HistoryDisclosure
	 * calls this on an open only, never on a close, and `ask` is what makes
	 * a second open cost no second request.
	 *
	 * Each entry is marked here with whether it was made before she joined
	 * this Practice, rather than the snippet asking the page a second
	 * question: `memberSince` belongs to the page the entry arrived on, and
	 * folding it in as the page lands is what keeps it from becoming a
	 * parallel record of its own.
	 */
	function openWorkStateHistory(staffId: string) {
		void historyFor(
			workStateHistories,
			staffId,
			async (cursor) => {
				const loaded = await fetchWorkStateHistory(
					apiFetchWithSession,
					page.params.practiceId!,
					staffId,
					cursor
				);
				return {
					...loaded,
					items: loaded.items.map((change) => ({
						...change,
						beforeJoining: isBeforeJoining(change, loaded.memberSince)
					}))
				};
			},
			'Failed to load work state history'
		).ask();
	}

	function openMembershipHistory(staffId: string) {
		void historyFor(
			membershipHistories,
			staffId,
			(cursor) =>
				fetchMembershipHistory(apiFetchWithSession, page.params.practiceId!, staffId, cursor),
			'Failed to load membership history'
		).ask();
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
			await endSessions(apiFetchWithSession, page.params.practiceId!, staffId);
			endSessionsDone[staffId] = true;
		} catch (error_) {
			// Rethrown so ConfirmDialog's own contract (left open on a
			// rejected onConfirm) holds and `error` below renders inside the
			// dialog rather than behind it (#804).
			endSessionsError[staffId] =
				error_ instanceof Error ? error_.message : 'Failed to end sessions';
			throw error_;
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
			await updateMembership(
				apiFetchWithSession,
				page.params.practiceId!,
				editingStaffId,
				editRoles,
				editEmploymentType
			);
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
		try {
			await removeMember(apiFetchWithSession, page.params.practiceId!, staffId);
			await loadRoster();
		} catch (error_) {
			// Rethrown for the same reason handleEndSessions above rethrows.
			removeError[staffId] =
				error_ instanceof Error ? error_.message : 'Failed to remove membership';
			throw error_;
		}
	}

	async function handleRevoke(invitationId: string) {
		revokeError[invitationId] = '';
		try {
			await revokeInvitation(apiFetchWithSession, page.params.practiceId!, invitationId);
			await loadRoster();
		} catch (error_) {
			// Rethrown for the same reason handleEndSessions above rethrows.
			revokeError[invitationId] =
				error_ instanceof Error ? error_.message : 'Failed to revoke invitation';
			throw error_;
		}
	}
</script>

{#snippet memberActions(member: StaffSummary, view: DataTableView)}
	<!--
		#666: DataTable renders this snippet into both of its trees, so
		every id below is scoped by which tree is asking. Without that,
		each `aria-describedby` here resolves to the `<table>` copy of the
		span whether or not the table is the view on screen.
	-->

	<!--
		The history behind the "Works from" value on this row (#459). The
		column shows the current state and the day it was asserted, which
		answers "how did this get set?" only while the value has never
		moved; once it has, the earlier assertion -- the one every Credit
		purchase before that date was apportioned on -- had nowhere to be
		read.
	-->
	{@const history = workStateHistories.get(member.staffId)}
	<HistoryDisclosure
		label="Work state history"
		subjectName={member.name}
		items={history?.entries}
		key={(change: WorkStateEntry) => change.eventId}
		error={history?.loadMoreError}
		emptyMessage="Nothing recorded."
		hasMore={history?.hasMore ?? false}
		isLoadingMore={history?.isLoadingMore}
		loadMoreLabel="Show older changes"
		idPrefix="{view}-{member.staffId}-work-state"
		onOpen={() => openWorkStateHistory(member.staffId)}
		onLoadMore={() => history?.loadMore()}
		entry={workStateEntry}
	/>
	{#snippet workStateEntry(change: WorkStateEntry)}
		{workStateChangeSentence(change)} &mdash;
		<time datetime={change.createdAt}>{workStateReportedOn(change.createdAt)}</time>
		{#if change.beforeJoining}
			<span class="elsewhere">(before joining this practice)</span>
		{/if}
	{/snippet}

	<!--
		The history behind the row itself (#872): how this person came to
		hold these roles and this employment type. Every Membership write
		site has recorded itself since the beginning -- signup, an accepted
		Invitation, an Owner's edit, a removal, an ended session -- and
		until this disclosure none of it could be read anywhere in the
		product.
	-->
	{@const membershipHistory = membershipHistories.get(member.staffId)}
	<HistoryDisclosure
		label="Membership history"
		subjectName={member.name}
		items={membershipHistory?.entries}
		key={(change: MembershipChange) => change.eventId}
		error={membershipHistory?.loadMoreError}
		emptyMessage="Nothing recorded."
		hasMore={membershipHistory?.hasMore ?? false}
		isLoadingMore={membershipHistory?.isLoadingMore}
		loadMoreLabel="Show older membership changes"
		idPrefix="{view}-{member.staffId}-membership"
		onOpen={() => openMembershipHistory(member.staffId)}
		onLoadMore={() => membershipHistory?.loadMore()}
		entry={membershipEntry}
	/>
	{#snippet membershipEntry(change: MembershipChange)}
		{membershipChangeSentence(change)} &mdash;
		<time datetime={change.createdAt}>{formatActivityTimestamp(change.createdAt)}</time>
		<!--
			Who did it, which is half of what the audit trail is for. Quieter
			than the change itself, the same treatment the activity ledger
			gives an actor (brief.md's "One signature component").
		-->
		<span class="actor">by {change.actorName}</span>
	{/snippet}
	{#if editingStaffId === member.staffId}
		<!--
			#1108: the ordinary form rhythm, not a tighter one, even though
			this sits inside a `DataTable` row. The row's detail region is a
			full-width panel rather than a dense cell -- `HistoryDisclosure`
			opens a whole ledger beside it -- so nothing here is short of
			room, and a membership editor that spaced its fields differently
			from every other form would be the fifth arrangement this ticket
			set out to remove. `ReauthPrompt` is the precedent for the
			confirm-and-cancel pair below stacking rather than sitting in a
			row.
		-->
		<StackedForm onSubmit={handleSaveMembership}>
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
		</StackedForm>
	{:else}
		<Button
			label="Edit membership"
			variant="secondary"
			size="sm"
			describedBy="{view}-{member.staffId}-edit-name"
			onClick={() => startEditing(member)}
		/>
		<span class="visually-hidden" id="{view}-{member.staffId}-edit-name">{member.name}</span>
	{/if}
	<!--
		#694: Owner-only, matching the vouch endpoint's own guard -- an
		Admin who followed this would meet a 403 and nothing else. Drawing,
		never a gate (roles.ts): the BFF refuses the POST regardless.

		A Link rather than a Button: it only ever navigates, and what it
		navigates to is a screen with a consequence to read and a
		re-authentication of its own -- neither of which fits in a table
		cell at 320px.
	-->
	{#if isPracticeOwner}
		<Link
			href={resolve('/practices/[practiceId]/staff/[staffId]/mfa-recovery', {
				practiceId: page.params.practiceId!,
				staffId: member.staffId
			})}
			label="Send a recovery code"
			describedBy="{view}-{member.staffId}-recovery-name"
		/>
		<span class="visually-hidden" id="{view}-{member.staffId}-recovery-name">{member.name}</span>
	{/if}
	<Button
		label="End sessions everywhere"
		variant="destructive"
		size="sm"
		describedBy="{view}-{member.staffId}-end-sessions-name"
		onClick={() => (confirmEndSessionsFor = member.staffId)}
	/>
	<span class="visually-hidden" id="{view}-{member.staffId}-end-sessions-name">{member.name}</span>
	<ConfirmDialog
		bind:open={
			() => confirmEndSessionsFor === member.staffId,
			(value) => {
				if (!value) confirmEndSessionsFor = '';
			}
		}
		title="End sessions everywhere"
		consequence={`${member.name} is signed out on every device immediately.`}
		confirmLabel="End sessions everywhere"
		error={endSessionsError[member.staffId]}
		onConfirm={() => handleEndSessions(member.staffId)}
	/>
	{#if endSessionsDone[member.staffId]}
		<Notice variant="status" message="Sessions ended." />
	{/if}
	<Button
		label="Remove from practice"
		variant="destructive"
		size="sm"
		describedBy="{view}-{member.staffId}-remove-name"
		onClick={() => (confirmRemoveStaffId = member.staffId)}
	/>
	<span class="visually-hidden" id="{view}-{member.staffId}-remove-name">{member.name}</span>
	<ConfirmDialog
		bind:open={
			() => confirmRemoveStaffId === member.staffId,
			(value) => {
				if (!value) confirmRemoveStaffId = '';
			}
		}
		title="Remove from Practice"
		consequence={`${member.name} loses access to this Practice's Clients immediately.`}
		confirmLabel="Remove from Practice"
		error={removeError[member.staffId]}
		onConfirm={() => handleRemoveMembership(member.staffId)}
	/>
{/snippet}

{#snippet invitationActions(invitation: InvitationSummary, view: DataTableView)}
	{#if invitation.expired}
		<Badge label="Expired -- invite again or revoke" variant="neutral" />
	{/if}
	{#if invitation.deliveryFailed}
		<Badge label="Email could not be delivered" variant="warning" />
	{/if}
	<Button
		label="Revoke"
		variant="destructive"
		size="sm"
		describedBy="{view}-{invitation.invitationId}-revoke-name"
		onClick={() => (confirmRevokeInvitationId = invitation.invitationId)}
	/>
	<span class="visually-hidden" id="{view}-{invitation.invitationId}-revoke-name">{invitation.address}</span>
	<ConfirmDialog
		bind:open={
			() => confirmRevokeInvitationId === invitation.invitationId,
			(value) => {
				if (!value) confirmRevokeInvitationId = '';
			}
		}
		title="Revoke invitation"
		consequence={`The invitation to ${invitation.address} no longer works.`}
		confirmLabel="Revoke invitation"
		error={revokeError[invitation.invitationId]}
		onConfirm={() => handleRevoke(invitation.invitationId)}
	/>
{/snippet}

{#snippet actions()}
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
{/snippet}

{#snippet content()}
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
			<!-- hasMore is always false: the roster is a bounded population
			     (server-capped at maxMembers, #446) and never paginates, unlike
			     Invitations below. -->
			<DataTable
				columns={memberColumns}
				rows={members}
				rowActions={{ label: 'Actions', content: memberActions }}
				hasMore={false}
				emptyMessage="No Staff yet."
			/>
		{/if}

		<Heading level={2} text="Pending invitations" />
		{#if invitations.items.length === 0}
			<Text text="No pending invitations." />
		{:else}
			<DataTable
				columns={invitationColumns}
				rows={invitations.items}
				rowActions={{ label: 'Actions', content: invitationActions }}
				hasMore={invitations.hasMore}
				onLoadMore={() => invitations.loadMore()}
				isLoadingMore={invitations.isLoadingMore}
				loadMoreError={invitations.loadMoreError}
				emptyMessage="No pending invitations."
			/>
		{/if}
	{/if}
{/snippet}

<ListPage title="Staff" {actions} {content} />

<style>
	@layer components {
		/* The list, its items and the summary are HistoryDisclosure's own
		   (#872), since both disclosures on this row want them identical.
		   What stays here is what belongs to one entry's own words. */

		/* Both disclosures end an entry with something quieter than the
		   entry itself: where the row came from, on a work state asserted
		   before she joined, and who made the change, on a Membership
		   event. Neither is part of what happened; both qualify it. */
		.elsewhere,
		.actor {
			color: var(--color-on-surface-muted);
		}
	}
</style>
