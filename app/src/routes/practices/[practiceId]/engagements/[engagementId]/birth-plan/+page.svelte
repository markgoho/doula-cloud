<script lang="ts">
	import { onMount } from 'svelte';
	import { page } from '#lib/appState.svelte.js';
	import { resolve } from '$app/paths';
	import { apiFetchWithSession } from '#lib/api.js';
	import { loadInstance, downloadBirthPlanPdf, type Instance } from '#lib/planInstance.js';
	import { triggerBlobDownload } from '#lib/blobDownload.js';
	import { formatInstant } from '#lib/dates.js';
	import BirthPlanView from '#lib/components/molecules/BirthPlanView.svelte';
	import Heading from '#lib/components/atoms/Heading.svelte';
	import Text from '#lib/components/atoms/Text.svelte';
	import Button from '#lib/components/atoms/Button.svelte';
	import Notice from '#lib/components/atoms/Notice.svelte';
	import Skeleton from '#lib/components/atoms/Skeleton.svelte';
	import BackLink from '#lib/components/molecules/BackLink.svelte';
	import PageTitle from '#lib/components/PageTitle.svelte';
	import type { PageProps as PageProperties } from './$types';

	let { data }: PageProperties = $props();

	// Distinct from `instance === undefined`, which the Plan Instance read
	// also uses for "no Birth Plan created yet" (#planInstance.js's own
	// doc comment) -- without this flag that absence is indistinguishable
	// from "hasn't loaded yet", the same problem the hub's own
	// `planLoaded` solves for its two Plan sections.
	let isLoaded = $state(false);
	let instance = $state<Instance | undefined>();
	let error = $state('');
	let isDownloadingPdf = $state(false);
	let downloadError = $state('');

	const engagementHref = $derived(
		resolve('/practices/[practiceId]/engagements/[engagementId]', {
			practiceId: page.params.practiceId!,
			engagementId: page.params.engagementId!
		})
	);

	// #280: names the Client this address opened cold cannot otherwise
	// identify, in the one heading the page actually has.
	const heading = $derived(`${data.clientName}'s Birth Plan`);

	onMount(async () => {
		try {
			instance = await loadInstance(
				apiFetchWithSession,
				page.params.practiceId!,
				page.params.engagementId!,
				'birth_plan'
			);
		} catch (error_) {
			error = error_ instanceof Error ? error_.message : 'Failed to load Birth Plan';
		} finally {
			isLoaded = true;
		}
	});

	// #306: built fresh from the plan's current answers on every request,
	// never a stored snapshot -- the same PDF the Client's own portal page
	// downloads, from the same rendering.
	async function handleDownloadPdf() {
		downloadError = '';
		isDownloadingPdf = true;
		try {
			const blob = await downloadBirthPlanPdf(
				apiFetchWithSession,
				page.params.practiceId!,
				page.params.engagementId!
			);
			triggerBlobDownload(blob, 'birth-plan.pdf');
		} catch (error_) {
			downloadError = error_ instanceof Error ? error_.message : 'Failed to download Birth Plan';
		} finally {
			isDownloadingPdf = false;
		}
	}
</script>

<div class="no-print">
	<BackLink href={engagementHref} label="Back to {data.clientName}" />
</div>

<PageTitle page={heading} />

<Heading level={1} text={heading} />
<Text text="Engagement created {formatInstant(data.createdAt)}" tone="muted" />

{#if error}
	<div class="no-print"><Notice variant="error" message={error} /></div>
{:else if !isLoaded}
	<div class="no-print"><Skeleton label="Loading Birth Plan" variant="text" lines={4} /></div>
{:else if instance === undefined}
	<Notice variant="info" message="No Birth Plan has been created for this Engagement yet." />
{:else}
	<div class="no-print">
		<Button label="Print" onClick={() => print()} />
		<Button
			label="Download Birth Plan (PDF)"
			icon="file-text"
			variant="secondary"
			onClick={handleDownloadPdf}
			loading={isDownloadingPdf}
		/>
		{#if downloadError}
			<Notice variant="error" message={downloadError} />
		{/if}
	</div>
	<BirthPlanView fields={instance.fields} answers={instance.answers} />
{/if}

<style>
	@media print {
		.no-print {
			display: none;
		}
	}
</style>
