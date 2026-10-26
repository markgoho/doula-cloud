<script lang="ts">
	import { onMount } from 'svelte';
	import { page } from '#lib/appState.svelte.js';
	import { resolve } from '$app/paths';
	import { apiFetchWithSession } from '#lib/api.js';
	import { loadClientBirthPlan, acknowledgeClientBirthPlan, downloadClientBirthPlanPdf, type Instance } from '#lib/planInstance.js';
	import { triggerBlobDownload } from '#lib/blobDownload.js';
	import { formatInstant } from '#lib/dates.js';
	import BirthPlanView from '#lib/components/molecules/BirthPlanView.svelte';
	import Button from '#lib/components/atoms/Button.svelte';
	import Notice from '#lib/components/atoms/Notice.svelte';
	import DocumentPage from '#lib/components/templates/DocumentPage.svelte';
	import ErrorPage from '#lib/components/templates/ErrorPage.svelte';

	let instance = $state<Instance | null | undefined>();
	let error = $state('');
	let isAcknowledging = $state(false);
	let acknowledgeError = $state('');
	// Set only by a confirmation made on this visit, so its Notice takes
	// focus where the pressed button was (the Notification banner row).
	let isAcknowledgedHere = $state(false);
	let isDownloadingPdf = $state(false);
	let downloadError = $state('');

	onMount(async () => {
		// Not applicable, per ADR-0015's suppression rule (#311's kind
		// half, #294's outcome half) -- skip the fetch entirely rather
		// than let a real Birth Plan endpoint 404 read as "not yet". The
		// endpoint refuses this Engagement on its own either way; this
		// only spares her the request.
		if (!page.data.offersBirthPlan) return;
		try {
			instance = await loadClientBirthPlan(apiFetchWithSession, page.params.engagementId!);
		} catch (error_) {
			error = error_ instanceof Error ? error_.message : 'Failed to load Birth Plan';
		}
	});

	// Not gated to Print (#301): a Client can confirm she's read this
	// whether or not she ever prints it. Repeatable and one-way -- there
	// is no Client-facing way to undo it, only a Staff edit that changes
	// the answers (which clears it server-side and would show up here on
	// the next load).
	async function handleAcknowledge() {
		acknowledgeError = '';
		isAcknowledging = true;
		try {
			instance = await acknowledgeClientBirthPlan(apiFetchWithSession, page.params.engagementId!);
			isAcknowledgedHere = true;
		} catch (error_) {
			acknowledgeError = error_ instanceof Error ? error_.message : 'Failed to confirm you read this Birth Plan';
		} finally {
			isAcknowledging = false;
		}
	}

	// #306: a file she can keep, built fresh from the plan's current
	// answers rather than a stored snapshot -- there is no "final" Birth
	// Plan the way a signed Contract has one. A fetch that fails is
	// reported in words (mirrors contract's own handleDownloadSignedContractPdf),
	// since triggerBlobDownload's anchor never reaches the DOM tree.
	async function handleDownloadPdf() {
		downloadError = '';
		isDownloadingPdf = true;
		try {
			const blob = await downloadClientBirthPlanPdf(apiFetchWithSession, page.params.engagementId!);
			triggerBlobDownload(blob, 'birth-plan.pdf');
		} catch (error_) {
			downloadError = error_ instanceof Error ? error_.message : 'Failed to download Birth Plan';
		} finally {
			isDownloadingPdf = false;
		}
	}
</script>

{#if !page.data.offersBirthPlan}
	<!--
		#311: an Engagement this does not apply to gets the portal's
		ordinary not-found state -- the same one `+error.svelte` renders --
		rather than a Birth-Plan-specific message. GLOSSARY.md's Birth Plan
		entry: where it does not apply, she meets no mention of it at all.
	-->
	<ErrorPage
		kind="notFound"
		wayOutHref={resolve('/portal/(authenticated)/engagements/[engagementId]', { engagementId: page.params.engagementId! })}
		wayOutLabel="Go to your care"
	/>
{:else}
	<DocumentPage
		title="Birth Plan"
		serviceName={page.data.practiceName}
		backHref={resolve('/portal/(authenticated)/engagements/[engagementId]', { engagementId: page.params.engagementId! })}
		loadError={error || undefined}
		loading={instance === undefined ? 'Loading Birth Plan' : undefined}
		empty={instance === null ? 'No Birth Plan has been created for your care yet.' : undefined}
	>
		{#snippet content()}
			<!-- `content` renders only once the Template's own states are past,
			     so `instance` is a loaded Birth Plan here. -->
			{#if instance}
				<!-- A cluster, so the two buttons keep a gap between them when
				     they wrap onto two rows at a narrow width (#1576). Both
				     secondary: what she does with the plan, not the task. -->
				<cluster-l class="no-print" space="var(--space-3)">
					<Button label="Print" variant="secondary" onClick={() => print()} />
					<Button
						label="Download Birth Plan (PDF)"
						icon="file-text"
						variant="secondary"
						onClick={handleDownloadPdf}
						loading={isDownloadingPdf}
					/>
				</cluster-l>
				{#if downloadError}
					<div class="no-print"><Notice variant="error" message={downloadError} /></div>
				{/if}
				<BirthPlanView fields={instance.fields} answers={instance.answers} />
				<!--
					#2036: the act that ends the task comes after the thing she
					reads (Fitts's Law: a form's submit sits at the end), and it
					is the screen's one primary. Its confirmation replaces it in
					place, focused only when she has just pressed it -- a plan she
					confirmed on an earlier visit announces nothing on arrival.
				-->
				<div class="no-print">
					{#if instance.clientAcknowledgedAt}
						<Notice
							variant="status"
							message="You confirmed you've read this on {formatInstant(instance.clientAcknowledgedAt)}."
							isFocusedOnAppear={isAcknowledgedHere}
						/>
					{:else}
						<stack-l space="var(--space-related)">
							{#if acknowledgeError}
								<Notice variant="error" message={acknowledgeError} />
							{/if}
							<cluster-l space="var(--space-3)">
								<Button label="I've read this" onClick={handleAcknowledge} loading={isAcknowledging} />
							</cluster-l>
						</stack-l>
					{/if}
				</div>
			{/if}
		{/snippet}
	</DocumentPage>
{/if}

<style>
	@media print {
		.no-print {
			display: none;
		}
	}
</style>
