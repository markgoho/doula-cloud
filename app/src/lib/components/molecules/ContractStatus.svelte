<script lang="ts">
	import Button from '#lib/components/atoms/Button.svelte';

	/**
	 * Status display for a Contract on the Staff Engagement view: the raw
	 * status, a clear terminal-state indicator once Voided, the PDF
	 * download (#302) and the Void action -- both offered only on a Signed
	 * Contract, since Void is a one-way transition into the terminal
	 * Voided state and the PDF only exists once signed. The Client-portal
	 * Contract view stopped using this component on #212 (NH-G5): a Client
	 * reads `clientRegister.ts`'s own label and voided notice, never this
	 * component's Staff wording -- its own download control (#302) is
	 * built directly into that route instead.
	 */
	let {
		status,
		onVoid,
		onDownloadPdf
	}: {
		status: string;
		onVoid?: () => Promise<void>;
		onDownloadPdf?: () => Promise<void>;
	} = $props();

	let isVoiding = $state(false);
	let isDownloading = $state(false);
	let error = $state('');

	// Only ever wired to the Void button below, which itself only renders
	// when onVoid is provided -- the non-null assertion reflects that,
	// rather than adding an unreachable defensive branch.
	async function handleVoid() {
		error = '';
		isVoiding = true;
		try {
			await onVoid!();
		} catch (error_) {
			error = error_ instanceof Error ? error_.message : 'Failed to void contract';
		} finally {
			isVoiding = false;
		}
	}

	// Same shape as handleVoid above: the page's onDownloadPdf does the
	// fetch, Blob-to-object-URL conversion and the click that drives the
	// browser's own save, and throws on a failed fetch (#305 is the one
	// still live in local/CI) for this component to catch and show.
	async function handleDownloadPdf() {
		error = '';
		isDownloading = true;
		try {
			await onDownloadPdf!();
		} catch (error_) {
			error = error_ instanceof Error ? error_.message : 'Failed to download signed Contract';
		} finally {
			isDownloading = false;
		}
	}
</script>

<p>Status: {status}</p>

{#if status === 'voided'}
	<p role="status">Voided — this Contract is no longer active.</p>
{/if}

{#if status === 'signed' && onDownloadPdf}
	<Button
		label="Download signed Contract (PDF)"
		icon="file-text"
		variant="secondary"
		onClick={handleDownloadPdf}
		loading={isDownloading}
	/>
{/if}

{#if status === 'signed' && onVoid}
	<Button label="Void Contract" onClick={handleVoid} loading={isVoiding} />
{/if}

{#if error}
	<p role="alert">{error}</p>
{/if}
