<script lang="ts">
	import { onMount } from 'svelte';
	import { page } from '$app/state';
	import { resolve } from '$app/paths';
	import { apiFetchWithSession } from '#lib/api.js';
	import { loadClientBirthPlan, type Instance } from '#lib/planInstance.js';
	import BirthPlanView from '#lib/BirthPlanView.svelte';
	import Heading from '#lib/components/atoms/Heading.svelte';
	import Text from '#lib/components/atoms/Text.svelte';
	import Button from '#lib/components/atoms/Button.svelte';
	import Notice from '#lib/components/atoms/Notice.svelte';
	import Link from '#lib/components/atoms/Link.svelte';

	let instance = $state<Instance | null | undefined>();
	let error = $state('');

	onMount(async () => {
		try {
			instance = await loadClientBirthPlan(apiFetchWithSession, page.params.engagementId!);
		} catch (error_) {
			error = error_ instanceof Error ? error_.message : 'Failed to load Birth Plan';
		}
	});
</script>

<div class="no-print">
	<Link
		href={resolve('/portal/(authenticated)/engagements/[engagementId]', { engagementId: page.params.engagementId! })}
		label="Back"
	/>
</div>

{#if error}
	<div class="no-print"><Notice variant="error" message={error} /></div>
{:else if instance === undefined}
	<div class="no-print"><Text text="Loading..." /></div>
{:else if instance === null}
	<div class="no-print"><Text text="No Birth Plan has been created for this Engagement yet." /></div>
{:else}
	<Heading level={1} text="Birth Plan" />
	<div class="no-print"><Button label="Print" onClick={() => print()} /></div>
	<BirthPlanView fields={instance.fields} answers={instance.answers} />
{/if}

<style>
	@media print {
		.no-print {
			display: none;
		}
	}
</style>
