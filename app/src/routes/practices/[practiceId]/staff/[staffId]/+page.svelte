<script lang="ts">
	/*
	 * One Staff member's page (#2041): the destination of the roster's one
	 * row action, the name. It holds what the roster row used to hold in
	 * its Actions cell -- two history disclosures, an in-cell Membership
	 * form, a recovery link and two red buttons side by side -- laid out as
	 * a record page instead (`docs/design/govuk-alignment.md`, rule 5,
	 * "Sections of a record page"): the facts as a summary list, one
	 * Change per fact that opens its own question page, the two histories
	 * as sections, and the two acts that end something in a last Access
	 * section, apart from each other and from everything benign.
	 *
	 * ## Who sees what
	 *
	 * The roster and both histories are Owner-and-Admin; changing a
	 * Membership, ending its sessions, removing it and vouching are
	 * Owner-only (`staffauth/mount.go`). So an Admin reads the same facts
	 * and histories with no Change, no recovery link and no Access
	 * section. Drawing, never a gate (roles.ts): the BFF refuses each act
	 * regardless.
	 */
	import { onMount } from 'svelte';
	import { resolve } from '$app/paths';
	import { page } from '#lib/appState.svelte.js';
	import { apiFetchWithSession } from '#lib/api.js';
	import {
		endSessions,
		loadMembershipHistory,
		loadWorkStateHistory,
		removeMember,
		type MembershipChange,
		type StaffSummary,
		type WorkStateChange
	} from '#lib/staff.js';
	import { PaginatedList } from '#lib/paginatedList.svelte.js';
	import { formatActivityTimestamp } from '#lib/dates.js';
	import { membershipChangeSentence } from '#lib/membershipHistory.js';
	import { gotoWithOutcome, takeOutcome } from '#lib/outcome.js';
	import { workStateName, workStateReportedOn } from '#lib/workStates.js';
	import { employmentTypeLabel, isOwner as checkIsOwner, rolesLabel } from '#lib/roles.js';
	import Button from '#lib/components/atoms/Button.svelte';
	import Heading from '#lib/components/atoms/Heading.svelte';
	import Link from '#lib/components/atoms/Link.svelte';
	import Notice from '#lib/components/atoms/Notice.svelte';
	import Skeleton from '#lib/components/atoms/Skeleton.svelte';
	import Text from '#lib/components/atoms/Text.svelte';
	import WarningText from '#lib/components/atoms/WarningText.svelte';
	import ConfirmDialog from '#lib/components/molecules/ConfirmDialog.svelte';
	import SummaryList, { type SummaryListRow } from '#lib/components/molecules/SummaryList.svelte';
	import DataTable from '#lib/components/organisms/DataTable.svelte';
	import RecordDetail from '#lib/components/templates/RecordDetail.svelte';
	import type { PracticeSession } from '../../+layout.js';
	import { loadMember, rosterHref } from './member.js';

	const practiceId = $derived(page.params.practiceId!);
	const staffId = $derived(page.params.staffId!);
	const isOwner = $derived(checkIsOwner((page.data as { session: PracticeSession }).session));

	// What the change that led here did, if one did: a roles or an
	// employment type question page. Read once, while the page renders.
	const outcome = takeOutcome(page.url);

	let member = $state<StaffSummary | undefined>();
	let loadError = $state('');

	/*
	 * One work state entry as this page reads it: the endpoint's own
	 * fields, plus whether the assertion predates the Membership here. A
	 * contractor doula who recorded a work state at another Practice
	 * carries that row into this one, and the page must not read as though
	 * it was said here (#459) -- so the answer travels with the entry.
	 */
	type WorkStateEntry = WorkStateChange & { beforeJoining: boolean };

	/*
	 * Both histories are read with the page, not behind a disclosure. The
	 * roster deferred them because it was one row per person and would
	 * have grown with every change anybody ever made (#872, #1149); a page
	 * about one person has no such cost, and its sections are the reason
	 * it was opened. Each still pages, through the list's own guards.
	 */
	const workStateHistory = PaginatedList.deferred<WorkStateEntry>({
		loadPage: async (cursor) => {
			const loaded = await loadWorkStateHistory(apiFetchWithSession, practiceId, staffId, cursor);
			return {
				...loaded,
				items: loaded.items.map((change) => ({
					...change,
					beforeJoining: new Date(change.createdAt) < new Date(loaded.memberSince)
				}))
			};
		},
		failureMessage: 'Failed to load work state history'
	});

	const membershipHistory = PaginatedList.deferred<MembershipChange>({
		loadPage: (cursor) => loadMembershipHistory(apiFetchWithSession, practiceId, staffId, cursor),
		failureMessage: 'Failed to load membership history'
	});

	let isEndSessionsOpen = $state(false);
	let endSessionsError = $state('');
	let isSessionsEnded = $state(false);
	let isRemoveOpen = $state(false);
	let removeError = $state('');

	onMount(async () => {
		void workStateHistory.ask();
		void membershipHistory.ask();
		try {
			member = await loadMember(apiFetchWithSession, practiceId, staffId);
		} catch (error_) {
			loadError = error_ instanceof Error ? error_.message : 'Failed to load this Staff member';
		}
	});

	const name = $derived(member?.name ?? '');

	interface FactRow extends SummaryListRow {
		href?: string;
	}

	// The roster's own columns, as facts with a Change on the two an Owner
	// may change. Only the person may set a work state, so it has none.
	const facts = $derived<FactRow[]>(
		member
			? [
					{ label: 'Email address', value: member.email },
					{
						label: 'Roles',
						value: rolesLabel(member.roles) || 'No roles yet',
						changes: isOwner ? 'roles' : undefined,
						href: resolve('/practices/[practiceId]/staff/[staffId]/roles', { practiceId, staffId })
					},
					{
						label: 'Employment type',
						value: employmentTypeLabel(member.employmentType),
						changes: isOwner ? 'employment type' : undefined,
						href: resolve('/practices/[practiceId]/staff/[staffId]/employment-type', { practiceId, staffId })
					},
					{
						label: 'Works from',
						value: `${workStateName(member.workState)}, self-reported ${workStateReportedOn(member.workStateReportedAt)}`
					}
				]
			: []
	);

	// What one work state entry says. A first assertion (no previous
	// value, migration 00043 leaves it NULL) and a move are different
	// sentences: "changed from blank to New York" would invent a change.
	// Neither names this Practice, since 00043 records none (#459).
	function workStateWhat(change: WorkStateEntry): string {
		const sentence = change.previousWorkState
			? `Changed from ${workStateName(change.previousWorkState)} to ${workStateName(change.workState)}`
			: `Reported ${workStateName(change.workState)}`;
		return change.beforeJoining ? `${sentence} (before joining this practice)` : sentence;
	}

	// Offboarding or a lost device: ends every session this Staff member
	// holds, on every device, at once -- not sign-out, which ends only the
	// browser asking (#154). Rethrown so ConfirmDialog stays open with the
	// error inside it (#804).
	async function handleEndSessions() {
		endSessionsError = '';
		isSessionsEnded = false;
		try {
			await endSessions(apiFetchWithSession, practiceId, staffId);
			isSessionsEnded = true;
		} catch (error_) {
			endSessionsError = error_ instanceof Error ? error_.message : 'Failed to end sessions';
			throw error_;
		}
	}

	// Ends the Membership: reach over this Practice stops, the Staff
	// account and everything done here stay (#291). The page is about a
	// Membership that no longer exists, so the save lands on the roster
	// and says so there.
	async function handleRemove() {
		removeError = '';
		try {
			await removeMember(apiFetchWithSession, practiceId, staffId);
		} catch (error_) {
			removeError = error_ instanceof Error ? error_.message : 'Failed to remove this Staff member';
			throw error_;
		}
		await gotoWithOutcome(rosterHref(practiceId), `${name} was removed from this Practice.`);
	}
</script>

{#snippet actions()}
	<!--
		#694's way in to Owner vouching, moved here from the roster row.
		A Link, not a Button: it only navigates, to a screen with a
		consequence to read and a re-authentication of its own.
	-->
	<Link
		href={resolve('/practices/[practiceId]/staff/[staffId]/mfa-recovery', { practiceId, staffId })}
		label="Send a recovery code"
	/>
{/snippet}

{#snippet summary()}
	<SummaryList rows={facts} isBordered>
		{#snippet action(row, describedBy)}
			<!-- One Change per fact, each to its own question page: GOV.UK's
			     Summary list row action, and never a form in the row. -->
			<Link href={row.href!} label="Change" {describedBy} />
		{/snippet}
	</SummaryList>
{/snippet}

{#snippet workStateSection()}
	<Text
		text="Work states are self-reported by each person and are not verified. They set how much sales tax your practice pays on credits."
		tone="variant"
	/>
	{#if workStateHistory.entries}
		<!-- An append-only trail: the newest first, each entry a sentence.
		     The sentence is the one column that wraps (#2098), so the date
		     beside it stays on one line. -->
		<DataTable
			columns={[
				{
					label: 'When',
					accessor: (change: WorkStateEntry) => workStateReportedOn(change.createdAt),
					datetimeAccessor: (change: WorkStateEntry) => change.createdAt,
					variant: 'meta' as const
				},
				{ label: 'What', accessor: workStateWhat, variant: 'body' as const, wraps: true }
			]}
			rows={[...workStateHistory.entries]}
			hasMore={workStateHistory.hasMore}
			onLoadMore={() => workStateHistory.loadMore()}
			isLoadingMore={workStateHistory.isLoadingMore}
			loadMoreError={workStateHistory.loadMoreError}
			emptyMessage="Nothing recorded."
		/>
	{:else if workStateHistory.loadMoreError}
		<Notice variant="error" message={workStateHistory.loadMoreError} />
	{:else}
		<Skeleton variant="row" lines={3} label="Loading work state history" />
	{/if}
{/snippet}

{#snippet membershipSection()}
	{#if membershipHistory.entries}
		<!-- How this person came to hold these roles and this employment
		     type (#872), and who made each change: the audit trail. The
		     sentence is the one column that wraps (#2098); When and Who stay
		     on one line. -->
		<DataTable
			columns={[
				{
					label: 'When',
					accessor: (change: MembershipChange) => formatActivityTimestamp(change.createdAt),
					datetimeAccessor: (change: MembershipChange) => change.createdAt,
					variant: 'meta' as const
				},
				{ label: 'What', accessor: membershipChangeSentence, variant: 'body' as const, wraps: true },
				{ label: 'Who', accessor: (change: MembershipChange) => change.actorName, variant: 'muted' as const }
			]}
			rows={[...membershipHistory.entries]}
			hasMore={membershipHistory.hasMore}
			onLoadMore={() => membershipHistory.loadMore()}
			isLoadingMore={membershipHistory.isLoadingMore}
			loadMoreError={membershipHistory.loadMoreError}
			emptyMessage="Nothing recorded."
		/>
	{:else if membershipHistory.loadMoreError}
		<Notice variant="error" message={membershipHistory.loadMoreError} />
	{:else}
		<Skeleton variant="row" lines={3} label="Loading membership history" />
	{/if}
{/snippet}

{#snippet accessSection()}
	<!--
		The two acts that end something, last on the page and apart from
		each other (Fitts's Law): each its own heading, its consequence as
		WarningText before the button, and a ConfirmDialog after it. The
		roster used to put both red buttons side by side on every row.
	-->
	<stack-l space="var(--space-group)">
		<stack-l space="var(--space-related)">
			<Heading level={3} variant="card" text="End sessions everywhere" />
			<WarningText message={`${name} is signed out on every device at once. Use this when a phone or computer is lost.`} />
			<div>
				<Button label="End sessions everywhere" variant="destructive" onClick={() => (isEndSessionsOpen = true)} />
			</div>
			{#if isSessionsEnded}
				<Notice variant="status" message="Sessions ended." />
			{/if}
		</stack-l>
		<stack-l space="var(--space-related)">
			<Heading level={3} variant="card" text="Remove from this Practice" />
			<WarningText
				message={`${name} loses access to this Practice's Clients immediately. The account, and everything done here, stays.`}
			/>
			<div>
				<Button label="Remove from this Practice" variant="destructive" onClick={() => (isRemoveOpen = true)} />
			</div>
		</stack-l>
	</stack-l>
	<ConfirmDialog
		bind:open={isEndSessionsOpen}
		title="End sessions everywhere"
		consequence={`${name} is signed out on every device immediately.`}
		confirmLabel="End sessions everywhere"
		error={endSessionsError}
		onConfirm={handleEndSessions}
	/>
	<ConfirmDialog
		bind:open={isRemoveOpen}
		title="Remove from this Practice"
		consequence={`${name} loses access to this Practice's Clients immediately.`}
		confirmLabel="Remove from this Practice"
		error={removeError}
		onConfirm={handleRemove}
	/>
{/snippet}

<RecordDetail
	title={name}
	{outcome}
	backHref={rosterHref(practiceId)}
	backLabel="Staff"
	{summary}
	actions={isOwner ? actions : undefined}
	sections={[
		{ heading: 'Work state history', content: workStateSection },
		{ heading: 'Membership history', content: membershipSection },
		...(isOwner ? [{ heading: 'Access', content: accessSection }] : [])
	]}
	loading={member || loadError ? undefined : 'Loading this Staff member'}
	loadError={loadError || undefined}
/>
