<script lang="ts">
	import { resolve } from '$app/paths';
	import { page } from '#lib/appState.svelte.js';
	import { Streamed } from '#lib/streamedRead.svelte.js';
	import RecordDetail from '#lib/components/templates/RecordDetail.svelte';
	import FailedRead from '../../FailedRead.svelte';
	import EngagementScreen from './EngagementScreen.svelte';
	import type { PageProps as PageProperties } from './$types';

	let { data }: PageProperties = $props();
	const engagement = new Streamed(() => data.read);

	// EngagementScreen's sections, in its order. Offers is left out: it
	// shows only once its own read lands, after the Engagement's.
	const sectionHeadings = [
		'On call',
		'Birth outcome',
		'Contract',
		'Visits',
		'Care Plan',
		'Birth Plan',
		'Invoices',
		'Messages',
		'Activity'
	];
</script>

{#snippet nothing()}{/snippet}

{#if engagement.failure}
	<FailedRead kind={engagement.failure} />
{:else if engagement.value}
	<EngagementScreen data={{ ...data, ...engagement.value }} />
{:else}
	<RecordDetail
		title=""
		backHref={resolve('/practices/[practiceId]/clients', { practiceId: page.params.practiceId! })}
		backLabel="Clients"
		isContentsShown
		sections={sectionHeadings.map((heading) => ({ heading, content: nothing }))}
		loading="Loading the Engagement"
	/>
{/if}
