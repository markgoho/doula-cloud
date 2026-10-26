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
	import { triggerBlobDownload } from '#lib/blobDownload.js';
	import { contractSignedNotice, contractStatusLabel, contractVoidedNotice } from '#lib/clientRegister.js';
	import ContractView from '#lib/components/molecules/ContractView.svelte';
	import SignContract from '#lib/components/organisms/SignContract.svelte';
	import Badge from '#lib/components/atoms/Badge.svelte';
	import Button from '#lib/components/atoms/Button.svelte';
	import Notice from '#lib/components/atoms/Notice.svelte';
	import DocumentPage from '#lib/components/templates/DocumentPage.svelte';

	/*
	 * The status as a Badge, the Tag row's pattern (#2037): "Ready for your
	 * signature" is the one state that waits on her, so it is info; signed
	 * is done; a voided or draft Contract is neither, so neutral.
	 */
	const STATUS_BADGE_VARIANT: Record<string, 'info' | 'success' | 'neutral'> = {
		sent: 'info',
		signed: 'success'
	};

	let contract = $state<Contract | null | undefined>();
	let error = $state('');
	let isDownloadingPdf = $state(false);
	let downloadError = $state('');
	// Set only by a signature made on this visit, so the Notice is news
	// and takes focus; a signed Contract loaded fresh announces nothing.
	let isSignedHere = $state(false);

	onMount(async () => {
		try {
			contract = await loadClientContract(apiFetchWithSession, page.params.engagementId!);
		} catch (error_) {
			error = error_ instanceof Error ? error_.message : 'Failed to load Contract';
		}
	});

	async function handleSign(fullLegalName: string, isAttestation: boolean) {
		contract = await signContract(apiFetchWithSession, page.params.engagementId!, fullLegalName, isAttestation);
		isSignedHere = true;
	}

	// #302: a fetch that fails here (#305 is the one still live in
	// local/CI) is reported in words rather than swallowed -- the anchor
	// triggerBlobDownload creates never reaches the DOM tree, so there's
	// no href for a screen reader or a failed navigation to fall back on.
	async function handleDownloadSignedContractPdf() {
		downloadError = '';
		isDownloadingPdf = true;
		try {
			const blob = await downloadClientSignedContractPdf(apiFetchWithSession, page.params.engagementId!);
			triggerBlobDownload(blob, 'signed-contract.pdf');
		} catch (error_) {
			downloadError = error_ instanceof Error ? error_.message : 'Failed to download signed Contract';
		} finally {
			isDownloadingPdf = false;
		}
	}
</script>

<DocumentPage
	title="Contract"
	serviceName={page.data.practiceName}
	backHref={resolve('/portal/(authenticated)/engagements/[engagementId]', { engagementId: page.params.engagementId! })}
	loadError={error || undefined}
	loading={contract === undefined ? 'Loading Contract' : undefined}
	empty={contract === null ? 'No Contract has been sent for your care yet.' : undefined}
>
	{#snippet content()}
		<!-- `content` renders only once the Template's own states are past,
		     so `contract` is a loaded Contract here. -->
		{#if contract}
			<!--
				NH-G5 (#212): a Client label, not the Staff `ContractStatus`
				component's bare enum -- `clientRegister.ts` is the one place that
				decides both the status label and the voided notice's wording.
			-->
			<Badge label={contractStatusLabel(contract.status)} variant={STATUS_BADGE_VARIANT[contract.status] ?? 'neutral'} />
			{#if contract.status === 'voided'}
				<Notice variant="info" message={contractVoidedNotice(page.data.practiceName)} />
			{/if}
			<ContractView prose={contract.prose} values={contract.values} />
			{#if contract.status === 'sent'}
				<SignContract onSign={handleSign} />
			{:else if isSignedHere}
				<!-- Where the form stood, and focused: the Sign button that
				     caused it is gone (the Notification banner row, #1598). -->
				<Notice variant="status" message={contractSignedNotice(page.data.practiceName)} isFocusedOnAppear />
			{/if}
			<!--
				#1119: the control is gated on the PDF existing, never on the
				Contract's status. A Contract her Practice has since voided is
				still the one she signed, and the endpoint has always served it
				(#299) -- gating on `status === 'signed'` here was the reason she
				met no way to her own copy the moment it was voided.
			-->
			{#if contract.hasSignedPdf}
				<Button
					label="Download signed Contract (PDF)"
					icon="file-text"
					variant="secondary"
					onClick={handleDownloadSignedContractPdf}
					loading={isDownloadingPdf}
				/>
				{#if downloadError}
					<Notice variant="error" message={downloadError} />
				{/if}
			{/if}
		{/if}
	{/snippet}
</DocumentPage>
