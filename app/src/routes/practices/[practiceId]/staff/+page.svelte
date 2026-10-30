<script lang="ts">
	import { onMount } from 'svelte';
	import { page } from '#lib/appState.svelte.js';
	import { resolve } from '$app/paths';
	import { apiFetchWithSession } from '#lib/api.js';
	import { loadStaff, revokeInvitation, type InvitationSummary, type StaffSummary } from '#lib/staff.js';
	import { PaginatedList } from '#lib/paginatedList.svelte.js';
	import DataTable, { type DataTableView } from '#lib/components/organisms/DataTable.svelte';
	import Heading from '#lib/components/atoms/Heading.svelte';
	import Text from '#lib/components/atoms/Text.svelte';
	import Notice from '#lib/components/atoms/Notice.svelte';
	import Skeleton from '#lib/components/atoms/Skeleton.svelte';
	import Badge from '#lib/components/atoms/Badge.svelte';
	import Button from '#lib/components/atoms/Button.svelte';
	import Link from '#lib/components/atoms/Link.svelte';
	import ConfirmDialog from '#lib/components/molecules/ConfirmDialog.svelte';
	import ListPage from '#lib/components/templates/ListPage.svelte';
	import { formatInstant } from '#lib/dates.js';
	import { takeOutcome } from '#lib/outcome.js';
	import { workStateName, workStateReportedOn } from '#lib/workStates.js';
	import { rolesLabel, employmentTypeLabel } from '#lib/roles.js';
	import { memberHref } from './[staffId]/member.js';

	// What the act that led here did, if one did: a removal on a Staff
	// member's page (#2041). Read once, while the page renders.
	const outcome = takeOutcome(page.url);

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

	let revokeError = $state<Record<string, string>>({});
	let confirmRevokeInvitationId = $state('');

	/*
	 * A member row is facts and one link, the name (#2041). Everything a
	 * row used to hold in an Actions cell -- two history disclosures, the
	 * in-cell Membership form, a recovery link and two red buttons side by
	 * side -- is on the Staff member's own page now, where a change opens
	 * its own question page and the two acts that end something stand
	 * apart (`govuk-alignment.md`, "Sections of a record page"). GOV.UK's
	 * Table: a table is for reading.
	 */
	const memberColumns = [
		{ label: 'Name', accessor: (member: StaffSummary) => member.name },
		{ label: 'Email address', accessor: (member: StaffSummary) => member.email },
		{
			label: 'Roles',
			accessor: (member: StaffSummary) => rolesLabel(member.roles) || 'No roles yet'
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
				`${workStateName(member.workState)}, self-reported ${workStateReportedOn(member.workStateReportedAt)}`
		}
	];

	const invitationColumns = [
		{ label: 'Email address', accessor: (invitation: InvitationSummary) => invitation.address },
		{ label: 'Roles', accessor: (invitation: InvitationSummary) => rolesLabel(invitation.roles) },
		{
			label: 'Employment type',
			accessor: (invitation: InvitationSummary) => employmentTypeLabel(invitation.employmentType)
		},
		{
			label: 'Expires',
			// expiresAt is an instant (staffauth/staff.go, RFC 3339).
			accessor: (invitation: InvitationSummary) => formatInstant(invitation.expiresAt)
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

	async function handleRevoke(invitationId: string) {
		revokeError[invitationId] = '';
		try {
			await revokeInvitation(apiFetchWithSession, page.params.practiceId!, invitationId);
			await loadRoster();
		} catch (error_) {
			// Rethrown so ConfirmDialog's own contract (left open on a
			// rejected onConfirm) holds and the error renders inside the
			// dialog rather than behind it (#804).
			revokeError[invitationId] =
				error_ instanceof Error ? error_.message : 'Failed to revoke invitation';
			throw error_;
		}
	}
</script>

<!--
	An Invitation has no page of its own to send a reader to -- it is an
	address that has been asked, with nobody behind it yet -- so its one
	act stays on its row.
-->
{#snippet invitationActions(invitation: InvitationSummary, view: DataTableView)}
	{#if invitation.expired}
		<Badge label="Expired — invite again or revoke" variant="neutral" />
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
		variant="start"
	/>
{/snippet}

{#snippet content()}
	{#if error}
		<Notice variant="error" message={error} />
	{:else}
		<!-- Two groups, not one list: a pending invitation is an address that
		     has been asked, with nobody behind it yet, and #261 found the
		     single-list shape unable to tell that apart from a member holding
		     no roles. The headings and the sentence are drawn while the roster
		     loads (#2248); bars hold each table's place. -->
		<Heading level={2} text="Members" />
		<Text
			text="Work states are self-reported by each person and are not verified. They set how much sales tax your Practice pays on Credits."
		/>
		{#if isLoaded}
			<!-- hasMore is always false: the roster is a bounded population
			     (server-capped at maxMembers, #446) and never paginates, unlike
			     Invitations below. Either list's empty state is DataTable's own
			     (#2015); the invite link above is the action for both. -->
			<div class="members-table">
				<DataTable
					columns={memberColumns}
					rows={members}
					rowHref={(member: StaffSummary) => memberHref(page.params.practiceId!, member.staffId)}
					hasMore={false}
					emptyMessage="No Staff yet."
				/>
			</div>
		{:else}
			<Skeleton variant="row" lines={6} label="Loading your Staff" />
		{/if}

		<Heading level={2} text="Pending invitations" />
		{#if isLoaded}
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
		{:else}
			<Skeleton variant="row" lines={2} />
		{/if}
	{/if}
{/snippet}

<ListPage title="Staff" {outcome} {actions} {content} />

<style>
	@layer components {
		/* The members table's content floor (#1945, #1981): below it the
		   table's cells wrap to more lines than at the old type floors, so
		   it shows its records -- the founder's answer on #1945. Measured
		   on this route's typical pilot screen, not its hostile one -- the
		   founder's answer of 2026-10-25 on #1981. Re-measured on #2041,
		   once the Actions column left the row: it was 86.5rem with it, a
		   width a 1440px window minus its gutters never reached, so the
		   roster never showed as a table. Measured 2026-10-26: the typical
		   screen's table needs 1094px on macOS/Chromium, and 68.5rem
		   (1096px) is the floor, with no margin beyond it. `DataTable.svelte`'s floor
		   comment says why the floor is the route's own. */
		.members-table {
			container: members-table / inline-size;
		}

		.members-table > :global(*) {
			font-size: var(--text-body-size);
			--data-table-table-display: none;
			--data-table-records-display: block;
		}

		@container members-table (min-width: 68.5rem) {
			.members-table > :global(*) {
				--data-table-table-display: table;
				--data-table-records-display: none;
			}
		}
	}
</style>
