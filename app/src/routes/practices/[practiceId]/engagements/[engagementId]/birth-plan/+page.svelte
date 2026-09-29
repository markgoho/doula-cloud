<script lang="ts">
	import { onMount } from 'svelte';
	import { page } from '#lib/appState.svelte.js';
	import { resolve } from '$app/paths';
	import { apiFetchWithSession } from '#lib/api.js';
	import { loadInstance, downloadBirthPlanPdf, type Instance } from '#lib/planInstance.js';
	import { triggerBlobDownload } from '#lib/blobDownload.js';
	import { formatInstant } from '#lib/dates.js';
	import BirthPlanView from '#lib/components/molecules/BirthPlanView.svelte';
	import Text from '#lib/components/atoms/Text.svelte';
	import Button from '#lib/components/atoms/Button.svelte';
	import Notice from '#lib/components/atoms/Notice.svelte';
	import DocumentPage from '#lib/components/templates/DocumentPage.svelte';
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

<DocumentPage
	title={heading}
	backHref={engagementHref}
	backLabel="Back to {data.clientName}"
	loadError={error || undefined}
	loading={isLoaded ? undefined : 'Loading Birth Plan'}
	empty={instance === undefined ? 'No Birth Plan has been created for this Engagement yet.' : undefined}
>
	{#snippet content()}
		<!-- `content` renders only once the Template's own states are past,
		     so `instance` is a loaded Birth Plan here. -->
		{#if instance}
			<Text text="Engagement created {formatInstant(data.createdAt)}" tone="muted" />
			<!-- DocumentPage's own `.no-print` is scoped to that component and
			     never reaches this snippet, so the route keeps its own (#1576),
			     the same as the portal Birth Plan (#1574). -->
			<div class="no-print">
				<!-- A cluster, so the two buttons keep a gap between them when
				     they wrap onto two rows at a narrow width. -->
				<cluster-l space="var(--space-3)">
					<Button label="Print" onClick={() => print()} />
					<Button
						label="Download Birth Plan (PDF)"
						icon="file-text"
						variant="secondary"
						onClick={handleDownloadPdf}
						loading={isDownloadingPdf}
					/>
				</cluster-l>
				{#if downloadError}
					<Notice variant="error" message={downloadError} />
				{/if}
			</div>
			<BirthPlanView fields={instance.fields} answers={instance.answers} />
		{/if}
	{/snippet}
</DocumentPage>

<style>
	@media print {
		.no-print {
			display: none;
		}
	}
</style>
