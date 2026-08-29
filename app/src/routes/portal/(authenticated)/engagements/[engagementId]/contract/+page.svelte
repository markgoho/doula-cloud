<script lang="ts">
	import { onMount } from 'svelte';
	import { page } from '$app/state';
	import { resolve } from '$app/paths';
	import { apiFetchWithSession } from '#lib/api.js';
	import { loadClientContract, signContract, type Contract } from '#lib/contract.js';
	import ContractView from '#lib/components/molecules/ContractView.svelte';
	import ContractStatus from '#lib/components/molecules/ContractStatus.svelte';
	import SignContract from '#lib/components/organisms/SignContract.svelte';
	import Heading from '#lib/components/atoms/Heading.svelte';
	import Text from '#lib/components/atoms/Text.svelte';
	import Notice from '#lib/components/atoms/Notice.svelte';
	import Link from '#lib/components/atoms/Link.svelte';

	let contract = $state<Contract | null | undefined>();
	let error = $state('');

	onMount(async () => {
		try {
			contract = await loadClientContract(apiFetchWithSession, page.params.engagementId!);
		} catch (error_) {
			error = error_ instanceof Error ? error_.message : 'Failed to load Contract';
		}
	});

	async function handleSign(fullLegalName: string, isAttestation: boolean) {
		contract = await signContract(apiFetchWithSession, page.params.engagementId!, fullLegalName, isAttestation);
	}
</script>

<Link
	href={resolve('/portal/(authenticated)/engagements/[engagementId]', { engagementId: page.params.engagementId! })}
	label="Back"
/>

{#if error}
	<Notice variant="error" message={error} />
{:else if contract === undefined}
	<Text text="Loading..." />
{:else if contract === null}
	<Text text="No Contract has been sent for this Engagement yet." />
{:else}
	<Heading level={1} text="Contract" />
	<ContractStatus status={contract.status} />
	<ContractView prose={contract.prose} values={contract.values} />
	{#if contract.status === 'sent'}
		<SignContract onSign={handleSign} />
	{/if}
{/if}
