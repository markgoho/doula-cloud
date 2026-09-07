<script lang="ts">
	import { onMount } from 'svelte';
	import { page } from '#lib/appState.svelte.js';
	import { resolve } from '$app/paths';
	import { apiFetchWithSession } from '#lib/api.js';
	import {
		loadClientContract,
		signContract,
		downloadClientSignedContractPdf,
		type Contract
	} from '#lib/contract.js';
	import { contractStatusLabel, contractVoidedNotice } from '#lib/clientRegister.js';
	import ContractView from '#lib/components/molecules/ContractView.svelte';
	import SignContract from '#lib/components/organisms/SignContract.svelte';
	import Heading from '#lib/components/atoms/Heading.svelte';
	import Text from '#lib/components/atoms/Text.svelte';
	import Notice from '#lib/components/atoms/Notice.svelte';
	import Button from '#lib/components/atoms/Button.svelte';
	import BackLink from '#lib/components/molecules/BackLink.svelte';
	import PageTitle from '#lib/components/PageTitle.svelte';

	let contract = $state<Contract | null | undefined>();
	let error = $state('');
	let isDownloadingPdf = $state(false);
	let downloadError = $state('');

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

	// #302: a fetch that fails here (#305 is the one still live in
	// local/CI) is reported in words rather than swallowed -- the anchor
	// created below never reaches the DOM tree, so there's no href for a
	// screen reader or a failed navigation to fall back on.
	async function handleDownloadSignedContract() {
		downloadError = '';
		isDownloadingPdf = true;
		try {
			const blob = await downloadClientSignedContractPdf(apiFetchWithSession, page.params.engagementId!);
			const url = URL.createObjectURL(blob);
			const link = document.createElement('a');
			link.href = url;
			link.download = 'signed-contract.pdf';
			link.click();
			URL.revokeObjectURL(url);
		} catch (error_) {
			downloadError = error_ instanceof Error ? error_.message : 'Failed to download signed Contract';
		} finally {
			isDownloadingPdf = false;
		}
	}
</script>

<BackLink
	href={resolve('/portal/(authenticated)/engagements/[engagementId]', { engagementId: page.params.engagementId! })}
/>

<PageTitle page="Contract" serviceName={page.data.practiceName} />

{#if error}
	<Notice variant="error" message={error} />
{:else if contract === undefined}
	<Text text="Loading..." />
{:else if contract === null}
	<Text text="No Contract has been sent for your care yet." />
{:else}
	<Heading level={1} text="Contract" />
	<!--
		NH-G5 (#212): a Client label, not the Staff `ContractStatus`
		component's bare enum -- `clientRegister.ts` is the one place that
		decides both the status label and the voided notice's wording.
	-->
	<Text text={contractStatusLabel(contract.status)} />
	{#if contract.status === 'voided'}
		<p role="status">{contractVoidedNotice(page.data.practiceName)}</p>
	{/if}
	<ContractView prose={contract.prose} values={contract.values} />
	{#if contract.status === 'sent'}
		<SignContract onSign={handleSign} />
	{/if}
	{#if contract.status === 'signed'}
		<Button
			label="Download signed Contract (PDF)"
			icon="file-text"
			variant="secondary"
			onClick={handleDownloadSignedContract}
			loading={isDownloadingPdf}
		/>
		{#if downloadError}
			<p role="alert">{downloadError}</p>
		{/if}
	{/if}
{/if}
