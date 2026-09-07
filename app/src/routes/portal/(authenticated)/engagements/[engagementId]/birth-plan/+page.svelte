<script lang="ts">
	import { onMount } from 'svelte';
	import { page } from '#lib/appState.svelte.js';
	import { resolve } from '$app/paths';
	import { apiFetchWithSession } from '#lib/api.js';
	import { loadClientBirthPlan, acknowledgeClientBirthPlan, type Instance } from '#lib/planInstance.js';
	import { formatInstant } from '#lib/dates.js';
	import BirthPlanView from '#lib/components/molecules/BirthPlanView.svelte';
	import Heading from '#lib/components/atoms/Heading.svelte';
	import Text from '#lib/components/atoms/Text.svelte';
	import Button from '#lib/components/atoms/Button.svelte';
	import Notice from '#lib/components/atoms/Notice.svelte';
	import BackLink from '#lib/components/molecules/BackLink.svelte';
	import PageTitle from '#lib/components/PageTitle.svelte';
	import ErrorPage from '#lib/components/templates/ErrorPage.svelte';

	let instance = $state<Instance | null | undefined>();
	let error = $state('');
	let isAcknowledging = $state(false);
	let acknowledgeError = $state('');

	onMount(async () => {
		// #311: not applicable, per ADR-0015's suppression rule -- skip the
		// fetch entirely rather than let a real Birth Plan endpoint 404
		// read as "not yet".
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
		} catch (error_) {
			acknowledgeError = error_ instanceof Error ? error_.message : 'Failed to confirm you read this Birth Plan';
		} finally {
			isAcknowledging = false;
		}
	}
</script>

{#if !page.data.offersBirthPlan}
	<!--
		#311: an Engagement this does not apply to gets the portal's
		ordinary not-found state -- the same one `+error.svelte` renders --
		rather than a Birth-Plan-specific message. CONTEXT.md's Birth Plan
		entry: where it does not apply, she meets no mention of it at all.
	-->
	<ErrorPage
		kind="notFound"
		wayOutHref={resolve('/portal/(authenticated)/engagements/[engagementId]', { engagementId: page.params.engagementId! })}
		wayOutLabel="Go to your care"
	/>
{:else}
	<div class="no-print">
		<BackLink
			href={resolve('/portal/(authenticated)/engagements/[engagementId]', { engagementId: page.params.engagementId! })}
		/>
	</div>

	<PageTitle page="Birth Plan" serviceName={page.data.practiceName} />

	{#if error}
		<div class="no-print"><Notice variant="error" message={error} /></div>
	{:else if instance === undefined}
		<div class="no-print"><Text text="Loading..." /></div>
	{:else if instance === null}
		<div class="no-print"><Text text="No Birth Plan has been created for your care yet." /></div>
	{:else}
		<Heading level={1} text="Birth Plan" />
		<div class="no-print">
			<Button label="Print" onClick={() => print()} />
			{#if instance.clientAcknowledgedAt}
				<Notice variant="status" message="You confirmed you've read this on {formatInstant(instance.clientAcknowledgedAt)}." />
			{:else}
				<Button label="I've read this" onClick={handleAcknowledge} loading={isAcknowledging} />
			{/if}
			{#if acknowledgeError}
				<Notice variant="error" message={acknowledgeError} />
			{/if}
		</div>
		<BirthPlanView fields={instance.fields} answers={instance.answers} />
	{/if}
{/if}

<style>
	@media print {
		.no-print {
			display: none;
		}
	}
</style>
