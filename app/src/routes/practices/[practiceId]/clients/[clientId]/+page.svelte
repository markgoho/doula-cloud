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
		type EngagementSummary,
		type HistoryEntry
	} from '#lib/clientDetail.js';
	import RecordDetail from '#lib/components/templates/RecordDetail.svelte';
	import DescriptionList from '#lib/components/molecules/DescriptionList.svelte';
	import DataTable from '#lib/components/organisms/DataTable.svelte';
	import Heading from '#lib/components/atoms/Heading.svelte';
	import Link from '#lib/components/atoms/Link.svelte';
	import Notice from '#lib/components/atoms/Notice.svelte';

	let detail = $state<ClientDetail | undefined>();
	let error = $state('');

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
		const kindLabel = request.kind === 'birth' ? 'Birth' : 'Postpartum';
		switch (request.state) {
			case 'pending': {
				return `${kindLabel} Engagement requested`;
			}
			case 'approved': {
				return `${kindLabel} Engagement approved`;
			}
			case 'refused': {
				return `${kindLabel} Engagement refused${request.reason ? `: ${request.reason}` : ''}`;
			}
			case 'withdrawn': {
				return `${kindLabel} Engagement request withdrawn`;
			}
			default: {
				return `${kindLabel} Engagement request`;
			}
		}
	}

	function historyWhen(entry: HistoryEntry): string {
		return formattedDate(entry.at);
	}

	onMount(async () => {
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
			<Notice
				variant="info"
				message="{request.kind === 'birth'
					? 'Birth'
					: 'Postpartum'} Engagement requested by {request.requestedByName} on {formattedDate(
					request.requestedAt
				)}"
			/>
		{/each}
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
