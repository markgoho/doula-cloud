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

	let instance = $state<Instance | null | undefined>();
	let error = $state('');
	let isAcknowledging = $state(false);
	let acknowledgeError = $state('');

	onMount(async () => {
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
			<p role="status">You confirmed you've read this on {formatInstant(instance.clientAcknowledgedAt)}.</p>
		{:else}
			<Button label="I've read this" onClick={handleAcknowledge} loading={isAcknowledging} />
		{/if}
		{#if acknowledgeError}
			<p role="alert">{acknowledgeError}</p>
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
