<script lang="ts">
	import { onMount } from 'svelte';
	import { page } from '$app/state';
	import { apiFetchWithSession } from '#lib/api.js';
	import {
		displayName,
		loadClientDetail,
		pendingRequests,
		resolvedFieldValueText,
		type ClientDetail,
		type EngagementRequestSummary,
		type EngagementSummary,
		type HistoryEntry
	} from '#lib/clientDetail.js';
	import { kindLabel, withdrawRequest } from '#lib/engagementRequest.js';
	import RecordDetail from '#lib/components/templates/RecordDetail.svelte';
	import DescriptionList from '#lib/components/molecules/DescriptionList.svelte';
	import DataTable from '#lib/components/organisms/DataTable.svelte';
	import Button from '#lib/components/atoms/Button.svelte';
	import Heading from '#lib/components/atoms/Heading.svelte';
	import Link from '#lib/components/atoms/Link.svelte';
	import Notice from '#lib/components/atoms/Notice.svelte';

	let detail = $state<ClientDetail | undefined>();
	let error = $state('');
	// The signed-in Staff member's own id, read once alongside the Client
	// (#504): "Withdraw, available to the requester alone" is a fact about
	// who is looking, not about the Client record itself. Left blank on a
	// failed read rather than surfaced as a page error -- the endpoint
	// enforces the real rule (ADR-0017's requested_by = $1) regardless, so
	// the worst a lost session read costs is a Withdraw button that does
	// not render for its own requester this one time.
	let staffId = $state('');
	let withdrawingRequestId = $state('');
	let withdrawError = $state('');
	let withdrawnConfirmation = $state('');

	function editHref(): string {
		return `/practices/${page.params.practiceId}/clients/${page.params.clientId}/edit`;
	}

	function newEngagementRequestHref(): string {
		return `/practices/${page.params.practiceId}/clients/${page.params.clientId}/engagement-requests/new`;
	}

	function formattedDate(iso: string): string {
		return new Date(iso).toLocaleDateString();
	}

	// A history row can be either shape; DataTable's Column is one
	// accessor per column, so each column below switches on the row it is
	// given rather than the two shapes getting two tables -- the AC is one
	// merged timeline, not two lists.
	function historyWho(entry: HistoryEntry): string {
		if (entry.type === 'client_event') {
			// ADR-0022: the third actor kind displays as "Doula Cloud", never
			// the engineering word "System" -- and only that kind lacks a
			// name, since Staff and Client actors are both joined in.
			return entry.clientEvent.actorKind === 'system' ? 'Doula Cloud' : (entry.clientEvent.actorName ?? 'Unknown staff');
		}
		return entry.engagementRequest.requestedByName;
	}

	function historyWhat(entry: HistoryEntry): string {
		if (entry.type === 'client_event') {
			return entry.clientEvent.eventType === 'created' ? 'Record created' : 'Record updated';
		}
		const request = entry.engagementRequest;
		const label = kindLabel(request.kind);
		switch (request.state) {
			case 'pending': {
				return `${label} Engagement requested`;
			}
			case 'approved': {
				return `${label} Engagement approved`;
			}
			case 'refused': {
				return `${label} Engagement refused${request.reason ? `: ${request.reason}` : ''}`;
			}
			case 'withdrawn': {
				return `${label} Engagement request withdrawn`;
			}
			default: {
				return `${label} Engagement request`;
			}
		}
	}

	function historyWhen(entry: HistoryEntry): string {
		return formattedDate(entry.at);
	}

	// Best-effort, and deliberately not folded into the try/catch below:
	// a lost read here costs one Staff member her own Withdraw button for
	// one page load, not the Client record she came here to see.
	async function loadStaffId() {
		const response = await apiFetchWithSession('/api/staff/session');
		if (response.ok) {
			const session: { staffId: string } = await response.json();
			staffId = session.staffId;
		}
	}

	// Withdraws request, then flips its own history entry to "withdrawn" in
	// place rather than reloading the whole Client -- pendingRequests()
	// re-derives from that same array, so the block disappears on the next
	// render for free. The disappearance itself is silent to a screen
	// reader, which is why a role=status confirmation follows it (#504 AC:
	// "keyboard-reachable and announced") rather than standing in as the
	// only signal that anything happened.
	async function handleWithdraw(request: EngagementRequestSummary) {
		withdrawError = '';
		withdrawnConfirmation = '';
		withdrawingRequestId = request.requestId;
		try {
			await withdrawRequest(apiFetchWithSession, page.params.practiceId!, request.requestId);
			const entry = detail!.history.find(
				(historyEntry) => historyEntry.type === 'engagement_request' && historyEntry.engagementRequest.requestId === request.requestId
			);
			if (entry?.type === 'engagement_request') {
				entry.engagementRequest.state = 'withdrawn';
			}
			withdrawnConfirmation = `${kindLabel(request.kind)} Engagement request withdrawn`;
		} catch (error_) {
			withdrawError = error_ instanceof Error ? error_.message : 'Failed to withdraw request';
		} finally {
			withdrawingRequestId = '';
		}
	}

	onMount(async () => {
		loadStaffId();
		try {
			detail = await loadClientDetail(apiFetchWithSession, page.params.practiceId!, page.params.clientId!);
		} catch (error_) {
			error = error_ instanceof Error ? error_.message : 'Failed to load Client';
		}
	});
</script>

{#snippet actions()}
	<!--
		"Edit" alone doesn't say whose record it edits, the #513 violation
		CheckAnswers's Change links already solved -- a sibling
		visually-hidden span joined by aria-describedby, so the announced
		name becomes "Edit, Jane Smith" without a second visible word.
	-->
	<Link href={editHref()} label="Edit" describedBy="edit-client-name" />
	<span class="visually-hidden" id="edit-client-name">{detail ? displayName(detail) : ''}</span>
	<!--
		The hub is the only door to an Engagement Request (#496). The label
		names her rather than saying "her", so it reads on its own out of
		context and needs no describedBy sibling. The request screen itself
		then splits the wording by role -- "Ask to start work with" for an
		employee Doula, "Start work with" for an Owner or Admin -- because
		only there is the Credit cost known.
	-->
	{#if detail}
		<Link href={newEngagementRequestHref()} label="Start new work with {displayName(detail)}" />
	{/if}
{/snippet}

{#snippet summary()}
	<stack-l space="var(--space-4)">
		<DescriptionList
			items={[
				{ label: 'Given name', value: detail!.givenName },
				{ label: 'Family name', value: detail!.familyName || '—' },
				{ label: 'Preferred name', value: detail!.preferredName || '—' },
				{ label: 'Email', value: detail!.email || '—' },
				{ label: 'Phone', value: detail!.phone || '—' },
				{
					label: 'Address',
					value:
						[
							detail!.addressLine1,
							detail!.addressLine2,
							detail!.addressLocality,
							detail!.addressRegion,
							detail!.addressPostalCode
						]
							.filter(Boolean)
							.join(', ') || '—'
				},
				{ label: 'Date of birth', value: detail!.dateOfBirth || '—' }
			]}
		/>

		{#each pendingRequests(detail!.history) as request (request.requestId)}
			<!--
				cluster-l composes an existing layout primitive rather than
				writing new layout CSS (the temporary block on new components,
				CLAUDE.md) -- Notice carries no action slot of its own, so
				Withdraw sits beside it rather than inside it. Birth and
				postpartum can both be pending at once (ADR-0017's unique index
				is per kind), so the button names its own kind: two bare
				"Withdraw" controls in one tab order would be indistinguishable
				by ear.
			-->
			<cluster-l space="var(--space-3)" align="center">
				<Notice
					variant="info"
					message="{kindLabel(request.kind)} Engagement requested by {request.requestedByName} on {formattedDate(
						request.requestedAt
					)}"
				/>
				{#if request.requestedBy === staffId}
					<Button
						label="Withdraw {kindLabel(request.kind)} request"
						variant="secondary"
						size="sm"
						loading={withdrawingRequestId === request.requestId}
						onClick={() => handleWithdraw(request)}
					/>
				{/if}
			</cluster-l>
		{/each}

		{#if withdrawnConfirmation}
			<Notice variant="status" message={withdrawnConfirmation} />
		{/if}
		{#if withdrawError}
			<Notice variant="error" message={withdrawError} />
		{/if}
	</stack-l>
{/snippet}

{#snippet practiceDefinedFieldsSection()}
	{#if detail!.resolvedFields.length === 0}
		<p>No Practice-defined fields yet.</p>
	{:else}
		<stack-l space="var(--space-3)">
			{#each detail!.resolvedFields as field (field.fieldId)}
				{#if field.type === 'section_header'}
					<Heading level={3} variant="card" text={field.label} />
				{:else}
					<DescriptionList
						items={[
							{
								label: field.label,
								value: field.note ? `${resolvedFieldValueText(field) || '—'} (${field.note})` : resolvedFieldValueText(field) || '—'
							}
						]}
					/>
				{/if}
			{/each}
		</stack-l>
	{/if}
{/snippet}

{#snippet engagementsSection()}
	<DataTable
		columns={[
			{ label: 'Kind', accessor: (row: EngagementSummary) => (row.kind === 'birth' ? 'Birth' : 'Postpartum') },
			{ label: 'Status', accessor: (row: EngagementSummary) => row.status },
			{ label: 'Started', accessor: (row: EngagementSummary) => formattedDate(row.createdAt) }
		]}
		rows={detail!.engagements}
		hasMore={false}
		emptyMessage="No Engagements yet."
	/>
{/snippet}

{#snippet historySection()}
	<DataTable
		columns={[
			{ label: 'When', accessor: historyWhen },
			{ label: 'Who', accessor: historyWho },
			{ label: 'What', accessor: historyWhat }
		]}
		rows={detail!.history}
		hasMore={false}
		emptyMessage="No history yet."
	/>
{/snippet}

<RecordDetail
	title={detail ? displayName(detail) : ''}
	{summary}
	{actions}
	sections={[
		{ heading: 'Practice-defined fields', content: practiceDefinedFieldsSection },
		{ heading: 'Engagements', content: engagementsSection },
		{ heading: 'History', content: historySection }
	]}
	loading={detail || error ? undefined : 'Loading the Client'}
	loadError={error || undefined}
/>
