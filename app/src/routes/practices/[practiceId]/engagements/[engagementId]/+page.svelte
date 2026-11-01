<script lang="ts">
	import { resolve } from '$app/paths';
	import { page } from '#lib/appState.svelte.js';
	import { takeOutcome } from '#lib/outcome.js';
	import { Streamed } from '#lib/streamedRead.svelte.js';
	import RecordDetail from '#lib/components/templates/RecordDetail.svelte';
	import Link from '#lib/components/atoms/Link.svelte';
	import { isOwnerOrAdmin } from '#lib/roles.js';
	import FailedRead from '../../FailedRead.svelte';
	import EngagementScreen from './EngagementScreen.svelte';
	import type { PageProps as PageProperties } from './$types';

	let { data }: PageProperties = $props();
	const engagement = new Streamed(() => data.read);

	// What the save that led here did, if one did (#1710). Taken while the
	// navigation runs, which ends before the read lands.
	const outcome = takeOutcome(page.url);

	// EngagementScreen's sections, in its order. Offers is an Owner's or
	// an Admin's (#2258); Doula fees shows only once its own read lands.
	const sectionHeadings = $derived([
		'On call',
		'Birth outcome',
		'Contract',
		'Visits',
		'Care Plan',
		'Birth Plan',
		'Invoices',
		...(isOwnerOrAdmin(data.session) ? ['Offers'] : []),
		'Messages',
		'Activity'
	]);

	// The summary card's usual rows (#2251): Care, Due and Care phase with
	// a Change, the Doula with one for an Owner or Admin, then Portal
	// invite and Created. An Engagement with no due date has one row fewer.
	const summaryRows = $derived([
		{ label: 'Care', value: 'Birth', action: 'Change' },
		{ label: 'Due', value: 'Nov 20, 2026', action: 'Change' },
		{ label: 'Care phase', value: 'Care in progress', action: 'Change' },
		{ label: 'Doula', value: 'Given Family-name', action: isOwnerOrAdmin(data.session) ? 'Change' : undefined },
		{ label: 'Portal invite', value: 'Accepted' },
		{ label: 'Created', value: 'Oct 27, 2026, 9:00 am' }
	]);
</script>

{#snippet nothing()}{/snippet}

<!-- The header's one action that waits on nothing (#2258). -->
{#snippet messageTheClient()}
	<Link href="#messages" label="Message the Client" variant="secondary" />
{/snippet}

{#if engagement.failure}
	<FailedRead kind={engagement.failure} />
{:else if engagement.value}
	<EngagementScreen data={{ ...data, ...engagement.value }} {outcome} />
{:else}
	<RecordDetail
		title=""
		backHref={resolve('/practices/[practiceId]/clients', { practiceId: page.params.practiceId! })}
		backLabel="Clients"
		isContentsShown
		{summaryRows}
		summaryLinks={['View Client']}
		loadingActions={messageTheClient}
		sections={sectionHeadings.map((heading) => ({ heading, content: nothing }))}
		loading="Loading the Engagement"
	/>
{/if}
