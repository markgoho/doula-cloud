<script lang="ts">
	import { resolve } from '$app/paths';
	import { page } from '#lib/appState.svelte.js';
	import { takeOutcome } from '#lib/outcome.js';
	import { ListBack } from '#lib/listOrigin.svelte.js';
	import { Streamed } from '#lib/streamedRead.svelte.js';
	import RecordDetail from '#lib/components/templates/RecordDetail.svelte';
	import Link from '#lib/components/atoms/Link.svelte';
	import { isAmbientContractor, isOwnerOrAdmin } from '#lib/roles.js';
	import FailedRead from '../../FailedRead.svelte';
	import EngagementScreen from './EngagementScreen.svelte';
	import WorkEnded from './WorkEnded.svelte';
	import type { PageProps as PageProperties } from './$types';

	let { data }: PageProperties = $props();
	const engagement = new Streamed(() => data.read);

	// What the save that led here did, if one did (#1710). Taken while the
	// navigation runs, which ends before the read lands.
	const outcome = takeOutcome(page.url);

	// #2864: the list the page was opened from, read while the navigation
	// runs, so the loading frame and the page name the same one.
	const back = new ListBack(
		() => ({
			schedule: resolve('/practices/[practiceId]/schedule', { practiceId: page.params.practiceId! }),
			invoices: resolve('/practices/[practiceId]/invoices', { practiceId: page.params.practiceId! }),
			'on-call': resolve('/practices/[practiceId]/on-call', { practiceId: page.params.practiceId! }),
			contracts: resolve('/practices/[practiceId]/contracts', { practiceId: page.params.practiceId! })
		}),
		() => page.url.pathname
	);

	// EngagementScreen's sections, while the Engagement loads. Its real order
	// follows the care phase and waits on that read, so the frame holds the
	// usual run of headings: the first section's top is where it will be, and
	// Activity closes it. Offers is an Owner's or an Admin's
	// (#2258); Doula fees stands once the Engagement read says there is a fee
	// (#2492), which this frame drawn before it cannot know.
	const sectionHeadings = $derived([
		'On call',
		'Birth Plan',
		'Visits',
		'Messages',
		'Care Plan',
		'Contract',
		'Invoices',
		...(isOwnerOrAdmin(data.session) ? ['Offers'] : []),
		'Birth outcome',
		'Activity'
	]);

	// The facts' shape before their data (#2755): six rows, as a reader who
	// reads money has them.
	const factsRows = $derived([
		{ label: 'On call tonight', value: 'Priya Raman' },
		{ label: 'Next Visit', value: 'Thu Nov 19, 10:00am, Priya Raman' },
		{ label: 'Contract', value: 'Sent, waiting for the Client to sign' },
		...(isAmbientContractor(data.session) ? [] : [{ label: 'Owed', value: '$1,200.00, due Dec 1' }]),
		{ label: 'Messages', value: 'Unanswered since 9:22am' },
		{ label: 'Phone', value: '(585) 555-0142' }
	]);
</script>

{#snippet nothing()}{/snippet}

<!-- The header's one move while the record loads (#2258, #2755): a link that
     waits on nothing. -->
{#snippet moves()}
	<ul class="moves">
		<li><Link href="#messages" label="Message the Client" variant="outline" /></li>
	</ul>
{/snippet}

{#if engagement.refusal?.code === 'ATTACHMENT_ENDED' && engagement.refusal.message}
	<WorkEnded message={engagement.refusal.message} />
{:else if engagement.failure}
	<FailedRead kind={engagement.failure} />
{:else if engagement.value}
	<EngagementScreen data={{ ...data, ...engagement.value }} {outcome} {back} />
{:else}
	<RecordDetail
		title=""
		backHref={back.href(resolve('/practices/[practiceId]/clients', { practiceId: page.params.practiceId! }))}
		backLabel={back.label('Clients')}
		onBack={back.go}
		isContentsShown
		isStatusLinePending
		hasDetails
		{factsRows}
		loadingActions={moves}
		sections={sectionHeadings.map((heading) => ({ heading, content: nothing }))}
		read={{ status: 'loading', label: 'Loading the work' }}
	/>
{/if}

<style>
	.moves {
		display: flex;
		flex-wrap: wrap;
		gap: var(--space-2);
		margin: 0;
		padding: 0;
		list-style: none;
	}
</style>
